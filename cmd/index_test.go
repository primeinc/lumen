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

package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ory/lumen/internal/config"
	"github.com/ory/lumen/internal/index"
	"github.com/spf13/cobra"
)

func TestRunIndex_RefusesUnindexableRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".lumenignore"), []byte("**\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runIndex(indexCmd, []string{dir})
	if err == nil {
		t.Fatal("expected runIndex to refuse an un-indexable root, got nil error")
	}
	if !strings.Contains(err.Error(), ".lumenignore catch-all") {
		t.Fatalf("expected error to mention the .lumenignore catch-all reason, got %q", err.Error())
	}
}

func TestRunIndex_RefusesOversizedNestedRoot(t *testing.T) {
	// A non-git directory holding more nested git repos than the limit is a
	// workspace/home/temp dir, not a single project. runIndex must refuse it up
	// front — before the embedding-backend check and before indexing any nested
	// repo — so the 2026-06-19 runaway-indexing incident cannot recur from the
	// CLI. A .git dir satisfies IsGitRoot, so fake repos keep this fast and
	// backend-free; the low cap keeps the fixture small.
	//
	// Isolate XDG_CONFIG_HOME so the test never reads a developer's real
	// ~/.config/lumen/config.yaml: loadConfigWithFlags runs before the
	// nested-repo refusal, so without this the test is non-hermetic.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LUMEN_MAX_NESTED_REPOS", "2")
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		if err := os.MkdirAll(filepath.Join(dir, name, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	err := runIndex(indexCmd, []string{dir})
	if err == nil {
		t.Fatal("expected runIndex to refuse an oversized nested root, got nil error")
	}
	if !errors.Is(err, index.ErrTooManyNestedRepos) {
		t.Fatalf("expected err to wrap index.ErrTooManyNestedRepos, got %q", err.Error())
	}
}

func TestIndexingWorkPending(t *testing.T) {
	// runIndex must not require a live embedding backend for an already-fresh
	// index (a no-op that embeds nothing); it must still require one when there is
	// real work. These cover the three signals indexingWorkPending reports on.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg, err := config.NewConfigService("")
	if err != nil {
		t.Fatalf("NewConfigService: %v", err)
	}
	emb := newEmbedder(cfg)
	model := emb.ModelName()
	dims := emb.Dimensions()

	noForce := func() *cobra.Command {
		c := &cobra.Command{}
		c.Flags().Bool("force", false, "")
		return c
	}

	t.Run("force always reports work pending", func(t *testing.T) {
		c := &cobra.Command{}
		c.Flags().Bool("force", true, "")
		got, werr := indexingWorkPending(c, cfg, emb, t.TempDir(), nil, nil)
		if werr != nil {
			t.Fatalf("indexingWorkPending: %v", werr)
		}
		if !got {
			t.Error("force=true: pending = false, want true")
		}
	})

	t.Run("never-indexed target reports work pending", func(t *testing.T) {
		got, werr := indexingWorkPending(noForce(), cfg, emb, t.TempDir(), nil, nil)
		if werr != nil {
			t.Fatalf("indexingWorkPending: %v", werr)
		}
		if !got {
			t.Error("never-indexed: pending = false, want true")
		}
	})

	t.Run("already-fresh target reports no work pending", func(t *testing.T) {
		target := t.TempDir()
		if werr := os.WriteFile(filepath.Join(target, "a.go"), []byte("package a\n\nfunc Foo() {}\n"), 0o644); werr != nil {
			t.Fatal(werr)
		}
		// Build a real fresh index at the path indexingWorkPending will look up,
		// using a mock embedder so no backend is needed; IsFresh then matches.
		dbPath := config.DBPathForProject(target, model)
		if werr := os.MkdirAll(filepath.Dir(dbPath), 0o755); werr != nil {
			t.Fatal(werr)
		}
		idx, werr := index.NewIndexer(dbPath, &fakeEmbedder{dims: dims, model: model}, 0)
		if werr != nil {
			t.Fatalf("NewIndexer: %v", werr)
		}
		if _, werr := idx.Index(context.Background(), target, true, func(int, int, string) {}); werr != nil {
			t.Fatalf("Index: %v", werr)
		}
		_ = idx.Close()

		got, werr := indexingWorkPending(noForce(), cfg, emb, target, nil, nil)
		if werr != nil {
			t.Fatalf("indexingWorkPending: %v", werr)
		}
		if got {
			t.Error("already-fresh: pending = true, want false")
		}
	})
}

// fakeEmbedder is a no-network Embedder used to build a fresh index in tests.
type fakeEmbedder struct {
	dims  int
	model string
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		v := make([]float32, f.dims)
		if f.dims > 0 {
			v[0] = 1
		}
		out[i] = v
	}
	return out, nil
}

func (f *fakeEmbedder) Dimensions() int   { return f.dims }
func (f *fakeEmbedder) ModelName() string { return f.model }
