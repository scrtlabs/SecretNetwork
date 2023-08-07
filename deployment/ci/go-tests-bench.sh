set -euv

# Run bench mark tests
# go test -count 1 -v ./x/compute/internal/... -run "TestRunBenchmarks"
LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/lib/x86_64-linux-gnu/ LOG_LEVEL=ERROR go test -tags "sgx hw test" -count 1 -v ./x/compute/internal/... -run "TestRunExecuteBenchmarks"
LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/lib/x86_64-linux-gnu/ LOG_LEVEL=ERROR go test -tags "sgx hw test" -count 1 -v ./x/compute/internal/... -run "TestRunQueryBenchmarks"