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

package config

import (
	"runtime"
	"testing"
)

// TestDBPathForProject_PathIdentity pins per-platform DB-path identity. On
// Windows the filesystem is case-insensitive and accepts forward slashes, so
// every spelling of one directory must map to a single index DB — otherwise the
// same repo reached via C:\Repo, c:\repo, and C:/Repo would build duplicate,
// mutually-invisible indexes and re-embed the whole tree each way. On Unix paths
// are case-sensitive, so distinct casings must remain distinct DBs and the
// existing index identity must not change.
func TestDBPathForProject_PathIdentity(t *testing.T) {
	base := t.TempDir()
	const model = "model"

	if runtime.GOOS == "windows" {
		variants := []string{
			`C:\Repo\Project`,
			`c:\repo\project`,
			`C:/Repo/Project`,
			`C:\REPO\PROJECT`,
		}
		want := DBPathForProjectBase(base, variants[0], model)
		for _, v := range variants[1:] {
			if got := DBPathForProjectBase(base, v, model); got != want {
				t.Errorf("Windows path variants must map to one index DB:\n  %q -> %s\n  %q -> %s",
					variants[0], want, v, got)
			}
		}
		return
	}

	// Unix: case-sensitive filesystem — different casings are different repos.
	if DBPathForProjectBase(base, "/Repo", model) == DBPathForProjectBase(base, "/repo", model) {
		t.Error("Unix paths are case-sensitive; /Repo and /repo must map to different index DBs")
	}
}
