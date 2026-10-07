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
	"runtime"
	"testing"
)

// TestLongestAncestor covers the ancestor resolution purge uses to map a target
// path to the index that owns it, including the Windows case-insensitivity that
// the old strings.HasPrefix implementation got wrong (an index stored under one
// casing was invisible to a target reached via another, so the orphan survived
// a purge while the user believed it was removed).
func TestLongestAncestor(t *testing.T) {
	indexMap := map[string][]string{
		tp("/users/me/repo"):        {"h1"},
		tp("/users/me/repo/nested"): {"h2"},
		tp("/other"):                {"h3"},
	}

	cases := []struct {
		name   string
		target string
		want   string
	}{
		{"under a stored root", tp("/users/me/repo/src/file"), tp("/users/me/repo")},
		{"nearest (longest) ancestor wins", tp("/users/me/repo/nested/x"), tp("/users/me/repo/nested")},
		{"exact match", tp("/users/me/repo"), tp("/users/me/repo")},
		{"no ancestor", tp("/elsewhere/x"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := longestAncestor(indexMap, c.target); got != c.want {
				t.Errorf("longestAncestor(%q) = %q, want %q", c.target, got, c.want)
			}
		})
	}

	if runtime.GOOS == "windows" {
		t.Run("case-insensitive match on Windows", func(t *testing.T) {
			m := map[string][]string{`C:\Users\Me\Repo`: {"h1"}}
			// The index was written from a path with one casing; purge arrives
			// with another (its EvalSymlinks canonicalization vs the writer's raw
			// path). filepath.Rel folds case on Windows, so they must still match.
			if got := longestAncestor(m, `c:\users\me\repo\sub`); got != `C:\Users\Me\Repo` {
				t.Errorf("case-insensitive ancestor match failed: got %q, want %q", got, `C:\Users\Me\Repo`)
			}
		})
	}
}
