package main

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/spf13/viper"

	tmlog "github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"

	"github.com/enigmampc/cosmos-sdk/baseapp"
	"github.com/enigmampc/cosmos-sdk/server"
	"github.com/enigmampc/cosmos-sdk/store"
	sdk "github.com/enigmampc/cosmos-sdk/types"

	app "github.com/enigmampc/SecretNetwork"
	rumor "github.com/enigmampc/SecretNetwork/rumor-go/app"
	"github.com/enigmampc/SecretNetwork/rumor-go/db/leveldb"
)

func main() {
	conf := getConfig()

	logger := tmlog.NewTMLogger(os.Stderr)
	db := leveldb.NewLevelDB(conf.dbDir)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Println(closeErr)
		}
	}()
	genesis, err := tmtypes.GenesisDocFromFile(conf.genesisPath)
	if err != nil {
		panic(err)
	}
	// TODO add query gas limit
	secretd := makeApp(logger, db.GetCosmosAdapter(), nil, conf)

	rum := rumor.NewMantle(secretd, db, genesis)
	rum.Sync(rumor.SyncConfiguration {
		TendermintEndpoint: conf.tendermintEndpoint,
		SyncUntil:          conf.syncUntil,
		Reconnect:          true,
	})
}

func makeApp(logger tmlog.Logger, db dbm.DB, traceStore io.Writer, conf *config) *app.SecretNetworkApp{
	var skipUpgradeHeights map[int64]bool = nil

	var cache sdk.MultiStorePersistentCache

	if viper.GetBool(server.FlagInterBlockCache) {
		cache = store.NewCommitKVStoreCacheManager()
	}

	return app.NewSecretNetworkApp(
		logger, db, traceStore, true, false, 0, skipUpgradeHeights,
		conf.queryGasLimit,
		baseapp.SetInterBlockCache(cache),
		fauxMerkleModeOpt,
		setPruningOptions(),
	)
}

// Pass this in as an option to use a dbStoreAdapter instead of an IAVLStore for simulation speed.
func fauxMerkleModeOpt(bapp *baseapp.BaseApp) {
	bapp.SetFauxMerkleMode()
}

func setPruningOptions() func(*baseapp.BaseApp) {
	// prune nothing
	pruningOptions := sdk.PruningOptions{
		KeepEvery:  0,
		SnapshotEvery: 0,
	}
	return baseapp.SetPruning(pruningOptions)
}

type config struct {
	queryGasLimit uint64
	interBlockCache bool
	tendermintEndpoint string
	syncUntil uint64
	dbDir string
	genesisPath string
}

func getConfig() *config {
	flagQueryGasLimit := "query-gas-limit"
	flagInterBlockCache := "inter-block-cache"
	flagTendermintEndpoint := "tendermint-endpoint"
	flagSyncUntil := "sync-until"
	flagDbDir := "db-dir"
	flagGenesisPath := "genesis-path"

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("couldn't find home directory\n"))
	}
	appDir := ".rumor-go"

	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("$HOME/" + appDir)
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error
		} else {
			panic(fmt.Errorf("Fatal error config file: %s \n", err))
		}
	}

	viper.SetDefault(flagQueryGasLimit, 3000000)
	viper.SetDefault(flagInterBlockCache, true)
	viper.SetDefault(flagTendermintEndpoint, "localhost:26667")
	viper.SetDefault(flagSyncUntil, 0)
	viper.SetDefault(flagDbDir, path.Join(userHomeDir, appDir, "data"))
	viper.SetDefault(flagGenesisPath, path.Join(userHomeDir, appDir, "genesis.json"))

	return &config {
		queryGasLimit: viper.GetUint64(flagQueryGasLimit),
		interBlockCache: viper.GetBool(flagInterBlockCache),
		tendermintEndpoint: viper.GetString(flagTendermintEndpoint),
		syncUntil: viper.GetUint64(flagSyncUntil),
		dbDir: viper.GetString(flagDbDir),
	}
}
