set -euv

# Run go system tests for compute module
LOG_LEVEL=ERROR go test -p 1 -v ./x/compute/internal/...
