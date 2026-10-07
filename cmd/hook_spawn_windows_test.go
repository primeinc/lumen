//go:build windows

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
	"syscall"
	"testing"
)

// TestBackgroundIndexerSysProcAttr_NoWindow pins the console-window contract for
// the SessionStart background indexer. lumen.exe is a console-subsystem binary,
// so spawning it without CREATE_NO_WINDOW (or with DETACHED_PROCESS, which makes
// CREATE_NO_WINDOW ineffective) can flash a console window on every session —
// the console-spawn half of the 2026-06-19 incident.
func TestBackgroundIndexerSysProcAttr_NoWindow(t *testing.T) {
	const detachedProcess = 0x00000008 // DETACHED_PROCESS

	attr := backgroundIndexerSysProcAttr()

	if attr.CreationFlags&createNoWindow == 0 {
		t.Errorf("CreationFlags 0x%x is missing CREATE_NO_WINDOW (0x%x); the background indexer can spawn a visible console window",
			attr.CreationFlags, createNoWindow)
	}
	if attr.CreationFlags&detachedProcess != 0 {
		t.Errorf("CreationFlags 0x%x sets DETACHED_PROCESS (0x%x), which makes CREATE_NO_WINDOW ineffective on a console binary (the 2026-06-19 console-spawn regression)",
			attr.CreationFlags, detachedProcess)
	}
	if attr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Errorf("CreationFlags 0x%x is missing CREATE_NEW_PROCESS_GROUP (0x%x)",
			attr.CreationFlags, syscall.CREATE_NEW_PROCESS_GROUP)
	}
}
