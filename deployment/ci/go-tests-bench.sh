set -euv

export LD_LIBRARY_PATH=/usr/lib:/usr/local/lib:/opt/sgxsdk/sdk_libs:/usr/lib/x86_64-linux-gnu/
# Run bench mark tests
# go test -count 1 -v ./x/compute/internal/... -run "TestRunBenchmarks"
LOG_LEVEL=ERROR go test -tags "sgx hw test" -count 1 -v ./x/compute/internal/... -run "TestRunExecuteBenchmarks"
LOG_LEVEL=ERROR go test -tags "sgx hw test" -count 1 -v ./x/compute/internal/... -run "TestRunQueryBenchmarks"