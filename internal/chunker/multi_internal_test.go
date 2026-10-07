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

package chunker

import "testing"

// TestMultiChunker_UppercaseExtension pins the case-insensitive-filesystem
// contract: a source file the OS reports with an uppercase or mixed-case
// extension — routine on Windows and macOS, where Foo.GO and foo.go are the
// same file — must still dispatch to its language chunker instead of silently
// returning no chunks (which would leave the file unsearchable with no
// diagnostic).
func TestMultiChunker_UppercaseExtension(t *testing.T) {
	mc := NewMultiChunker(DefaultLanguages(512))
	const src = "package main\n\nfunc Hello() string { return \"hi\" }\n"

	for _, name := range []string{"Foo.GO", "bar.Go", "BAZ.go", "main.go"} {
		chunks, err := mc.Chunk(name, []byte(src))
		if err != nil {
			t.Fatalf("Chunk(%q): unexpected error: %v", name, err)
		}
		if len(chunks) == 0 {
			t.Errorf("Chunk(%q) returned no chunks; the extension was not recognized after case folding", name)
		}
	}
}
