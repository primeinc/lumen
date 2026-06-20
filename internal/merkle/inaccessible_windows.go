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
	"errors"
	"io/fs"
	"syscall"
)

// Windows file-locking errnos. A file held open by another process without
// read sharing (editors, antivirus scanners, any FILE_SHARE_NONE handle)
// fails to open with one of these — the practical Windows equivalent of a
// permission denial, and far more common than chmod-style denial on a
// case-insensitive, ACL-based filesystem.
const (
	errorSharingViolation = syscall.Errno(32) // ERROR_SHARING_VIOLATION
	errorLockViolation    = syscall.Errno(33) // ERROR_LOCK_VIOLATION
)

// IsInaccessibleErr reports whether err means a single file could not be read
// right now and should be skipped rather than aborting the whole index walk.
// On Windows that covers an access-denied permission error and a transient
// sharing/lock violation from a file another process holds open; the file is
// left out of this run and re-tried on the next one rather than failing the
// entire index pass. Shared by the merkle walk and the index orchestrator.
func IsInaccessibleErr(err error) bool {
	return errors.Is(err, fs.ErrPermission) ||
		errors.Is(err, errorSharingViolation) ||
		errors.Is(err, errorLockViolation)
}
