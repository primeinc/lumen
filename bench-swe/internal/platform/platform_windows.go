//go:build windows

package platform

// ExeSuffix is the host executable extension, resolved at compile time via build
// tags (mirroring Go's own cfg.ExeSuffix in cmd/go/internal/cfg) so business
// logic never branches on runtime.GOOS.
const ExeSuffix = ".exe"

// DefaultCGO is the CGO_ENABLED value used when building lumen from source;
// sqlite-vec requires cgo.
const DefaultCGO = "1"
