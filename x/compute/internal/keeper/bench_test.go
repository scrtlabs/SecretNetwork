package keeper

import (
	"fmt"
	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gonum/stat"
	"github.com/stretchr/testify/require"
	"math"
	"os"
	"testing"
	"time"
)

type Bench string

// Available benches
const (
	Noop                          Bench = "noop"
	BenchCPU                            = "bench_c_p_u"
	BenchReadStorage                    = "bench_read_storage"
	BenchWriteStorage                   = "bench_write_storage"
	BenchAllocate                       = "bench_allocate"
	BenchReadLargeItemFromStorage       = "bench_read_large_item_from_storage"
	BenchWriteLargeItemToStorage  Bench = "bench_write_large_item_from_storage"
)

func buildBenchMessage(bench Bench) []byte {
	x := fmt.Sprintf("{\"%s\": {}}", bench)
	return []byte(x)
}

type BenchTime struct {
	Name       string
	Case       Bench
	Mean       float64
	iterations uint64
	Min        time.Duration
	Max        time.Duration
	datapoints []float64
	StdEv      float64
	AvgGas     uint64
}

func NewBenchTimer(name string, bench Bench) BenchTime {
	return BenchTime{
		Name:       name,
		Case:       bench,
		Mean:       0,
		Min:        math.MaxInt64,
		Max:        0,
		datapoints: []float64{},
		StdEv:      0,
		AvgGas:     0,
	}
}

func (b *BenchTime) appendGas(gasUsed uint64) {
	currentAvgGas := b.AvgGas * b.iterations
	newAvgSum := currentAvgGas + gasUsed

	b.AvgGas = newAvgSum / b.iterations
}

func (b *BenchTime) AppendResult(singleRunTime time.Duration, gasUsed uint64) {
	b.iterations += 1
	b.datapoints = append(b.datapoints, float64(singleRunTime))

	if singleRunTime > b.Max {
		b.Max = singleRunTime
	}
	if singleRunTime < b.Min {
		b.Min = singleRunTime
	}

	//currentAvgSum := uint64(b.Mean) * b.iterations
	//newAvgSum := currentAvgSum + uint64(singleRunTime)

	//b.Mean = time.Duration(newAvgSum / b.iterations)
	//
	b.Mean, b.StdEv = stat.MeanStdDev(b.datapoints, nil)

	b.appendGas(gasUsed)
}

func (b *BenchTime) PrintReport() {

	stdevTime := time.Duration(math.Floor(b.StdEv))
	stdevMean := time.Duration(math.Floor(b.Mean))

	s := fmt.Sprintf("*** Timer for test %s *** \n Ran benchmark: %s for %d runs \n ** Results ** \n\t Mean: %s \n\t Min: %s \n\t Max: %s \n\t StdDev: %s \n\t Gas Used (average): %d \n\t Gas Value: %f [Kgas/ms]",
		b.Name,
		b.Case,
		b.iterations,
		stdevMean,
		b.Min,
		b.Max,
		stdevTime,
		b.AvgGas,
		float64(b.AvgGas)/b.Mean*1000,
	)

	// todo: log this properly
	println(s)
}

func initBenchContract(t *testing.T) (contract sdk.AccAddress, creator sdk.AccAddress, creatorPriv crypto.PrivKey, ctx sdk.Context, keeper Keeper) {

	encodingConfig := MakeEncodingConfig()

	encoders := DefaultEncoders(nil, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, creatorPriv = CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))

	// store the code
	wasmCode, err := os.ReadFile(TestContractPaths[benchContract])
	require.NoError(t, err)
	codeID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, creator, creatorPriv, `{"init": {}}`, true, true, defaultGasForTests)
	require.Empty(t, initErr)

	return contractAddr, creator, creatorPriv, ctx, keeper
}

func TestRunBenchmarks(t *testing.T) {

	cases := map[string]struct {
		gasLimit   uint64
		bench      Bench
		loops      uint64
		callbackfn func() uint64
	}{
		"warmup": {
			gasLimit: 1_000_000,
			bench:    Noop,
			loops:    10,
		},
		"Empty execution (contract startup time)": {
			gasLimit: 1_000_000,
			bench:    Noop,
			loops:    10,
		},
		"CPU 5000 rounds of sha2": {
			gasLimit: 1_000_000,
			bench:    BenchCPU,
			loops:    10,
		},
		"Storage Write 100 different keys": {
			gasLimit: 1_000_000,
			bench:    BenchWriteStorage,
			loops:    10,
		},
		"Storage Read 100 times same key": {
			gasLimit: 1_000_000,
			bench:    BenchReadStorage,
			loops:    10,
		},
	}

	contractAddr, creator, creatorPriv, ctx, keeper := initBenchContract(t)

	timers := make(map[string]BenchTime)

	for name, tc := range cases {

		t.Run(name, func(t *testing.T) {
			timer := NewBenchTimer(name, tc.bench)
			// make sure we set a limit before calling
			ctx = ctx.WithGasMeter(sdk.NewGasMeter(tc.gasLimit))
			require.Equal(t, uint64(0), ctx.GasMeter().GasConsumed())

			msg := buildBenchMessage(tc.bench)

			for i := uint64(1); i < tc.loops+1; i++ {
				start := time.Now()
				// call bench
				_, _, qErr, _, gasUsed, _ := execHelper(
					t,
					keeper,
					ctx,
					contractAddr,
					creator,
					creatorPriv,
					string(msg),
					false,
					true,
					tc.gasLimit,
					0,
					false,
				)
				elapsed := time.Since(start)
				require.Empty(t, qErr)

				timer.AppendResult(elapsed, gasUsed)
			}
			timers[name] = timer
		})

	}

	for name := range cases {
		timer, _ := timers[name]
		timer.PrintReport()
	}
}
