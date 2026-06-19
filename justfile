# Lumen runaway-indexing fix — baseline vs fixed demonstration.
#
# Reproduces the 2026-06-19 incident defect and proves the fix:
#   1. IsRootUnindexable did not refuse the system temp dir (C:\WINDOWS\TEMP),
#      which is a *subdir* of the case-sensitively-matched C:\Windows entry.
#   2. DiscoverNestedGitRepos walked any non-git root without bound.
#
# A reviewer can run `just demo` and visibly compare baseline (released `main`)
# vs fixed (this branch) without guessing what changed.
#
# Requires: go, git. A stray go.work can shadow the module, so the workspace is
# disabled for every go command.

set windows-shell := ["bash", "-c"]
export GOWORK := "off"

# Default: side-by-side baseline vs fixed.
demo: baseline fixed

# BASELINE (released guard): stash ONLY the guard implementation so the same
# regression test runs against the old code. On baseline the system-temp guard
# test FAILS — temp is not refused. (The implementation is restored afterward.)
baseline:
    @echo "========= BASELINE (release main): system-temp guard regression — expect FAIL ========="
    git stash push --quiet -- internal/merkle/ignore.go
    -go test ./internal/merkle/ -run 'TestIsRootUnindexable/system_temp_directory_is_refused' -count=1
    git stash pop --quiet

# FIXED (this branch): the same regression PASSES and the nested-repo walk is
# bounded.
fixed:
    @echo "========= FIXED (this branch): regressions — expect PASS ========="
    go test ./internal/merkle/ -run TestIsRootUnindexable -count=1
    go test ./internal/git/ -run TestDiscoverNestedGitRepos_BoundsRunawayWalk -count=1

# Full unit tests for the two changed packages.
test:
    go test ./internal/merkle/ ./internal/git/ -count=1

# Format + vet the changed packages.
check:
    gofmt -l internal/merkle internal/git
    go vet ./internal/merkle/ ./internal/git/
