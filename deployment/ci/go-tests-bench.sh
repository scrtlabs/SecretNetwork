set -euv

# Run bench mark tests
# go test -count 1 -v ./x/compute/internal/... -run "TestRunBenchmarks"
LOG_LEVEL=ERROR go test -tags "sgx hw test" -count 1 -v ./x/compute/internal/... -run "TestRunExecuteBenchmarks"
LOG_LEVEL=ERROR go test -tags "sgx hw test" -count 1 -v ./x/compute/internal/... -run "TestRunQueryBenchmarks"