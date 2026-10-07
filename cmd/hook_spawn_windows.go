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
	"os"
	"os/exec"
	"syscall"
)

// createNoWindow is the Win32 CREATE_NO_WINDOW process-creation flag
// (0x08000000). Go's syscall package does not define a named constant for it.
// It runs a console-subsystem binary with no console window — the correct flag
// for suppressing a window, unlike DETACHED_PROCESS (0x8) which only severs the
// parent console and, per the Win32 "Process Creation Flags" docs, is mutually
// exclusive with CREATE_NO_WINDOW (the latter is ignored when both are set).
const createNoWindow = 0x08000000

// backgroundIndexerSysProcAttr returns the SysProcAttr used to launch the
// detached background indexer with no visible console window. Extracted so the
// flag choice is unit-testable: spawning DETACHED_PROCESS on the console-subsystem
// lumen.exe was the console-spawn half of the 2026-06-19 incident, and CI must
// catch any regression back to it.
func backgroundIndexerSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}

// spawnBackgroundIndexer launches "lumen index <projectPath>" as a detached,
// no-window background process on Windows. The spawned process acquires an
// advisory lock (via LockFileEx) before indexing, so concurrent calls are safe.
//
// Errors are silently ignored: background indexing is best-effort.
func spawnBackgroundIndexer(projectPath string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "index", projectPath)
	cmd.SysProcAttr = backgroundIndexerSysProcAttr()
	// Discard both streams. The child writes structured logs to debug.log itself
	// via slog (newDebugLogger); wiring its raw stderr into that same file would
	// interleave pterm progress text with the JSON log. The unix sibling
	// (hook_spawn_unix.go) sets both to nil for exactly this reason — see CLAUDE.md
	// "Output & Logging".
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return
	}
	// Reap the child to avoid a zombie / handle leak.
	go func() { _ = cmd.Wait() }()
}
