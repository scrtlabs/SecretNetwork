set -euv

# Run go system tests for compute module
mkdir -p ./x/compute/internal/keeper/.sgx_secrets
GOMAXPROCS=8 SCRT_SGX_STORAGE='./' go test -failfast -timeout 90m -v ./x/compute/internal/...
