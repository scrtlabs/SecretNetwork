package main

import (
	"encoding/json"
	"io"

	"github.com/enigmampc/cosmos-sdk/server"
	"github.com/enigmampc/cosmos-sdk/store"
	"github.com/enigmampc/cosmos-sdk/x/auth"
	"github.com/enigmampc/cosmos-sdk/x/staking"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/cli"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/enigmampc/cosmos-sdk/baseapp"
	"github.com/enigmampc/cosmos-sdk/client/flags"

	sdk "github.com/enigmampc/cosmos-sdk/types"
	genutilcli "github.com/enigmampc/cosmos-sdk/x/genutil/client/cli"
	app "github.com/enigmampc/SecretNetwork"
	scrt "github.com/enigmampc/SecretNetwork/types"
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
	rootCmd.AddCommand(genutilcli.InitCmd(ctx, cdc, app.ModuleBasics, app.DefaultNodeHome))
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
}

func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer) abci.Application {
	var cache sdk.MultiStorePersistentCache

	if viper.GetBool(server.FlagInterBlockCache) {
		cache = store.NewCommitKVStoreCacheManager()
	}

	bootstrap := viper.GetBool("bootstrap")

	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range viper.GetIntSlice(server.FlagUnsafeSkipUpgrades) {
		skipUpgradeHeights[int64(h)] = true
	}

	return app.NewSecretNetworkApp(
		logger, db, traceStore, true, bootstrap, invCheckPeriod, skipUpgradeHeights,
		baseapp.SetPruning(store.NewPruningOptionsFromString(viper.GetString("pruning"))),
		baseapp.SetMinGasPrices(viper.GetString(server.FlagMinGasPrices)),
		baseapp.SetHaltHeight(viper.GetUint64(server.FlagHaltHeight)),
		baseapp.SetHaltTime(viper.GetUint64(server.FlagHaltTime)),
		baseapp.SetInterBlockCache(cache),
	)
}

func exportAppStateAndTMValidators(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailWhiteList []string,
) (json.RawMessage, []tmtypes.GenesisValidator, error) {

	bootstrap := viper.GetBool("bootstrap")

	if height != -1 {
		secretApp := app.NewSecretNetworkApp(logger, db, traceStore, false, bootstrap, uint(1), map[int64]bool{})
		err := secretApp.LoadHeight(height)
		if err != nil {
			return nil, nil, err
		}
		return secretApp.ExportAppStateAndValidators(forZeroHeight, jailWhiteList)
	}

	secretApp := app.NewSecretNetworkApp(logger, db, traceStore, true, bootstrap, uint(1), map[int64]bool{})
	return secretApp.ExportAppStateAndValidators(forZeroHeight, jailWhiteList)
}
