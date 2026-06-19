package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Candidates returns the executable paths to probe for base in dir, most
// specific first: the canonical name, then the GOOS-GOARCH release-artifact
// name (e.g. lumen-windows-amd64.exe / lumen-linux-arm64). The executable
// extension comes from the build-tagged ExeSuffix, so callers never spell
// out ".exe" and never branch on runtime.GOOS.
func Candidates(dir, base string) []string {
	return []string{
		filepath.Join(dir, base+ExeSuffix),
		filepath.Join(dir, fmt.Sprintf("%s-%s-%s%s", base, runtime.GOOS, runtime.GOARCH, ExeSuffix)),
	}
}

// Resolve returns the first existing candidate for base in dir, or the
// canonical build target (base+ExeSuffix) when none is present yet — the path
// `go build -o` should write to. filepath keeps separators host-correct.
func Resolve(dir, base string) string {
	for _, cand := range Candidates(dir, base) {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}
	return filepath.Join(dir, base+ExeSuffix)
}
