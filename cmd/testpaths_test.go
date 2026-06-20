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
	"path/filepath"
	"runtime"
)

// tp turns a forward-slash logical absolute path (e.g. "/project/src") into a
// platform-native ABSOLUTE path for tests. On Unix it is the identity. On
// Windows it gains a volume and backslash separators ("/project/src" ->
// "C:\project\src") so the value is both absolute (filepath.IsAbs true, which
// validateSearchInput requires) and separator-correct (matching the backslash
// forms that filepath.Dir/Rel produce during the walk). Using one helper for
// both the inputs and the expectations keeps a test's paths internally
// consistent on every OS without hardcoding either separator.
func tp(p string) string {
	if runtime.GOOS != "windows" {
		return p
	}
	return filepath.Clean(`C:` + filepath.ToSlash(p))
}
