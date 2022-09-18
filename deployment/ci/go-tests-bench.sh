set -euv

# Run bench mark tests
go test -count 1 -v ./x/compute/internal/... -run "TestRunBenchmarks"
