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

//go:build !windows

package merkle

import (
	"os"
	"testing"
)

// makeFileUnreadable makes path return a permission error on read until the
// test ends. On Unix that is chmod(0). Returns false when the precondition
// cannot be established (running as root, which bypasses permission bits) so
// the caller can skip rather than assert a guarantee the OS won't honor.
func makeFileUnreadable(t *testing.T, path string) bool {
	t.Helper()
	if os.Getuid() == 0 {
		return false // root bypasses file permission checks
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	return true
}
