set -euv

# Run go system tests for compute module
go test -p 1 -v ./x/compute/internal/...
