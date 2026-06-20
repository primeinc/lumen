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
	"strings"
	"testing"
)

func TestBuildTree_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	tree, err := BuildTree(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tree.RootHash == "" {
		t.Fatal("expected non-empty root hash for empty dir")
	}
	if len(tree.Files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(tree.Files))
	}
}

func TestBuildTree_SingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")

	tree, err := BuildTree(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(tree.Files))
	}
	if _, ok := tree.Files["main.go"]; !ok {
		t.Fatal("expected main.go in files map")
	}
}

func TestBuildTree_SkipsGitAndVendor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".git/config", "git config")
	if err := os.MkdirAll(filepath.Join(dir, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "vendor/lib.go", "package lib\n")
	if err := os.MkdirAll(filepath.Join(dir, "testdata"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "testdata/fixture.go", "package testdata\n")

	tree, err := BuildTree(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 {
		t.Fatalf("expected 1 file (main.go only), got %d: %v", len(tree.Files), tree.Files)
	}
}

func TestDiff_NoChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")

	old, _ := BuildTree(dir, nil)
	cur, _ := BuildTree(dir, nil)
	added, removed, modified := Diff(old, cur)
	if len(added)+len(removed)+len(modified) != 0 {
		t.Fatal("expected no changes")
	}
}

func TestDiff_DetectsModifiedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")
	old, _ := BuildTree(dir, nil)

	writeFile(t, dir, "main.go", "package main\n\nfunc Hello() {}\n")
	cur, _ := BuildTree(dir, nil)

	added, removed, modified := Diff(old, cur)
	if len(modified) != 1 || modified[0] != "main.go" {
		t.Fatalf("expected modified=[main.go], got added=%v removed=%v modified=%v", added, removed, modified)
	}
}

func TestDiff_DetectsAddedAndRemovedFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", "package a\n")
	writeFile(t, dir, "b.go", "package b\n")
	old, _ := BuildTree(dir, nil)

	_ = os.Remove(filepath.Join(dir, "b.go"))
	writeFile(t, dir, "c.go", "package c\n")
	cur, _ := BuildTree(dir, nil)

	added, removed, _ := Diff(old, cur)
	if len(added) != 1 || added[0] != "c.go" {
		t.Fatalf("expected added=[c.go], got %v", added)
	}
	if len(removed) != 1 || removed[0] != "b.go" {
		t.Fatalf("expected removed=[b.go], got %v", removed)
	}
}

func TestBuildTree_OnlyGoFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "readme.md", "# readme\n")
	writeFile(t, dir, "data.json", "{}\n")

	tree, err := BuildTree(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 {
		t.Fatalf("expected 1 .go file, got %d: %v", len(tree.Files), tree.Files)
	}
}

func TestBuildTree_ParallelMatchesSerial(t *testing.T) {
	dir := t.TempDir()
	for i := range 20 {
		content := fmt.Sprintf("package main\n\nfunc F%d() {}\n", i)
		path := filepath.Join(dir, fmt.Sprintf("f%d.go", i))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tree1, err := BuildTree(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	tree2, err := BuildTree(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if tree1.RootHash != tree2.RootHash {
		t.Fatalf("two runs produced different root hashes: %s vs %s", tree1.RootHash, tree2.RootHash)
	}
	if len(tree1.Files) != 20 {
		t.Fatalf("expected 20 files, got %d", len(tree1.Files))
	}
}

func TestCollectFilePaths_SkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.go")
	if err := os.WriteFile(realFile, []byte("package p"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkFile := filepath.Join(dir, "link.go")
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	paths, err := collectFilePaths(dir, DefaultSkip)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if filepath.Base(p) == "link.go" {
			t.Errorf("symlink should be skipped, got: %v", paths)
		}
	}
}

func TestCollectFilePaths_SkipsLargeFiles(t *testing.T) {
	dir := t.TempDir()
	largeFile := filepath.Join(dir, "large.go")
	data := make([]byte, 11*1024*1024)
	if err := os.WriteFile(largeFile, data, 0o644); err != nil {
		t.Fatal(err)
	}
	smallFile := filepath.Join(dir, "small.go")
	if err := os.WriteFile(smallFile, []byte("package p"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := collectFilePaths(dir, DefaultSkip)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if filepath.Base(p) == "large.go" {
			t.Errorf("large file should be skipped, got: %v", paths)
		}
	}
	found := false
	for _, p := range paths {
		if filepath.Base(p) == "small.go" {
			found = true
		}
	}
	if !found {
		t.Error("small.go should be included")
	}
}

func TestBuildTree_SkipsPermissionDeniedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "accessible.go", "package main\n")
	writeFile(t, dir, "secret.go", "package main\n")
	// Make secret.go unreadable for the duration of the walk: chmod(0) on Unix,
	// an exclusive no-share handle on Windows (chmod does not deny reads there).
	if !makeFileUnreadable(t, filepath.Join(dir, "secret.go")) {
		t.Skip("cannot make a file unreadable in this environment (e.g. running as root)")
	}

	tree, err := BuildTree(dir, nil)
	if err != nil {
		t.Fatalf("expected no error for permission-denied file, got: %v", err)
	}
	if _, ok := tree.Files["accessible.go"]; !ok {
		t.Error("expected accessible.go in tree")
	}
	if _, ok := tree.Files["secret.go"]; ok {
		t.Error("expected secret.go to be skipped")
	}
}

// TestBuildTree_KeysAreForwardSlash pins the cross-platform indexing contract:
// every tree.Files key is forward-slash separated regardless of host OS, so the
// root hash, the stored file_path, chunk IDs, and embedding inputs are
// byte-identical on Windows, macOS, and Linux. Without normalization Windows
// filepath.Rel emits backslash keys, which break the store's "/"-anchored
// scoped-search prefix filter and make an index non-portable across platforms.
func TestBuildTree_KeysAreForwardSlash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "top.go", "package main\n")
	writeFile(t, dir, "sub/inner.go", "package sub\n")
	writeFile(t, dir, "a/b/c/deep.go", "package c\n")

	tree, err := BuildTree(dir, MakeSkip(dir, []string{".go"}))
	if err != nil {
		t.Fatal(err)
	}

	for key := range tree.Files {
		if strings.ContainsRune(key, '\\') {
			t.Errorf("tree key %q contains a backslash; keys must be forward-slash on every platform", key)
		}
	}
	for _, want := range []string{"top.go", "sub/inner.go", "a/b/c/deep.go"} {
		if _, ok := tree.Files[want]; !ok {
			t.Errorf("expected forward-slash key %q in tree, got keys: %v", want, tree.Files)
		}
	}
}

// TestMakeSkip_ExtensionCaseInsensitive pins the inclusion-side half of the
// case-insensitive-filesystem contract: the extension filter must accept a
// supported extension regardless of case (App.GO, notes.MD) — routine on
// Windows/macOS — while still skipping genuinely unsupported extensions. Without
// case folding an uppercase-extension source file is silently dropped from the
// tree with no diagnostic.
func TestMakeSkip_ExtensionCaseInsensitive(t *testing.T) {
	skip := MakeSkip(t.TempDir(), []string{".go", ".md"})

	for _, name := range []string{"Foo.GO", "bar.Md", "BAZ.go", "main.go", "readme.md"} {
		if skip(name, false) {
			t.Errorf("skip(%q) = true; a supported extension was rejected because of its case", name)
		}
	}
	for _, name := range []string{"img.PNG", "blob.BIN"} {
		if !skip(name, false) {
			t.Errorf("skip(%q) = false; an unsupported extension should be skipped regardless of case", name)
		}
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
