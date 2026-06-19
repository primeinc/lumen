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

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestIsWorktree_GitDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if IsWorktree(dir) {
		t.Fatal("expected false for .git directory")
	}
}

func TestIsWorktree_GitFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /some/path\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsWorktree(dir) {
		t.Fatal("expected true for .git file")
	}
}

func TestIsWorktree_NoGit(t *testing.T) {
	dir := t.TempDir()
	if IsWorktree(dir) {
		t.Fatal("expected false when .git does not exist")
	}
}

func TestListWorktrees(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	// Create a real git repo.
	main := t.TempDir()
	run(t, main, "git", "init")
	run(t, main, "git", "commit", "--allow-empty", "-m", "init")

	// Add a worktree.
	wt := filepath.Join(t.TempDir(), "wt")
	run(t, main, "git", "worktree", "add", wt)

	// Resolve symlinks for comparison (macOS /var → /private/var).
	wtResolved, err := filepath.EvalSymlinks(wt)
	if err != nil {
		t.Fatal(err)
	}

	paths, err := ListWorktrees(main)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 2 {
		t.Fatalf("expected ≥2 worktrees, got %d: %v", len(paths), paths)
	}

	found := false
	for _, p := range paths {
		if p == wtResolved {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected worktree %q in list %v", wtResolved, paths)
	}
}

func TestCommonDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	main := t.TempDir()
	run(t, main, "git", "init")
	run(t, main, "git", "commit", "--allow-empty", "-m", "init")

	wt := filepath.Join(t.TempDir(), "wt")
	run(t, main, "git", "worktree", "add", wt)

	commonFromMain, err := CommonDir(main)
	if err != nil {
		t.Fatal(err)
	}
	commonFromWT, err := CommonDir(wt)
	if err != nil {
		t.Fatal(err)
	}

	if commonFromMain != commonFromWT {
		t.Fatalf("expected same common dir, got %q and %q", commonFromMain, commonFromWT)
	}
}

func TestInternalWorktreePaths_InternalWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	main := t.TempDir()
	run(t, main, "git", "init")
	run(t, main, "git", "commit", "--allow-empty", "-m", "init")

	// Create a worktree INSIDE the main repo directory.
	internalWt := filepath.Join(main, ".worktrees", "feature")
	run(t, main, "git", "worktree", "add", internalWt)

	paths := InternalWorktreePaths(main)
	if len(paths) != 1 {
		t.Fatalf("expected 1 internal worktree path, got %d: %v", len(paths), paths)
	}
	want := filepath.Join(".worktrees", "feature")
	if paths[0] != want {
		t.Errorf("expected %q, got %q", want, paths[0])
	}
}

func TestInternalWorktreePaths_ExternalWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	main := t.TempDir()
	run(t, main, "git", "init")
	run(t, main, "git", "commit", "--allow-empty", "-m", "init")

	// Create a worktree OUTSIDE the main repo directory.
	externalWt := filepath.Join(t.TempDir(), "feature")
	run(t, main, "git", "worktree", "add", externalWt)

	paths := InternalWorktreePaths(main)
	if len(paths) != 0 {
		t.Errorf("expected 0 internal worktree paths for external worktree, got %v", paths)
	}
}

func TestInternalWorktreePaths_NotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	paths := InternalWorktreePaths(dir)
	if len(paths) != 0 {
		t.Errorf("expected nil for non-repo, got %v", paths)
	}
}

func TestListWorktrees_NotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dir := t.TempDir()
	paths, err := ListWorktrees(dir)
	if err == nil {
		t.Fatalf("expected error, got paths: %v", paths)
	}
}

func TestIsGitRoot_WithGitDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsGitRoot(dir) {
		t.Fatal("expected true for directory with .git dir")
	}
}

func TestIsGitRoot_WithGitFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /some/path\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsGitRoot(dir) {
		t.Fatal("expected true for directory with .git file (worktree)")
	}
}

func TestIsGitRoot_NoGit(t *testing.T) {
	dir := t.TempDir()
	if IsGitRoot(dir) {
		t.Fatal("expected false when no .git exists")
	}
}

func TestDiscoverNestedGitRepos_FindsNestedRepos(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	// Create a non-git parent with two git sub-repos.
	parent := t.TempDir()
	repoA := filepath.Join(parent, "sub-a")
	repoB := filepath.Join(parent, "sub-b")
	if err := os.Mkdir(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(repoB, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, repoA, "git", "init")
	run(t, repoB, "git", "init")

	repos, _ := DiscoverNestedGitRepos(parent)
	if len(repos) != 2 {
		t.Fatalf("expected 2 nested repos, got %d: %v", len(repos), repos)
	}

	// Resolve symlinks for comparison.
	resolvedA, _ := filepath.EvalSymlinks(repoA)
	resolvedB, _ := filepath.EvalSymlinks(repoB)
	want := map[string]bool{resolvedA: true, resolvedB: true}
	for _, r := range repos {
		resolved, _ := filepath.EvalSymlinks(r)
		if !want[resolved] {
			t.Errorf("unexpected repo path %q", r)
		}
	}
}

func TestDiscoverNestedGitRepos_SkipsWhenRootIsGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	// Parent is itself a git repo — should return nil.
	parent := t.TempDir()
	run(t, parent, "git", "init")

	sub := filepath.Join(parent, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, sub, "git", "init")

	repos, _ := DiscoverNestedGitRepos(parent)
	if len(repos) != 0 {
		t.Fatalf("expected nil when root is a git repo, got %v", repos)
	}
}

func TestDiscoverNestedGitRepos_DeeplyNested(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	// Non-git parent, with a git repo nested two levels deep.
	parent := t.TempDir()
	deep := filepath.Join(parent, "a", "b", "repo")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, deep, "git", "init")

	repos, _ := DiscoverNestedGitRepos(parent)
	if len(repos) != 1 {
		t.Fatalf("expected 1 nested repo, got %d: %v", len(repos), repos)
	}

	resolved, _ := filepath.EvalSymlinks(deep)
	got, _ := filepath.EvalSymlinks(repos[0])
	if got != resolved {
		t.Errorf("expected %q, got %q", resolved, got)
	}
}

func TestDiscoverNestedGitRepos_NoNestedRepos(t *testing.T) {
	parent := t.TempDir()
	repos, _ := DiscoverNestedGitRepos(parent)
	if len(repos) != 0 {
		t.Fatalf("expected nil for directory with no nested repos, got %v", repos)
	}
}

func TestDiscoverNestedGitRepos_DoesNotDescendIntoGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	// Non-git parent with a git repo that itself contains another git repo.
	// Only the outer one should be discovered.
	parent := t.TempDir()
	outer := filepath.Join(parent, "outer")
	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, outer, "git", "init")
	run(t, inner, "git", "init")

	repos, _ := DiscoverNestedGitRepos(parent)
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (outer only), got %d: %v", len(repos), repos)
	}

	resolved, _ := filepath.EvalSymlinks(outer)
	got, _ := filepath.EvalSymlinks(repos[0])
	if got != resolved {
		t.Errorf("expected %q, got %q", resolved, got)
	}
}

func TestDiscoverNestedGitRepos_BoundsRunawayWalk(t *testing.T) {
	// Regression for the runaway-indexing incident: a non-git root holding more
	// nested repos than the limit must report truncated=true and cap the returned
	// set, so the caller can refuse the root entirely instead of indexing a
	// partial subset (and leaking the overflow into the parent index). A .git dir
	// satisfies IsGitRoot, so fake repos keep this fast and git-free.
	makeRepos := func(t *testing.T, n int) string {
		t.Helper()
		parent := t.TempDir()
		for i := range n {
			repo := filepath.Join(parent, "repo"+strconv.Itoa(i))
			if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		return parent
	}

	t.Run("caps and reports truncation past the default limit", func(t *testing.T) {
		parent := makeRepos(t, DefaultMaxNestedRepos+25)
		repos, truncated := DiscoverNestedGitRepos(parent)
		if !truncated {
			t.Errorf("truncated = false, want true for %d > %d nested repos", DefaultMaxNestedRepos+25, DefaultMaxNestedRepos)
		}
		if len(repos) != DefaultMaxNestedRepos {
			t.Errorf("returned %d repos, want the cap %d", len(repos), DefaultMaxNestedRepos)
		}
	})

	t.Run("not truncated at or below the limit", func(t *testing.T) {
		parent := makeRepos(t, 5)
		repos, truncated := DiscoverNestedGitRepos(parent)
		if truncated {
			t.Error("truncated = true, want false for 5 nested repos")
		}
		if len(repos) != 5 {
			t.Errorf("returned %d repos, want 5", len(repos))
		}
	})

	t.Run("LUMEN_MAX_NESTED_REPOS overrides the limit", func(t *testing.T) {
		t.Setenv("LUMEN_MAX_NESTED_REPOS", "10")
		parent := makeRepos(t, 15)
		repos, truncated := DiscoverNestedGitRepos(parent)
		if !truncated {
			t.Error("truncated = false, want true for 15 > 10 (override)")
		}
		if len(repos) != 10 {
			t.Errorf("returned %d repos, want the override cap 10", len(repos))
		}
	})

	t.Run("boundary is exact at the cap (cap vs cap+1)", func(t *testing.T) {
		// Pin the off-by-one in `len(repos) >= limit`: exactly `limit` repos must
		// NOT truncate, and exactly limit+1 MUST. The other subtests use loose
		// gaps (cap+25, 5-vs-64) and would not catch a one-off boundary slip. Use
		// a small override so the test stays fast.
		t.Setenv("LUMEN_MAX_NESTED_REPOS", "4")

		atLimit := makeRepos(t, 4)
		repos, truncated := DiscoverNestedGitRepos(atLimit)
		if truncated {
			t.Error("truncated = true at exactly the limit (4), want false")
		}
		if len(repos) != 4 {
			t.Errorf("returned %d repos at the limit, want 4", len(repos))
		}

		overLimit := makeRepos(t, 5)
		repos, truncated = DiscoverNestedGitRepos(overLimit)
		if !truncated {
			t.Error("truncated = false at limit+1 (5), want true")
		}
		if len(repos) != 4 {
			t.Errorf("returned %d repos at limit+1, want the cap 4", len(repos))
		}
	})
}

// TestMaxNestedReposLimit_FailsSafeOnMalformedEnv pins the security-relevant
// fail-safe: a malformed LUMEN_MAX_NESTED_REPOS must never widen or disable the
// runaway-walk guard. The existing discovery tests only ever set positive
// overrides, so a regression that returned 0 (uncapped) or honored a negative
// would pass unnoticed. Only a value that parses as a positive int may override.
func TestMaxNestedReposLimit_FailsSafeOnMalformedEnv(t *testing.T) {
	for _, v := range []string{"", "0", "-1", "-64", "abc", "  ", "64x", "99999999999999999999"} {
		t.Setenv("LUMEN_MAX_NESTED_REPOS", v)
		if got := maxNestedReposLimit(); got != DefaultMaxNestedRepos {
			t.Errorf("maxNestedReposLimit() with LUMEN_MAX_NESTED_REPOS=%q = %d, want default %d", v, got, DefaultMaxNestedRepos)
		}
	}

	t.Setenv("LUMEN_MAX_NESTED_REPOS", "7")
	if got := maxNestedReposLimit(); got != 7 {
		t.Errorf("maxNestedReposLimit() with valid override %q = %d, want 7", "7", got)
	}
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
