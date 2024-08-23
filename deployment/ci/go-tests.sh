set -euv

export LD_LIBRARY_PATH=/usr/lib:/usr/local/lib:/opt/sgxsdk/sdk_libs:/usr/lib/x86_64-linux-gnu/
# Run go system tests for compute module
mkdir -p ./x/compute/internal/keeper/.sgx_secrets
LOG_LEVEL=ERROR GOMAXPROCS=8 SCRT_SGX_STORAGE='./' go test -tags "sgx hw test" -failfast -timeout 90m -v ./x/compute/internal/...
