//go:build !windows

package platform

// ExeSuffix is empty on Unix-like systems (no executable extension).
const ExeSuffix = ""

// DefaultCGO is the CGO_ENABLED value used when building lumen from source;
// sqlite-vec requires cgo.
const DefaultCGO = "1"
