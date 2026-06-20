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

package merkle

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMakeSkip_GitignorePatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "*.log\nbuild/\n")

	skip := MakeSkip(dir, []string{".go", ".log"})

	// .log files should be skipped by gitignore even though extension is allowed
	if !skip("app.log", false) {
		t.Error("expected app.log to be skipped via .gitignore")
	}
	// build/ directory should be skipped by gitignore
	if !skip("build", true) {
		t.Error("expected build/ to be skipped via .gitignore")
	}
	// .go files should pass
	if skip("main.go", false) {
		t.Error("expected main.go to pass")
	}
	// .txt files should be skipped by extension filter (not in exts)
	if !skip("readme.txt", false) {
		t.Error("expected readme.txt to be skipped by extension filter")
	}
}

func TestMakeSkip_NoGitignore(t *testing.T) {
	dir := t.TempDir()
	// No .gitignore created

	skip := MakeSkip(dir, []string{".go", ".py"})

	if skip("main.go", false) {
		t.Error("expected main.go to pass without .gitignore")
	}
	if skip("script.py", false) {
		t.Error("expected script.py to pass without .gitignore")
	}
	if !skip("readme.md", false) {
		t.Error("expected readme.md to be skipped by extension filter")
	}
	// Hardcoded dirs still skipped
	if !skip("node_modules", true) {
		t.Error("expected node_modules to be skipped")
	}
}

func TestMakeSkip_NegationPattern(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "*.gen.go\n!important.gen.go\n")

	skip := MakeSkip(dir, []string{".go"})

	if !skip("foo.gen.go", false) {
		t.Error("expected foo.gen.go to be skipped via .gitignore")
	}
	if skip("important.gen.go", false) {
		t.Error("expected important.gen.go to pass via negation pattern")
	}
	if skip("main.go", false) {
		t.Error("expected main.go to pass")
	}
}

func TestIsRootUnindexable(t *testing.T) {
	// .lumenignore-based scenarios: each case writes the given contents (or no
	// file when ignoreContents is the sentinel "<no file>") to a temp dir.
	const noFile = "<no file>"
	lumenIgnoreCases := []struct {
		name           string
		ignoreContents string
		want           bool
		wantReason     string
	}{
		{"no .lumenignore", noFile, false, ""},
		{"empty .lumenignore", "", false, ""},
		{"specific patterns", "node_modules/\n*.log\nbuild/\n", false, ""},
		{"doublestar matches everything", "**\n", true, ".lumenignore catch-all pattern"},
		{"doublestar-slash-star matches everything", "**/*\n", true, ".lumenignore catch-all pattern"},
		// The user's actual $HOME case — multiple broad patterns combined.
		{"combined catch-all patterns", "*\n**/**\n*/*\n**/*\n", true, ".lumenignore catch-all pattern"},
		// In gitignore semantics, a bare `*` (no `/`) matches any name at any
		// depth — so it ignores every file in the tree. That is a catch-all.
		{"single-star alone", "*\n", true, ".lumenignore catch-all pattern"},
		{"comments and blank lines do not trigger", "# comment\n\n  \n", false, ""},
	}
	for _, tc := range lumenIgnoreCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.ignoreContents != noFile {
				writeFile(t, dir, ".lumenignore", tc.ignoreContents)
			}
			got, reason := IsRootUnindexable(dir)
			if got != tc.want {
				t.Errorf("IsRootUnindexable() = %v, want %v", got, tc.want)
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}

	t.Run("hardcoded system paths are refused", func(t *testing.T) {
		// These paths should always be refused as index roots, regardless of
		// whether a .lumenignore exists. Indexing them is never the user's
		// intent and on macOS would trigger TCC prompts.
		//
		// The list is platform-aware: Unix paths only assert refusal when the
		// path exists on disk (so /Applications and /Library don't fail on
		// Linux), and Windows paths only run on Windows.
		unixPaths := []string{
			"/",
			"/Users",
			"/home",
			"/tmp",
			"/private/tmp",
			"/var",
			"/private/var",
			"/etc",
			"/usr",
			"/opt",
			"/Applications",
			"/Library",
			"/System",
		}
		windowsPaths := []string{
			`C:\`,
			`C:\Windows`,
			`C:\Program Files`,
			`C:\Program Files (x86)`,
			`C:\Users`,
			`C:\ProgramData`,
		}
		var paths []string
		if runtime.GOOS == "windows" {
			paths = windowsPaths
		} else {
			paths = unixPaths
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err != nil {
				// Skip paths that don't exist on this host — EvalSymlinks
				// will fail there and the fallback to filepath.Clean still
				// catches them, but we keep the assertion focused on real
				// roots that users could actually point lumen at.
				continue
			}
			got, reason := IsRootUnindexable(p)
			if !got {
				t.Errorf("expected %q to be refused as an index root", p)
			}
			if reason != "hardcoded system root" {
				t.Errorf("reason for %q = %q, want %q", p, reason, "hardcoded system root")
			}
		}
	})

	t.Run("system temp directory is refused", func(t *testing.T) {
		// Regression for the 2026-06-19 runaway-indexing incident: Lumen indexed
		// C:\WINDOWS\TEMP and walked its entire nested-repo tree. The OS temp dir
		// must be refused, and — independent of this process's TEMP/TMP — so must
		// the Windows SYSTEM temp dirs %SystemRoot%\Temp and %SystemRoot%\SystemTemp
		// (the dirs a service/background process gets per Win32 GetTempPath /
		// GetTempPath2). Deriving them from %SystemRoot% rather than hardcoding
		// C:\WINDOWS\TEMP keeps this deterministic on any host, including a clean
		// runner where os.TempDir() is %LOCALAPPDATA%\Temp.
		if got, reason := IsRootUnindexable(os.TempDir()); !got || reason == "" {
			t.Errorf("expected os.TempDir() %q to be refused, got=%v reason=%q", os.TempDir(), got, reason)
		}
		if runtime.GOOS != "windows" {
			return
		}
		candidates := []string{os.Getenv("TEMP"), os.Getenv("TMP")}
		if sysRoot := os.Getenv("SystemRoot"); sysRoot != "" {
			candidates = append(candidates, filepath.Join(sysRoot, "Temp"), filepath.Join(sysRoot, "SystemTemp"))
		}
		for _, p := range candidates {
			if p == "" {
				continue
			}
			// SystemTemp only exists on newer builds; skip any candidate that is
			// not present rather than asserting on a dir the host does not have.
			if _, err := os.Stat(p); err != nil {
				continue
			}
			got, reason := IsRootUnindexable(p)
			if !got {
				t.Errorf("expected Windows system temp dir %q to be refused as an index root", p)
			}
			if reason != "system temporary directory" {
				t.Errorf("reason for %q = %q, want %q", p, reason, "system temporary directory")
			}
		}
	})

	t.Run("windows system root match is case-insensitive", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("windows-only path casing")
		}
		// The OS reports C:\WINDOWS while the refusal list holds C:\Windows; the
		// exact-key map missed this. Refusal must be case-insensitive on Windows.
		got, reason := IsRootUnindexable(`C:\WINDOWS`)
		if !got {
			t.Errorf("expected C:\\WINDOWS to be refused via case-insensitive match")
		}
		if reason != "hardcoded system root" {
			t.Errorf("reason = %q, want %q", reason, "hardcoded system root")
		}
	})

	t.Run("symlink to home is refused", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("no home dir available")
		}
		linkParent := t.TempDir()
		link := filepath.Join(linkParent, "home-symlink")
		if err := os.Symlink(home, link); err != nil {
			t.Skipf("symlinks unsupported on this platform: %v", err)
		}
		got, reason := IsRootUnindexable(link)
		if !got {
			t.Errorf("expected %q (symlink to home) to be refused", link)
		}
		if reason != "user home directory" {
			t.Errorf("reason = %q, want %q", reason, "user home directory")
		}
	})

	t.Run("home directory is refused", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("no home dir available")
		}
		got, reason := IsRootUnindexable(home)
		if !got {
			t.Errorf("expected %q (home) to be refused as an index root", home)
		}
		if reason != "user home directory" {
			t.Errorf("reason = %q, want %q", reason, "user home directory")
		}
	})

	t.Run("hardcoded refusal does not falsely flag siblings of home", func(t *testing.T) {
		// A directory that is NOT in the refusal list and has no .lumenignore
		// must still be indexable. Use the test's tempdir which is outside any
		// hardcoded path.
		dir := t.TempDir()
		if got, _ := IsRootUnindexable(dir); got {
			t.Errorf("expected %q to be indexable (no .lumenignore, not hardcoded)", dir)
		}
	})
}

// TestIsRootUnindexable_RefusesCanonicalWindowsTempIndependentOfEnv proves the
// canonical Windows machine-temp roots are refused by directory identity even
// when no temp-related environment points at them — the gap where a normal-user
// os.TempDir() is %LOCALAPPDATA%\Temp and %SystemRoot% is unset. systemTempDirs
// adds these as env-independent literals, matched by the case-folded string
// fallback on any OS, so this assertion executes on Linux/macOS CI as well.
func TestIsRootUnindexable_RefusesCanonicalWindowsTempIndependentOfEnv(t *testing.T) {
	t.Setenv("TEMP", "")
	t.Setenv("TMP", "")
	t.Setenv("SystemRoot", "")
	t.Setenv("SystemTemp", "")

	for _, p := range []string{`C:\Windows\Temp`, `C:\Windows\SystemTemp`} {
		got, reason := IsRootUnindexable(p)
		if !got {
			t.Errorf("IsRootUnindexable(%q) = false, want true (env-independent refusal)", p)
		}
		if reason != "system temporary directory" {
			t.Errorf("reason for %q = %q, want %q", p, reason, "system temporary directory")
		}
	}
}

// TestMatchesRefusedRoot_CaseInsensitiveOnWindows verifies that a refused root
// supplied in non-canonical casing still matches. On a real Windows disk this is
// resolved by os.SameFile inside sameDir: C:\PROGRAMDATA and the refusedRoots
// key C:\ProgramData are the same directory on a case-insensitive filesystem, so
// they share a file ID. The absent / unconfirmed path that falls back to a
// case-folded string compare is covered directly by TestSameDir.
func TestMatchesRefusedRoot_CaseInsensitiveOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case-insensitive root matching is exercised on Windows")
	}
	if !matchesRefusedRoot(`C:\PROGRAMDATA`, `C:\PROGRAMDATA`, `C:\PROGRAMDATA`) {
		t.Error(`matchesRefusedRoot("C:\PROGRAMDATA") = false, want true (refusedRoots holds "C:\ProgramData"; matched via os.SameFile)`)
	}
	if matchesRefusedRoot(`C:\Users\someone\a-project`, `C:\Users\someone\a-project`, `C:\Users\someone\a-project`) {
		t.Error("matchesRefusedRoot(a normal project path) = true, want false")
	}
}

// TestSameDir covers sameDir directly — in particular the string-compare
// fallback that runs whenever os.SameFile cannot confirm identity (either path
// absent, or stat-able but not openable). That fallback is the only guard for
// refused roots not present on this host, so it must have executing coverage.
func TestSameDir(t *testing.T) {
	tmp := t.TempDir()

	if !sameDir(tmp, tmp) {
		t.Error("sameDir(tmp, tmp) = false, want true for an existing directory")
	}

	absent := filepath.Join(tmp, "does-not-exist-abc")
	absentSame := filepath.Join(tmp, "does-not-exist-abc")
	if !sameDir(absent, absentSame) {
		t.Error("sameDir(identical absent paths) = false, want true")
	}
	absentOther := filepath.Join(tmp, "does-not-exist-xyz")
	if sameDir(absent, absentOther) {
		t.Error("sameDir(distinct absent paths) = true, want false")
	}

	absentUpper := filepath.Join(tmp, "DOES-NOT-EXIST-ABC")
	if runtime.GOOS == "windows" {
		if !sameDir(absent, absentUpper) {
			t.Error("sameDir(absent case-variant paths) = false on Windows, want true (closes the case bypass for absent roots)")
		}
	} else if sameDir(absent, absentUpper) {
		t.Error("sameDir(absent case-variant paths) = true on a case-sensitive OS, want false")
	}
}

// TestSameDirWithInfo_MatchesViaResolvedFormWhenIdentityUnconfirmed guards the
// $HOME / system-temp refusal against the degraded path: when os.SameFile cannot
// confirm identity (aErr != nil — the Windows loadFileId failure for an
// un-openable directory), the string fallback must still match a path that
// equals the target only after symlink resolution, using the input's RESOLVED
// form. A clean-vs-clean compare alone would miss it and let the guard escape.
func TestSameDirWithInfo_MatchesViaResolvedFormWhenIdentityUnconfirmed(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}
	cleanA := filepath.Clean(link)
	resolvedA := resolvePath(link) // resolves through the symlink to target
	if pathsEqual(cleanA, filepath.Clean(target)) {
		t.Fatal("precondition: the symlink path should differ from its target")
	}
	// aErr != nil forces the string fallback (os.SameFile branch skipped).
	if !sameDirWithInfo(nil, os.ErrPermission, cleanA, resolvedA, target) {
		t.Error("sameDirWithInfo did not match via the resolved form; the $HOME/temp guard is narrowed")
	}
}

func TestMakeSkip_HardcodedFiles(t *testing.T) {
	dir := t.TempDir()
	skip := MakeSkip(dir, []string{".go", ".json", ".yaml"})

	for name := range SkipFiles {
		if !skip(name, false) {
			t.Errorf("expected hardcoded file %q to be skipped", name)
		}
	}

	// Regular files with same extensions should pass
	if skip("package.json", false) {
		t.Error("expected package.json to pass")
	}
	if skip("main.go", false) {
		t.Error("expected main.go to pass")
	}
}

func TestMakeSkip_HardcodedDirs(t *testing.T) {
	dir := t.TempDir()
	skip := MakeSkip(dir, []string{".go"})

	for name := range SkipDirs {
		if !skip(name, true) {
			t.Errorf("expected hardcoded dir %q to be skipped", name)
		}
	}

	// Non-skipped dirs should pass
	if skip("src", true) {
		t.Error("expected src/ to pass")
	}
	if skip("pkg", true) {
		t.Error("expected pkg/ to pass")
	}
}

func TestMakeSkip_NestedGitignore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "*.log\n")
	writeFile(t, dir, "sub/.gitignore", "secret.go\n")
	writeFile(t, dir, "sub/main.go", "package sub\n")
	writeFile(t, dir, "sub/secret.go", "package sub\n")

	skip := MakeSkip(dir, []string{".go", ".log"})

	// Root .gitignore applies everywhere
	if !skip("app.log", false) {
		t.Error("expected app.log to be skipped via root .gitignore")
	}
	if !skip("sub/app.log", false) {
		t.Error("expected sub/app.log to be skipped via root .gitignore")
	}
	// Nested .gitignore applies in its directory
	if !skip("sub/secret.go", false) {
		t.Error("expected sub/secret.go to be skipped via nested .gitignore")
	}
	// Nested .gitignore does not affect root
	if skip("secret.go", false) {
		t.Error("expected root secret.go to pass (nested .gitignore doesn't apply at root)")
	}
	// Normal file in sub passes
	if skip("sub/main.go", false) {
		t.Error("expected sub/main.go to pass")
	}
}

func TestMakeSkip_LumenIgnore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".lumenignore", "generated/\n*.pb.go\n")

	skip := MakeSkip(dir, []string{".go"})

	if !skip("generated", true) {
		t.Error("expected generated/ to be skipped via .lumenignore")
	}
	if !skip("foo.pb.go", false) {
		t.Error("expected foo.pb.go to be skipped via .lumenignore")
	}
	if skip("main.go", false) {
		t.Error("expected main.go to pass")
	}
}

func TestMakeSkip_NestedLumenIgnore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "api/.lumenignore", "mock_*.go\n")
	writeFile(t, dir, "api/handler.go", "package api\n")
	writeFile(t, dir, "api/mock_handler.go", "package api\n")

	skip := MakeSkip(dir, []string{".go"})

	if !skip("api/mock_handler.go", false) {
		t.Error("expected api/mock_handler.go to be skipped via nested .lumenignore")
	}
	if skip("api/handler.go", false) {
		t.Error("expected api/handler.go to pass")
	}
	// Root level mock_ files are not affected
	if skip("mock_handler.go", false) {
		t.Error("expected root mock_handler.go to pass (nested ignore doesn't apply)")
	}
}

func TestMakeSkip_GitattributesGenerated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitattributes", "bench-results linguist-generated\ndocs/plans linguist-generated=true\n")

	skip := MakeSkip(dir, []string{".go", ".txt"})

	if !skip("bench-results/data.txt", false) {
		t.Error("expected bench-results/data.txt to be skipped via linguist-generated")
	}
	if !skip("docs/plans/plan.txt", false) {
		t.Error("expected docs/plans/plan.txt to be skipped via linguist-generated=true")
	}
	if skip("src/main.go", false) {
		t.Error("expected src/main.go to pass")
	}
}

func TestMakeSkip_GitattributesNestedDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pkg/.gitattributes", "generated_*.go linguist-generated\n")

	skip := MakeSkip(dir, []string{".go"})

	if !skip("pkg/generated_types.go", false) {
		t.Error("expected pkg/generated_types.go to be skipped via nested .gitattributes")
	}
	if skip("pkg/types.go", false) {
		t.Error("expected pkg/types.go to pass")
	}
	// Root level is not affected
	if skip("generated_types.go", false) {
		t.Error("expected root generated_types.go to pass")
	}
}

func TestMakeSkip_GitattributesNonGenerated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitattributes", "*.go linguist-language=Go\nkeep.go linguist-generated=false\n")

	skip := MakeSkip(dir, []string{".go"})

	// Non-generated attributes should not cause skipping
	if skip("main.go", false) {
		t.Error("expected main.go to pass (linguist-language is not linguist-generated)")
	}
	if skip("keep.go", false) {
		t.Error("expected keep.go to pass (linguist-generated=false)")
	}
}

func TestMakeSkip_AllLayersCombined(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "*.log\n")
	writeFile(t, dir, ".lumenignore", "scratch/\n")
	writeFile(t, dir, ".gitattributes", "generated.go linguist-generated\n")
	writeFile(t, dir, "sub/.gitignore", "tmp_*.go\n")

	skip := MakeSkip(dir, []string{".go", ".log"})

	// Layer 1: SkipDirs
	if !skip("node_modules", true) {
		t.Error("expected node_modules to be skipped (SkipDirs)")
	}
	// Layer 2: .gitignore (root)
	if !skip("debug.log", false) {
		t.Error("expected debug.log to be skipped (.gitignore)")
	}
	// Layer 2: .gitignore (nested)
	if !skip("sub/tmp_data.go", false) {
		t.Error("expected sub/tmp_data.go to be skipped (nested .gitignore)")
	}
	// Layer 3: .lumenignore
	if !skip("scratch", true) {
		t.Error("expected scratch/ to be skipped (.lumenignore)")
	}
	// Layer 4: .gitattributes
	if !skip("generated.go", false) {
		t.Error("expected generated.go to be skipped (.gitattributes)")
	}
	// Layer 5: extension filter
	if !skip("readme.md", false) {
		t.Error("expected readme.md to be skipped (extension filter)")
	}
	// Normal file passes all layers
	if skip("main.go", false) {
		t.Error("expected main.go to pass all layers")
	}
}

func TestParseLinguistExcluded(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		match   []string // paths that should match
		noMatch []string // paths that should not match
	}{
		{
			name:    "bare linguist-generated",
			content: "bench-results linguist-generated\n",
			match:   []string{"bench-results"},
			noMatch: []string{"other"},
		},
		{
			name:    "linguist-generated=true",
			content: "docs/plans linguist-generated=true\n",
			match:   []string{"docs/plans"},
			noMatch: []string{"docs/other"},
		},
		{
			name:    "linguist-generated=false ignored",
			content: "keep.go linguist-generated=false\n",
			noMatch: []string{"keep.go"},
		},
		{
			name:    "negated -linguist-generated ignored",
			content: "keep.go -linguist-generated\n",
			noMatch: []string{"keep.go"},
		},
		{
			name:    "multiple attributes",
			content: "*.pb.go linguist-generated text\n",
			match:   []string{"foo.pb.go"},
			noMatch: []string{"foo.go"},
		},
		{
			name:    "comments and blanks",
			content: "# comment\n\n*.gen.go linguist-generated\n",
			match:   []string{"foo.gen.go"},
			noMatch: []string{"foo.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name+".gitattributes")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			gi := parseLinguistExcluded(path)

			for _, p := range tt.match {
				if gi == nil || !gi.MatchesPath(p) {
					t.Errorf("expected %q to match", p)
				}
			}
			for _, p := range tt.noMatch {
				if gi != nil && gi.MatchesPath(p) {
					t.Errorf("expected %q to NOT match", p)
				}
			}
		})
	}

	// Non-existent file
	if gi := parseLinguistExcluded(filepath.Join(dir, "nonexistent")); gi != nil {
		t.Error("expected nil for non-existent file")
	}
}

func TestParseLinguistAttributes_Vendored(t *testing.T) {
	dir := t.TempDir()
	attrs := "third_party/* linguist-vendored\ngenerated.go linguist-generated=true\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte(attrs), 0o644); err != nil {
		t.Fatal(err)
	}

	tree := NewIgnoreTree(dir, []string{".go"})
	if !tree.shouldSkip("third_party/lib.go", false) {
		t.Error("linguist-vendored file should be skipped")
	}
	if !tree.shouldSkip("generated.go", false) {
		t.Error("linguist-generated=true file should be skipped")
	}
	if tree.shouldSkip("normal.go", false) {
		t.Error("normal.go should not be skipped")
	}
}

func TestMakeSkipWithExtra_SkipsWorktreePaths(t *testing.T) {
	dir := t.TempDir()

	extraPaths := []string{".worktrees/feature", ".worktrees/bugfix"}
	skip := MakeSkipWithExtra(dir, []string{".go"}, extraPaths)

	// Extra paths should be skipped as directories
	if !skip(".worktrees/feature", true) {
		t.Error("expected .worktrees/feature dir to be skipped")
	}
	if !skip(".worktrees/bugfix", true) {
		t.Error("expected .worktrees/bugfix dir to be skipped")
	}
	// Files are not subject to the extra skip (directory pruning handles it)
	if skip("main.go", false) {
		t.Error("expected main.go to pass")
	}
	// A similarly named dir NOT in the skip list should pass
	if skip(".worktrees/other", true) {
		t.Error("expected .worktrees/other to pass (not in extra skip list)")
	}
}

func TestBuildTree_SkipsInternalWorktrees(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, ".worktrees/feature/main.go", "package main\n")
	writeFile(t, dir, ".worktrees/feature/util.go", "package util\n")

	skip := MakeSkipWithExtra(dir, []string{".go"}, []string{".worktrees/feature"})
	tree, err := BuildTree(dir, skip)
	if err != nil {
		t.Fatal(err)
	}

	if len(tree.Files) != 1 {
		t.Fatalf("expected 1 file (main.go only), got %d: %v", len(tree.Files), tree.Files)
	}
	if _, ok := tree.Files["main.go"]; !ok {
		t.Error("expected main.go in tree")
	}
}

func TestAncestorDirs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", []string{""}},
		{".", []string{""}},
		{"a", []string{"", "a"}},
		{"a/b", []string{"", "a", "a/b"}},
		{"a/b/c", []string{"", "a", "a/b", "a/b/c"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ancestorDirs(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ancestorDirs(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				// ancestorDirs returns OS-native keys (filepath.Join), used
				// internally with filepath.Join/Rel; on Windows "a/b" becomes
				// "a\\b". Compare against the host-localized expectation.
				want := filepath.FromSlash(tt.want[i])
				if got[i] != want {
					t.Errorf("ancestorDirs(%q)[%d] = %q, want %q", tt.input, i, got[i], want)
				}
			}
		})
	}
}

func TestBuildTree_WithGitignore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "generated/\n*.tmp\n")
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "util.go", "package main\n")

	if err := os.MkdirAll(filepath.Join(dir, "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "generated/code.go", "package generated\n")
	writeFile(t, dir, "data.tmp", "temp data")

	skip := MakeSkip(dir, []string{".go", ".tmp"})
	tree, err := BuildTree(dir, skip)
	if err != nil {
		t.Fatal(err)
	}

	// Should only have main.go and util.go
	if len(tree.Files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(tree.Files), tree.Files)
	}
	if _, ok := tree.Files["main.go"]; !ok {
		t.Error("expected main.go in tree")
	}
	if _, ok := tree.Files["util.go"]; !ok {
		t.Error("expected util.go in tree")
	}
	if _, ok := tree.Files["generated/code.go"]; ok {
		t.Error("expected generated/code.go to be excluded")
	}
	if _, ok := tree.Files["data.tmp"]; ok {
		t.Error("expected data.tmp to be excluded")
	}
}

func TestBuildTree_WithNestedGitignore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "*.log\n")
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "sub/sub.go", "package sub\n")
	writeFile(t, dir, "sub/.gitignore", "internal_*.go\n")
	writeFile(t, dir, "sub/internal_helper.go", "package sub\n")
	writeFile(t, dir, "sub/helper.go", "package sub\n")
	writeFile(t, dir, "app.log", "log data")

	skip := MakeSkip(dir, []string{".go", ".log"})
	tree, err := BuildTree(dir, skip)
	if err != nil {
		t.Fatal(err)
	}

	// main.go, sub/sub.go, sub/helper.go should be present
	// app.log (root .gitignore), sub/internal_helper.go (nested .gitignore) excluded
	expected := map[string]bool{
		"main.go":       true,
		"sub/sub.go":    true,
		"sub/helper.go": true,
	}
	if len(tree.Files) != len(expected) {
		t.Fatalf("expected %d files, got %d: %v", len(expected), len(tree.Files), tree.Files)
	}
	for f := range expected {
		if _, ok := tree.Files[f]; !ok {
			t.Errorf("expected %s in tree", f)
		}
	}
	if _, ok := tree.Files["app.log"]; ok {
		t.Error("expected app.log to be excluded by root .gitignore")
	}
	if _, ok := tree.Files["sub/internal_helper.go"]; ok {
		t.Error("expected sub/internal_helper.go to be excluded by nested .gitignore")
	}
}

func TestIgnoreTree_GlobalGitignore(t *testing.T) {
	// Create global gitignore
	globalIgnoreDir := t.TempDir()
	globalIgnorePath := filepath.Join(globalIgnoreDir, "gitignore_global")
	if err := os.WriteFile(globalIgnorePath, []byte("*.scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create git config pointing to it. Use forward slashes for the path:
	// git treats backslashes in config values as escape sequences, so a raw
	// Windows path (C:\Users\...) would be mangled. git accepts forward-slash
	// paths on every platform.
	configPath := filepath.Join(globalIgnoreDir, "gitconfig")
	configContent := fmt.Sprintf("[core]\n\texcludesFile = %s\n", filepath.ToSlash(globalIgnorePath))
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override git config location
	t.Setenv("GIT_CONFIG_GLOBAL", configPath)

	dir := t.TempDir()
	tree := NewIgnoreTree(dir, []string{".go", ".scratch"})
	if !tree.shouldSkip("notes.scratch", false) {
		t.Error("*.scratch should be skipped by global gitignore")
	}
	if tree.shouldSkip("main.go", false) {
		t.Error("main.go should not be skipped")
	}
}
