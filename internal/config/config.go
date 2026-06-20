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

// Package config loads and validates runtime configuration from environment variables.
package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// BackendOllama is the backend identifier for Ollama.
	BackendOllama = "ollama"
	// BackendLMStudio is the backend identifier for LM Studio.
	BackendLMStudio = "lmstudio"
)

// DBPathForProject returns the SQLite database path for a given project,
// derived from a SHA-256 hash of the project path, embedding model name, and
// IndexVersion. Including the model ensures that switching models creates a
// fresh index automatically. Including IndexVersion ensures that incompatible
// chunker/index format changes never share an index with older data.
func DBPathForProject(projectPath, model string) string {
	return DBPathForProjectBase(XDGDataDir(), projectPath, model)
}

// DBPathForProjectBase returns the SQLite database path for a given project
// using an explicit data directory instead of reading XDG_DATA_HOME from the
// environment. Safe to call from parallel goroutines.
func DBPathForProjectBase(dataDir, projectPath, model string) string {
	key := canonicalProjectKey(projectPath)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(key+"\x00"+model+"\x00"+IndexVersion)))
	return filepath.Join(dataDir, "lumen", hash[:16], "index.db")
}

// canonicalProjectKey normalizes a project path into the stable form used to
// derive the per-project DB hash, so the same directory always maps to the same
// index regardless of how the path was spelled. On Windows — a case-insensitive
// filesystem that also accepts forward slashes — C:\Repo, c:\repo, and C:/Repo
// all denote one directory, so the key folds both case and separators;
// otherwise paths are case-sensitive and only the separator is cleaned. For the
// already-absolute, cleaned paths callers pass on Unix this is the identity
// transform, so existing Unix indexes keep their current hash and are not
// rebuilt. On Windows it canonicalizes case so two differently-cased spellings
// of one repo no longer create duplicate, mutually-invisible indexes.
func canonicalProjectKey(projectPath string) string {
	p := filepath.Clean(projectPath)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(filepath.ToSlash(p))
	}
	return p
}

// XDGDataDir returns the XDG data home directory, defaulting to
// ~/.local/share if XDG_DATA_HOME is not set.
func XDGDataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

// XDGConfigDir returns XDG_CONFIG_HOME or ~/.config.
func XDGConfigDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	return filepath.Join(XDGConfigDir(), "lumen", "config.yaml")
}
