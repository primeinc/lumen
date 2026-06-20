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
	"errors"
	"io/fs"
)

// IsInaccessibleErr reports whether err means a single file could not be read
// right now and should be skipped rather than aborting the whole index walk.
// On Unix that is a permission error; the file is left out of this run and
// re-tried on the next one. Shared by the merkle walk and the index
// orchestrator so both layers treat an unreadable file identically.
func IsInaccessibleErr(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}
