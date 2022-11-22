package keeper

import (
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gonum/stat"
	"github.com/stretchr/testify/require"
)

type Bench string

// Available benches
const (
	Noop                          Bench = "noop"
	BenchCPU                            = "bench_c_p_u"
	BenchReadStorage                    = "bench_read_storage"
	BenchReadStorageMultipleKeys        = "bench_read_storage_multiple_keys"
	BenchWriteStorage                   = "bench_write_storage"
	BenchAllocate                       = "bench_allocate"
	BenchReadLargeItemFromStorage       = "bench_read_large_item_from_storage"
	SetupReadLargeItemFromStorage       = "setup_read_large_item"
	BenchWriteLargeItemToStorage  Bench = "bench_write_large_item_to_storage"
)

func buildBenchMessage(bench Bench) []byte {
	x := fmt.Sprintf("{\"%s\": {}}", bench)
	return []byte(x)
}

type BenchTime struct {
	Name        string
	Case        Bench
	Mean        float64
	iterations  uint64
	Min         time.Duration
	Max         time.Duration
	datapoints  []float64
	StdEv       float64
	AvgGas      uint64
	BaseAvgGas  uint64
	BaseAvgTime time.Duration
}

func NewBenchTimer(name string, bench Bench) BenchTime {
	return BenchTime{
		Name:        name,
		Case:        bench,
		Mean:        0,
		Min:         math.MaxInt64,
		Max:         0,
		datapoints:  []float64{},
		StdEv:       0,
		AvgGas:      0,
		BaseAvgGas:  0,
		BaseAvgTime: 0,
	}
}

func (b *BenchTime) SetBaselineValues(gas uint64, time time.Duration) {
	b.BaseAvgGas = gas
	b.BaseAvgTime = time
}

func (b *BenchTime) appendGas(gasUsed uint64) {
	currentAvgGas := b.AvgGas * b.iterations
	newAvgSum := currentAvgGas + gasUsed

	b.AvgGas = newAvgSum / (b.iterations + 1)
}

func (b *BenchTime) AppendResult(singleRunTime time.Duration, gasUsed uint64) {
	b.appendGas(gasUsed)
	b.iterations += 1

	b.datapoints = append(b.datapoints, float64(singleRunTime))

	if singleRunTime > b.Max {
		b.Max = singleRunTime
	}
	if singleRunTime < b.Min {
		b.Min = singleRunTime
	}

	// currentAvgSum := uint64(b.Mean) * b.iterations
	// newAvgSum := currentAvgSum + uint64(singleRunTime)

	//b.Mean = time.Duration(newAvgSum / b.iterations)
	//
	b.Mean, b.StdEv = stat.MeanStdDev(b.datapoints, nil)
}

func (b *BenchTime) PrintReport() {
	stdevTime := time.Duration(math.Floor(b.StdEv))
	stdevMean := time.Duration(math.Floor(b.Mean))

	s := fmt.Sprintf("*** Timer for test %s *** \n Ran benchmark: %s for %d runs \n ** Results ** \n\t Mean: %s \n\t Min: %s \n\t Max: %s \n\t StdDev: %s \n\t Gas Used (average): %d \n\t Gas Efficiency: %f [s/Mgas]",
		b.Name,
		b.Case,
		b.iterations,
		stdevMean,
		b.Min,
		b.Max,
		stdevTime,
		b.AvgGas,
		(stdevMean.Seconds())*1e6/float64(b.AvgGas),
	)

	ns := fmt.Sprintf("**** Normalized efficiency: \n\t Mean: %s \n\t Gas Used (average): %d \n\t Gas Efficiency: %f [s/Mgas]",
		stdevMean-b.BaseAvgTime,
		b.AvgGas-b.BaseAvgGas,
		(stdevMean.Seconds()-b.BaseAvgTime.Seconds())*1e6/float64(b.AvgGas-b.BaseAvgGas),
	)

	// todo: log this properly
	println(s)
	println(ns)
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
		//"warmup": {
		//	gasLimit: 1_000_000,
		//	bench:    Noop,
		//	loops:    1,
		//},
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
		"Allocate a lot of memory inside the contract": {
			gasLimit: 1_000_000,
			bench:    BenchAllocate,
			loops:    10,
		},
		"Read large item from storage": {
			gasLimit: 5_000_000,
			bench:    BenchReadLargeItemFromStorage,
			loops:    10,
		},
		"Write large item to storage": {
			gasLimit: 100_000_000,
			bench:    BenchWriteLargeItemToStorage,
			loops:    10,
		},
		"Bench read storage multiple keys": {
			gasLimit: 10_000_000,
			bench:    BenchReadStorageMultipleKeys,
			loops:    10,
		},
	}

	contractAddr, creator, creatorPriv, ctx, keeper := initBenchContract(t)
	// this is here so read multiple keys works without setup
	msg := buildBenchMessage(BenchWriteStorage)
	_, _, _, _, _, _ = execHelper(
		t,
		keeper,
		ctx,
		contractAddr,
		creator,
		creatorPriv,
		string(msg),
		false,
		true,
		10_000_000,
		0,
		false,
	)
	// this is here so read large keys works without setup
	msg = buildBenchMessage(SetupReadLargeItemFromStorage)
	_, _, _, _, _, _ = execHelper(
		t,
		keeper,
		ctx,
		contractAddr,
		creator,
		creatorPriv,
		string(msg),
		false,
		true,
		10_000_000,
		0,
		false,
	)
	// *** Measure baseline
	timer := NewBenchTimer("base contract execution", Noop)
	// make sure we set a limit before calling
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(100_000_000))
	require.Equal(t, uint64(0), ctx.GasMeter().GasConsumed())

	msg = buildBenchMessage(Noop)

	for i := uint64(1); i < 10; i++ {
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
			1_000_000,
			0,
			false,
		)
		elapsed := time.Since(start)
		require.Empty(t, qErr)
		timer.AppendResult(elapsed, gasUsed)
	}

	//
	AvgGasBase, AvgTimeBase := timer.AvgGas, timer.Mean

	timers := make(map[string]BenchTime)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			timer := NewBenchTimer(name, tc.bench)
			timer.SetBaselineValues(AvgGasBase, time.Duration(math.Floor(AvgTimeBase)))
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
