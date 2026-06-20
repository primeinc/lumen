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
	"fmt"
	"syscall"
	"testing"
)

// TestIsInaccessibleErr_Windows pins the Windows error classification that
// decides whether a single file is skipped (left for the next index pass) or
// aborts the whole walk. It runs unconditionally — unlike the integration
// permission tests, whose unreadable-file precondition some privileged CI
// runners cannot establish — so the contract is always validated on Windows.
func TestIsInaccessibleErr_Windows(t *testing.T) {
	const (
		errorFileNotFound  = syscall.Errno(2)  // ERROR_FILE_NOT_FOUND
		errorAccessDenied  = syscall.Errno(5)  // ERROR_ACCESS_DENIED -> fs.ErrPermission
		errorSharingViol   = syscall.Errno(32) // ERROR_SHARING_VIOLATION
		errorLockViolation = syscall.Errno(33) // ERROR_LOCK_VIOLATION
	)

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"sharing violation", errorSharingViol, true},
		{"lock violation", errorLockViolation, true},
		{"access denied (permission)", errorAccessDenied, true},
		{"wrapped sharing violation", fmt.Errorf("read x: %w", errorSharingViol), true},
		{"file not found", errorFileNotFound, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := IsInaccessibleErr(tc.err); got != tc.want {
			t.Errorf("%s: IsInaccessibleErr(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}
