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

//go:build windows

package merkle

import (
	"os"
	"syscall"
	"testing"
)

// makeFileUnreadable holds an exclusive (no-share) handle on path so any
// subsequent open -- including the indexer's os.ReadFile -- fails with
// ERROR_SHARING_VIOLATION, the Windows equivalent of a permission-denied /
// locked file (an editor or antivirus holding the file open). chmod(0) does NOT
// deny reads on Windows.
//
// Returns false (the caller skips) when the precondition cannot be established:
// the exclusive open fails, or -- probed explicitly -- the handle does not
// actually block a read in this environment. Some CI runners execute with a
// privileged token that bypasses the share restriction; there a file genuinely
// cannot be made unreadable, so the test skips honestly rather than asserting a
// guarantee the OS will not honor. The classification itself is covered
// unconditionally by TestIsInaccessibleErr_Windows.
func makeFileUnreadable(t *testing.T, path string) bool {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(
		p,
		syscall.GENERIC_READ,
		0, // dwShareMode == 0: deny all other opens while held
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return false // could not take an exclusive handle
	}
	if _, rerr := os.ReadFile(path); rerr == nil {
		// The exclusive handle did not block a read here (privileged/bypassing
		// token) -- can't establish the precondition.
		_ = syscall.CloseHandle(h)
		return false
	}
	t.Cleanup(func() { _ = syscall.CloseHandle(h) })
	return true
}
