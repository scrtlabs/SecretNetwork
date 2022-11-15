set -euv

# Run bench mark tests
# go test -count 1 -v ./x/compute/internal/... -run "TestRunBenchmarks"
LOG_LEVEL=ERROR go test -tags "test" -count 1 -v ./x/compute/internal/... -run "TestRunBenchmarks"