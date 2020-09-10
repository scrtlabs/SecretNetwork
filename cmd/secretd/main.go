package main

import (
	"encoding/json"
	"fmt"
	"github.com/enigmampc/cosmos-sdk/codec"
	"github.com/enigmampc/cosmos-sdk/types/module"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/enigmampc/cosmos-sdk/server"
	serverconfig "github.com/enigmampc/cosmos-sdk/server/config"
	"github.com/enigmampc/cosmos-sdk/store"
	"github.com/enigmampc/cosmos-sdk/x/auth"
	"github.com/enigmampc/cosmos-sdk/x/staking"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/cli"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/enigmampc/cosmos-sdk/baseapp"
	"github.com/enigmampc/cosmos-sdk/client/flags"

	//"github.com/CosmWasm/wasmd/app"

	app "github.com/enigmampc/SecretNetwork"
	scrt "github.com/enigmampc/SecretNetwork/types"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	genutilcli "github.com/enigmampc/cosmos-sdk/x/genutil/client/cli"
	abci "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

const flagInvCheckPeriod = "inv-check-period"
const flagIsBootstrap = "bootstrap"

var bootstrap bool
var invCheckPeriod uint

func main() {
	cdc := app.MakeCodec()

	config := sdk.GetConfig()
	config.SetCoinType(529)
	config.SetFullFundraiserPath("44'/529'/0'/0/0")
	config.SetBech32PrefixForAccount(scrt.Bech32PrefixAccAddr, scrt.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(scrt.Bech32PrefixValAddr, scrt.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(scrt.Bech32PrefixConsAddr, scrt.Bech32PrefixConsPub)
	config.Seal()

	ctx := server.NewDefaultContext()
	cobra.EnableCommandSorting = false
	rootCmd := &cobra.Command{
		Use:               "secretd",
		Short:             "The Secret Network App Daemon (server)",
		PersistentPreRunE: server.PersistentPreRunEFn(ctx),
	}

	// CLI commands to initialize the chain
	rootCmd.AddCommand(InitAttestation(ctx, cdc))
	rootCmd.AddCommand(ParseCert(ctx, cdc))
	rootCmd.AddCommand(ConfigureSecret(ctx, cdc))
	rootCmd.AddCommand(HealthCheck(ctx, cdc))
	rootCmd.AddCommand(ResetEnclave(ctx, cdc))
	rootCmd.AddCommand(InitBootstrapCmd(ctx, cdc, app.ModuleBasics))
	rootCmd.AddCommand(updateTmParamsAndInit(ctx, cdc, app.ModuleBasics, app.DefaultNodeHome))
	rootCmd.AddCommand(genutilcli.CollectGenTxsCmd(ctx, cdc, auth.GenesisAccountIterator{}, app.DefaultNodeHome))
	rootCmd.AddCommand(genutilcli.MigrateGenesisCmd(ctx, cdc))
	rootCmd.AddCommand(
		genutilcli.GenTxCmd(
			ctx, cdc, app.ModuleBasics, staking.AppModuleBasic{},
			auth.GenesisAccountIterator{}, app.DefaultNodeHome, app.DefaultCLIHome,
		),
	)
	rootCmd.AddCommand(genutilcli.ValidateGenesisCmd(ctx, cdc, app.ModuleBasics))
	rootCmd.AddCommand(AddGenesisAccountCmd(ctx, cdc, app.DefaultNodeHome, app.DefaultCLIHome))
	rootCmd.AddCommand(flags.NewCompletionCmd(rootCmd, true))

	server.AddCommands(ctx, cdc, rootCmd, newApp, exportAppStateAndTMValidators)

	// prepare and add flags
	executor := cli.PrepareBaseCmd(rootCmd, "EN", app.DefaultNodeHome)
	rootCmd.PersistentFlags().UintVar(&invCheckPeriod, flagInvCheckPeriod,
		0, "Assert registered invariants every N blocks")
	rootCmd.PersistentFlags().BoolVar(&bootstrap, flagIsBootstrap,
		false, "Start the node as the bootstrap node for the network (only used when starting a new network)")
	err := executor.Execute()
	if err != nil {
		panic(err)
	}

	// Set default gas limit for WASM queries
	if viper.IsSet("query-gas-limit") {
		// already set, not going to overwrite it
		return
	}

	appTomlPath := path.Join(ctx.Config.RootDir, "config", "app.toml")
	if !fileExists(appTomlPath) {
		// config file does not exist, this means `secretd init` still wasn't called
		return
	}

	queryGasLimitTemplate := `
# query-gas-limit sets the gas limit under which your node will run smart contracts queries.
# Queries that consume more than this value will be terminated prematurely with an error.
# This is a good way to protect your node from DoS by heavy queries.
query-gas-limit = 3000000
`

	appTomlFile, err := os.OpenFile(appTomlPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(fmt.Sprintf("failed opening file '%s' for appending query-gas-limit.\n", err))
	}
	defer appTomlFile.Close()

	_, err = appTomlFile.WriteString(queryGasLimitTemplate)
	if err != nil {
		panic(fmt.Sprintf("failed writing default query-gas-limit to file '%s'", err))
	}
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer) abci.Application {
	var cache sdk.MultiStorePersistentCache

	if viper.GetBool(server.FlagInterBlockCache) {
		cache = store.NewCommitKVStoreCacheManager()
	}

	bootstrap := viper.GetBool("bootstrap")
	queryGasLimit := viper.GetUint64("query-gas-limit")

	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range viper.GetIntSlice(server.FlagUnsafeSkipUpgrades) {
		skipUpgradeHeights[int64(h)] = true
	}

	return app.NewSecretNetworkApp(
		logger, db, traceStore, true, bootstrap, invCheckPeriod, skipUpgradeHeights,
		queryGasLimit,
		baseapp.SetPruning(store.NewPruningOptionsFromString(viper.GetString("pruning"))),
		baseapp.SetMinGasPrices(viper.GetString(server.FlagMinGasPrices)),
		baseapp.SetHaltHeight(viper.GetUint64(server.FlagHaltHeight)),
		baseapp.SetHaltTime(viper.GetUint64(server.FlagHaltTime)),
		baseapp.SetInterBlockCache(cache))
}

func exportAppStateAndTMValidators(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailWhiteList []string,
) (json.RawMessage, []tmtypes.GenesisValidator, error) {

	bootstrap := viper.GetBool("bootstrap")
	queryGasLimit := viper.GetUint64("query-gas-limit")

	if height != -1 {
		secretApp := app.NewSecretNetworkApp(logger, db, traceStore, false, bootstrap, uint(1), map[int64]bool{}, queryGasLimit)
		err := secretApp.LoadHeight(height)
		if err != nil {
			return nil, nil, err
		}
		return secretApp.ExportAppStateAndValidators(forZeroHeight, jailWhiteList)
	}

	secretApp := app.NewSecretNetworkApp(logger, db, traceStore, true, bootstrap, uint(1), map[int64]bool{}, queryGasLimit)
	return secretApp.ExportAppStateAndValidators(forZeroHeight, jailWhiteList)
}

// writeParamsAndConfigCmd patches the write-params cmd to additionally update the app pruning config.
func updateTmParamsAndInit(ctx *server.Context, cdc *codec.Codec, mbm module.BasicManager, defaultNodeHome string) *cobra.Command {
	cmd := genutilcli.InitCmd(ctx, cdc, mbm, defaultNodeHome)
	originalFunc := cmd.RunE

	wrappedFunc := func(cmd *cobra.Command, args []string) error {

		// time is in NS
		ctx.Config.Consensus.TimeoutPrecommit = 2_000_000_000

		appConfigFilePath := filepath.Join(defaultNodeHome, "config/app.toml")
		appConf, _ := serverconfig.ParseConfig()
		appConf.MinGasPrices = "0.25uscrt"

		serverconfig.WriteConfigFile(appConfigFilePath, appConf)

		if err := originalFunc(cmd, args); err != nil {
			return err
		}

		return nil
	}

	cmd.RunE = wrappedFunc
	return cmd
}
