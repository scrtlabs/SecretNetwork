package app_test

import (
	"encoding/json"
	"fmt"
	"github.com/enigmampc/SecretNetwork/go-cosmwasm/api"
	"github.com/enigmampc/SecretNetwork/x/compute"
	"os"
	"testing"

	secretapp "github.com/enigmampc/SecretNetwork/app"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/rand"
	dbm "github.com/tendermint/tm-db"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/store"
	simulation2 "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
)

func init() {
	simapp.GetSimulatorFlags()
}

// Profile with:
// /usr/local/go/bin/go test -benchmem -run=^$ -bench ^BenchmarkFullAppSimulation$ -Commit=true -cpuprofile cpu.out
func BenchmarkFullAppSimulation(b *testing.B) {
	config, db, dir, logger, _, err := simapp.SetupSimulation("goleveldb-app-sim", "Simulation")
	if err != nil {
		b.Fatalf("simulation setup failed: %s", err.Error())
	}

	_, _ = api.InitBootstrap([]byte("17FDDCC9477144A2CD84E27CDCE98BE4"), []byte("b023ac3c1a514f2f97ce2314a1837804"))

	defer func() {
		err := db.Close()
		if err != nil {
			b.Fatal(err)
		}

		err = os.RemoveAll(dir)
		if err != nil {
			b.Fatal(err)
		}
	}()

	encodingConfig := secretapp.MakeEncodingConfig()
	appCodec, _ := encodingConfig.Marshaler, encodingConfig.Amino
	app := secretapp.NewSecretNetworkApp(
		logger, db, nil, false, map[int64]bool{},
		"./secretd", simapp.FlagPeriodValue, true,
		simapp.EmptyAppOptions{}, compute.DefaultWasmConfig(), interBlockCacheOpt())

	// Run randomized simulation:w
	_, simParams, simErr := simulation.SimulateFromSeed(
		b,
		os.Stdout,
		app.BaseApp,
		simapp.AppStateFn(appCodec, app.SimulationManager()),
		simulation2.RandomAccounts, // Replace with own random account function if using keys other than secp256k1
		simapp.SimulationOperations(app, appCodec, config),
		app.ModuleAccountAddrs(),
		config,
		appCodec,
	)

	// export state and simParams before the simulation error is checked
	if err = simapp.CheckExportSimulation(app, config, simParams); err != nil {
		b.Fatal(err)
	}

	if simErr != nil {
		b.Fatal(simErr)
	}

	if config.Commit {
		simapp.PrintStats(db)
	}
}

// interBlockCacheOpt returns a BaseApp option function that sets the persistent
// inter-block write-through cache.
func interBlockCacheOpt() func(*baseapp.BaseApp) {
	return baseapp.SetInterBlockCache(store.NewCommitKVStoreCacheManager())
}

//// TODO: Make another test for the fuzzer itself, which just has noOp txs
//// and doesn't depend on the application.
func TestAppStateDeterminism(t *testing.T) {
	if !simapp.FlagEnabledValue {
		t.Skip("skipping application simulation")
	}

	config := simapp.NewConfigFromFlags()
	config.InitialBlockHeight = 1
	config.ExportParamsPath = ""
	config.OnOperation = false
	config.AllInvariants = false
	config.ChainID = "secret-app"

	numSeeds := 3
	numTimesToRunPerSeed := 5
	appHashList := make([]json.RawMessage, numTimesToRunPerSeed)

	for i := 0; i < numSeeds; i++ {
		config.Seed = rand.Int63()

		for j := 0; j < numTimesToRunPerSeed; j++ {
			var logger log.Logger
			if simapp.FlagVerboseValue {
				logger = log.TestingLogger()
			} else {
				logger = log.NewNopLogger()
			}

			db := dbm.NewMemDB()
			app := secretapp.NewSecretNetworkApp(
				logger, db, nil, true, map[int64]bool{}, "./secretd",
				simapp.FlagPeriodValue, true,
				simapp.EmptyAppOptions{}, compute.DefaultWasmConfig(), interBlockCacheOpt())

			fmt.Printf(
				"running non-determinism simulation; seed %d: %d/%d, attempt: %d/%d\n",
				config.Seed, i+1, numSeeds, j+1, numTimesToRunPerSeed,
			)
			encodingConfig := secretapp.MakeEncodingConfig()
			appCodec, _ := encodingConfig.Marshaler, encodingConfig.Amino
			_, _, err := simulation.SimulateFromSeed(
				t,
				os.Stdout,
				app.BaseApp,
				simapp.AppStateFn(appCodec, app.SimulationManager()),
				simulation2.RandomAccounts, // Replace with own random account function if using keys other than secp256k1
				simapp.SimulationOperations(app, appCodec, config),
				app.ModuleAccountAddrs(),
				config,
				appCodec,
			)
			require.NoError(t, err)

			if config.Commit {
				simapp.PrintStats(db)
			}

			appHash := app.LastCommitID().Hash
			appHashList[j] = appHash

			if j != 0 {
				require.Equal(
					t, string(appHashList[0]), string(appHashList[j]),
					"non-determinism in seed %d: %d/%d, attempt: %d/%d\n", config.Seed, i+1, numSeeds, j+1, numTimesToRunPerSeed,
				)
			}
		}
	}
}
