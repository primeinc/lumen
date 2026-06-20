// Copyright 2026 Aeneas Rekkas
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package embedder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ory/lumen/internal/config"
)

const healthCheckTimeout = 5 * time.Second
const reprobeInterval = 30 * time.Second

// FailoverEmbedder wraps multiple backend embedders with ordered health-check
// failover. It probes servers on first use and fails over on transient errors.
// When the ConfigService's server list changes (e.g. via hot reload), the
// embedder re-initializes on the next Embed call.
type FailoverEmbedder struct {
	cfg           *config.ConfigService
	logger        *slog.Logger
	mu            sync.Mutex
	probeMu       sync.Mutex // serializes health probing so it runs outside mu
	servers       []serverEntry
	cachedConfigs []config.ServerConfig // snapshot at last init
	active        int
	checked       bool
	lastProbeTime time.Time
}

type serverEntry struct {
	emb     Embedder
	healthy bool
}

// NewFailoverEmbedder creates a FailoverEmbedder. Servers are probed lazily on
// first Embed call.
func NewFailoverEmbedder(cfg *config.ConfigService) *FailoverEmbedder {
	return &FailoverEmbedder{cfg: cfg, active: -1}
}

// SetLogger attaches a logger to the embedder for structured diagnostic output.
func (f *FailoverEmbedder) SetLogger(l *slog.Logger) {
	f.logger = l
}

// ActiveServerIndex returns the index of the currently active server, or -1
// if no healthy server has been found yet.
func (f *FailoverEmbedder) ActiveServerIndex() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active
}

// Healthy probes the configured embedding servers (lazily, the same way Embed
// does) and reports whether at least one is currently reachable. Callers use it
// to fail fast before starting indexing when the backend is down, instead of
// churning through embed batches that all fail. Lumen must remain usable when
// the backend (e.g. Ollama) is not running; this is the gate that makes that so.
func (f *FailoverEmbedder) Healthy() bool {
	return f.ensureReady(true) >= 0
}

// ensureReady (re-)initializes and health-probes the server list when needed and
// returns the active server index (-1 if none is reachable). It must be called
// WITHOUT f.mu held: the probes perform network I/O (up to healthCheckTimeout per
// server), and holding f.mu across them would block every other embedder
// operation (Embed/Dimensions/ModelName) for the duration. probeMu serializes
// concurrent probers so the work happens once; results are committed under f.mu.
func (f *FailoverEmbedder) ensureReady(log bool) int {
	f.mu.Lock()
	if !f.needsProbeLocked() {
		active := f.active
		f.mu.Unlock()
		return active
	}
	f.mu.Unlock()

	f.probeMu.Lock()
	defer f.probeMu.Unlock()

	// Another goroutine may have probed while we waited for probeMu.
	f.mu.Lock()
	if !f.needsProbeLocked() {
		active := f.active
		f.mu.Unlock()
		return active
	}
	reprobe := f.checked && f.active < 0
	f.mu.Unlock()
	if reprobe && log && f.logger != nil {
		f.logger.Info("re-probing embedding servers after cooldown")
	}

	servers, cached, active := f.probeAndSelect()

	f.mu.Lock()
	f.servers, f.cachedConfigs, f.active = servers, cached, active
	f.checked = true
	f.lastProbeTime = time.Now()
	f.mu.Unlock()
	return active
}

// needsProbeLocked reports whether the server list needs (re-)initialization or
// a cooldown re-probe. Must be called with f.mu held.
func (f *FailoverEmbedder) needsProbeLocked() bool {
	needsInit := !f.checked || f.serversChanged()
	needsReprobe := f.active < 0 && time.Since(f.lastProbeTime) >= reprobeInterval
	return needsInit || needsReprobe
}

// Dimensions returns dims for the active server. On first call it probes
// servers for health to ensure the returned dimensions match the server
// that will actually handle embeddings.
func (f *FailoverEmbedder) Dimensions() int {
	idx := f.ensureReady(false)
	if idx < 0 {
		idx = 0
	}
	return f.cfg.ServerDims(idx)
}

// ModelName returns the model name for the active server. On first call it
// probes servers for health to ensure the returned name matches the server
// that will actually handle embeddings.
func (f *FailoverEmbedder) ModelName() string {
	idx := f.ensureReady(false)
	if idx < 0 {
		idx = 0
	}
	servers := f.cfg.Servers()
	if idx < len(servers) {
		return servers[idx].Model
	}
	return ""
}

// Embed generates embeddings for the given texts. On first call it probes
// all servers for health and selects the first healthy one. On transient
// errors (5xx, network) it fails over to the next healthy server.
func (f *FailoverEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if f.ensureReady(true) < 0 {
		return nil, fmt.Errorf("all embedding servers are unhealthy")
	}

	for {
		f.mu.Lock()
		active := f.active
		if active < 0 || active >= len(f.servers) {
			f.mu.Unlock()
			return nil, fmt.Errorf("all embedding servers exhausted")
		}
		emb := f.servers[active].emb
		f.mu.Unlock()

		result, err := emb.Embed(ctx, texts)
		if err == nil {
			return result, nil
		}

		if !isTransientError(err) {
			return nil, err // 4xx = config error, don't failover
		}

		// Mark current as unhealthy, find next
		f.mu.Lock()
		f.servers[active].healthy = false
		next := f.findNextHealthy(active)
		if next < 0 {
			f.active = -1
			f.mu.Unlock()
			return nil, fmt.Errorf("all embedding servers exhausted after failover: last error: %w", err)
		}
		if f.logger != nil {
			f.logger.Warn("embedding server failed, trying next", "failed", active, "next", next, "error", err)
		}
		f.active = next
		if err := f.ensureEmbedder(next); err != nil {
			f.mu.Unlock()
			return nil, fmt.Errorf("failed to initialize fallback server %d: %w", next, err)
		}
		f.mu.Unlock()
		// Loop continues with new active server
	}
}

// serversChanged returns true if the ConfigService's server list differs from
// the cached server list (e.g. after a hot reload). Must be called with f.mu held.
func (f *FailoverEmbedder) serversChanged() bool {
	current := f.cfg.Servers()
	if len(current) != len(f.servers) {
		return true
	}
	for i, srv := range current {
		// Compare key fields — if any differ, servers have changed.
		if f.servers[i].emb == nil {
			continue // not initialized yet, can't compare
		}
		cached := f.cachedConfigs[i]
		if srv.Backend != cached.Backend || srv.Host != cached.Host || srv.Model != cached.Model {
			return true
		}
	}
	return false
}

// probeAndSelect probes each configured server (network I/O; must run WITHOUT
// f.mu held) and returns the new server entries, a snapshot of their configs,
// and the index of the first healthy, constructible server (-1 if none). It
// reads only f.cfg and f.logger and does not touch f's shared mutable state; the
// caller commits the result under f.mu.
func (f *FailoverEmbedder) probeAndSelect() ([]serverEntry, []config.ServerConfig, int) {
	cfgServers := f.cfg.Servers()
	servers := make([]serverEntry, len(cfgServers))
	cached := make([]config.ServerConfig, len(cfgServers))
	copy(cached, cfgServers)
	if f.logger != nil {
		f.logger.Info("probing embedding servers", "count", len(cfgServers))
	}
	for i := range servers {
		if !f.probeHealth(i) {
			continue
		}
		servers[i].healthy = true
		emb, err := f.newEmbedderFor(i)
		if err != nil {
			continue // e.g. unknown backend; try next server
		}
		servers[i].emb = emb
		if f.logger != nil {
			srv := cfgServers[i]
			f.logger.Info("selected embedding server", "server", i, "backend", srv.Backend, "host", srv.Host, "model", srv.Model)
		}
		return servers, cached, i
	}
	if f.logger != nil {
		f.logger.Warn("no healthy embedding server found")
	}
	return servers, cached, -1
}

// findNextHealthy probes servers after index `after` and returns the first
// healthy one, or -1 if none found. Must be called with f.mu held.
func (f *FailoverEmbedder) findNextHealthy(after int) int {
	for i := after + 1; i < len(f.servers); i++ {
		if f.probeHealth(i) {
			f.servers[i].healthy = true
			return i
		}
	}
	return -1
}

// probeHealth checks if server i is healthy by sending a GET request.
// For Ollama: GET / (returns "Ollama is running").
// For LM Studio: GET /v1/models.
// Returns true only if StatusCode == 200.
func (f *FailoverEmbedder) probeHealth(i int) bool {
	servers := f.cfg.Servers()
	if i >= len(servers) {
		return false
	}
	srv := servers[i]
	endpoint := srv.Host + "/"
	if srv.Backend == "lmstudio" {
		endpoint = srv.Host + "/v1/models"
	}
	client := &http.Client{Timeout: healthCheckTimeout}
	resp, err := client.Get(endpoint)
	if err != nil {
		if f.logger != nil {
			f.logger.Warn("health probe failed", "server", i, "backend", srv.Backend, "host", srv.Host, "error", err)
		}
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		if f.logger != nil {
			f.logger.Warn("health probe non-200", "server", i, "backend", srv.Backend, "host", srv.Host, "status", resp.StatusCode)
		}
		return false
	}
	return true
}

// newEmbedderFor constructs the backend embedder for server i without touching
// f's shared state, so it is safe to call without f.mu held.
func (f *FailoverEmbedder) newEmbedderFor(i int) (Embedder, error) {
	servers := f.cfg.Servers()
	srv := servers[i]
	dims := f.cfg.ServerDims(i)
	ctxLen := f.cfg.ServerCtxLength(i)
	switch srv.Backend {
	case "ollama":
		return NewOllama(srv.Model, dims, ctxLen, srv.Host)
	case "lmstudio":
		return NewLMStudio(srv.Model, dims, srv.Host)
	default:
		return nil, fmt.Errorf("unknown backend %q", srv.Backend)
	}
}

// ensureEmbedder lazily initializes the backend embedder for server i.
// Must be called with f.mu held.
func (f *FailoverEmbedder) ensureEmbedder(i int) error {
	if f.servers[i].emb != nil {
		return nil
	}
	emb, err := f.newEmbedderFor(i)
	if err != nil {
		return err
	}
	f.servers[i].emb = emb
	return nil
}

// isTransientError returns true if the error represents a transient failure
// that warrants failover (5xx HTTP errors or network errors). Returns false
// for 4xx errors (configuration errors, no failover).
func isTransientError(err error) bool {
	var ee *EmbedError
	if errors.As(err, &ee) {
		return ee.StatusCode >= 500
	}
	return true // network errors are transient
}
