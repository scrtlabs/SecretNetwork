package main

import (
	"encoding/json"
	"io"

	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/x/genaccounts"
	genaccscli "github.com/cosmos/cosmos-sdk/x/genaccounts/client/cli"
	"github.com/cosmos/cosmos-sdk/x/staking"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/cli"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	app "github.com/enigmampc/Enigmachain"
	eng "github.com/enigmampc/Enigmachain/types"
	abci "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

func main() {
	cobra.EnableCommandSorting = false

	cdc := app.MakeCodec()

	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(eng.Bech32PrefixAccAddr, eng.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(eng.Bech32PrefixValAddr, eng.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(eng.Bech32PrefixConsAddr, eng.Bech32PrefixConsPub)
	config.Seal()

	ctx := server.NewDefaultContext()

	rootCmd := &cobra.Command{
		Use:               "engd",
		Short:             "Enigmachain App Daemon (server)",
		PersistentPreRunE: server.PersistentPreRunEFn(ctx),
	}
	// CLI commands to initialize the chain
	rootCmd.AddCommand(
		genutilcli.InitCmd(ctx, cdc, app.ModuleBasics, app.DefaultNodeHome),
		genutilcli.CollectGenTxsCmd(ctx, cdc, genaccounts.AppModuleBasic{}, app.DefaultNodeHome),
		genutilcli.GenTxCmd(
			ctx, cdc, app.ModuleBasics, staking.AppModuleBasic{},
			genaccounts.AppModuleBasic{}, app.DefaultNodeHome, app.DefaultCLIHome,
		),
		genutilcli.ValidateGenesisCmd(ctx, cdc, app.ModuleBasics),
		// AddGenesisAccountCmd allows users to add accounts to the genesis file
		genaccscli.AddGenesisAccountCmd(ctx, cdc, app.DefaultNodeHome, app.DefaultCLIHome),
	)

	server.AddCommands(ctx, cdc, rootCmd, newApp, exportAppStateAndTMValidators)

	// prepare and add flags
	executor := cli.PrepareBaseCmd(rootCmd, "EN", app.DefaultNodeHome)
	err := executor.Execute()
	if err != nil {
		panic(err)
	}
}

func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer) abci.Application {
	return app.NewEnigmaChainApp(logger, db, baseapp.SetMinGasPrices(viper.GetString(server.FlagMinGasPrices)))
}

func exportAppStateAndTMValidators(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailWhiteList []string,
) (json.RawMessage, []tmtypes.GenesisValidator, error) {

	if height != -1 {
		engApp := app.NewEnigmaChainApp(logger, db)
		err := engApp.LoadHeight(height)
		if err != nil {
			return nil, nil, err
		}
		return engApp.ExportAppStateAndValidators(forZeroHeight, jailWhiteList)
	}

	engApp := app.NewEnigmaChainApp(logger, db)

	return engApp.ExportAppStateAndValidators(forZeroHeight, jailWhiteList)
}
