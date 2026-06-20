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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ory/lumen/internal/index"
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
