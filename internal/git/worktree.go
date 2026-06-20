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

// Package git provides utilities for detecting git worktrees and finding
// sibling worktree paths.
package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// IsWorktree reports whether projectPath is a git worktree (as opposed to
// the main working tree). A worktree has a .git file pointing at the shared
// .git directory, whereas the main repo has a .git directory.
func IsWorktree(projectPath string) bool {
	info, err := os.Lstat(filepath.Join(projectPath, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// CommonDir returns the shared .git directory for the repository containing
// projectPath, by running "git rev-parse --git-common-dir". Returns an error
// if git is not available or projectPath is not inside a git repository.
func CommonDir(projectPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-common-dir")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(projectPath, dir)
	}
	dir = filepath.Clean(dir)
	// Resolve symlinks so paths are comparable across macOS
	// /var → /private/var symlink boundaries.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return dir, nil
}

// InternalWorktreePaths returns the relative paths (relative to projectPath)
// of any sibling worktrees that are checked out inside projectPath. These
// should be excluded from indexing to avoid double-counting their files.
// Returns nil if git is unavailable, projectPath is not a repo, or no
// worktrees are nested inside it.
func InternalWorktreePaths(projectPath string) []string {
	worktrees, err := ListWorktrees(projectPath)
	if err != nil {
		return nil
	}

	resolvedRoot := projectPath
	if r, err := filepath.EvalSymlinks(projectPath); err == nil {
		resolvedRoot = r
	}

	var result []string
	for _, wt := range worktrees {
		if wt == resolvedRoot {
			continue
		}
		rel, err := filepath.Rel(resolvedRoot, wt)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		result = append(result, rel)
	}
	return result
}

// RepoRoot returns the absolute path of the root directory of the git
// repository containing projectPath, by running "git rev-parse --show-toplevel".
// Returns an error if git is not available or projectPath is not inside a git
// repository.
func RepoRoot(projectPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	root := strings.TrimSpace(string(out))
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return root, nil
}

// IsGitRoot reports whether path is a git repository or worktree root
// (i.e. contains a .git directory or .git file).
func IsGitRoot(path string) bool {
	_, err := os.Lstat(filepath.Join(path, ".git"))
	return err == nil
}

// DefaultMaxNestedRepos is the default cap on how many nested git repositories
// DiscoverNestedGitRepos returns from a single non-git root, overridable via the
// LUMEN_MAX_NESTED_REPOS environment variable. A non-git directory holding more
// than this many nested repos is not a meaningful single index root — it is a
// home or workspace directory, or the system temp tree — and the caller refuses
// such a root outright rather than indexing a partial, misleading subset.
const DefaultMaxNestedRepos = 64

// maxNestedReposLimit returns the nested-repo cap. A LUMEN_MAX_NESTED_REPOS
// value that parses as a positive integer overrides DefaultMaxNestedRepos; any
// other value (empty, zero, negative, or non-numeric) falls back to the default.
// The fallback is intentional and fail-safe — a malformed override never widens
// or disables the guard. It is deliberately not logged here: this helper runs on
// both the interactive `lumen index` path and the background indexer whose
// stderr is discarded, so a warning would either be swallowed or violate the
// repo's tui-vs-slog output separation. The refusal message names the variable,
// which is the operator's signal to re-check the value.
func maxNestedReposLimit() int {
	if v := os.Getenv("LUMEN_MAX_NESTED_REPOS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return DefaultMaxNestedRepos
}

// DiscoverNestedGitRepos walks rootPath and returns the absolute paths of nested
// directories that are git repo roots, up to the limit (LUMEN_MAX_NESTED_REPOS,
// default DefaultMaxNestedRepos). It stops descending into discovered repos.
// truncated is true when rootPath holds MORE than the limit; the caller should
// treat such a root as not a meaningful single index root and refuse it, rather
// than index a partial subset and leave the overflow to pollute the parent
// index. Returns (nil, false) if rootPath is itself a git root or has no nested
// repos.
func DiscoverNestedGitRepos(rootPath string) (repos []string, truncated bool) {
	if IsGitRoot(rootPath) {
		return nil, false
	}

	limit := maxNestedReposLimit()
	_ = filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Skip an unreadable subtree and keep counting elsewhere. This can
			// undercount nested repos hidden beneath an unreadable dir (so an
			// oversized root might not be refused as oversized), but the blast is
			// bounded: an error on rootPath itself makes the subsequent
			// merkle.BuildTree fail too, turning the index into a clean error
			// rather than a runaway, and canonical temp/$HOME roots are refused by
			// IsRootUnindexable regardless of this count.
			return filepath.SkipDir
		}
		if !d.IsDir() {
			return nil
		}
		if path == rootPath {
			return nil
		}
		if IsGitRoot(path) {
			if len(repos) >= limit {
				truncated = true
				return filepath.SkipAll
			}
			repos = append(repos, path)
			return filepath.SkipDir
		}
		return nil
	})
	return repos, truncated
}

// ListWorktrees returns the absolute paths of all worktrees (including the
// main working tree) for the repository containing projectPath. Returns nil
// if git is not available or projectPath is not inside a git repository.
func ListWorktrees(projectPath string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			// Resolve symlinks so paths are comparable across macOS
			// /var → /private/var symlink boundaries.
			if resolved, err := filepath.EvalSymlinks(path); err == nil {
				path = resolved
			}
			paths = append(paths, path)
		}
	}
	return paths, nil
}
