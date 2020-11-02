package main

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	scrt "github.com/enigmampc/SecretNetwork/types"
	"os"
	"path"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/version"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	bankcmd "github.com/cosmos/cosmos-sdk/x/bank/client/cli"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/cli"

	"github.com/enigmampc/SecretNetwork/app"
)

const flagIsBootstrap = "bootstrap"

// ClientName is set via build process
const ClientName = "wasmcli"

// thanks @terra-project for this fix
const flagLegacyHdPath = "legacy-hd-path"

var bootstrap bool

func main() {
	cobra.EnableCommandSorting = false

	encodingConfig := app.MakeEncodingConfig()

	initClientCtx := client.Context{}.
		WithJSONMarshaler(encodingConfig.Marshaler).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastBlock).
		WithHomeDir(app.DefaultCLIHome)

	rootCmd := &cobra.Command{
		Use:   ClientName,
		Short: "Command line interface for interacting with " + version.AppName,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			err := initConfig(cmd)
			if err != nil {
				return err
			}

			if err = client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			return server.InterceptConfigsPreRunHandler(cmd)
		},
	}

	// Add --chain-id to persistent flags and mark it required
	rootCmd.PersistentFlags().Bool(flagLegacyHdPath, false, "Flag to specify the command uses old HD path - use this for ledger compatibility")
	rootCmd.PersistentFlags().String(flags.FlagChainID, "", "Chain ID of tendermint node")
	rootCmd.PersistentFlags().BoolVar(&bootstrap, flagIsBootstrap,
		false, "Start the node as the bootstrap node for the network (only used when starting a new network)")

	// Construct Root Command
	rootCmd.AddCommand(
		rpc.StatusCommand(),
		queryCmd(),
		txCmd(),
		flags.LineBreak,
		flags.LineBreak,
		keys.Commands(app.DefaultNodeHome),
		flags.LineBreak,
		//version.Cmd,
		cli.NewCompletionCmd(rootCmd, true),
	)

	// Create and set a client.Context on the command's Context. During the pre-run
	// of the root command, a default initialized client.Context is provided to
	// seed child command execution with values such as AccountRetriver, Keyring,
	// and a Tendermint RPC. This requires the use of a pointer reference when
	// getting and setting the client.Context. Ideally, we utilize
	// https://github.com/spf13/cobra/pull/1118.
	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &client.Context{})
	ctx = context.WithValue(ctx, server.ServerContextKey, server.NewDefaultContext())

	// Add flags and prefix all env exposed with EN
	executor := cli.PrepareBaseCmd(rootCmd, "EN", app.DefaultCLIHome)
	err := executor.Execute()
	if err != nil {
		fmt.Printf("Failed executing CLI command: %s, exiting...\n", err)
		os.Exit(1)
	}
}

func queryCmd() *cobra.Command {
	queryCmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	queryCmd.AddCommand(
		authcmd.GetAccountCmd(),
		flags.LineBreak,
		rpc.ValidatorCommand(),
		rpc.BlockCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(), // TODO add another one like this that decrypts the output if it's from the wallet that sent the tx
		flags.LineBreak,
		S20GetQueryCmd(),
	)

	// add modules' query commands
	app.ModuleBasics.AddQueryCommands(queryCmd)

	return queryCmd
}

func txCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	viper.SetDefault(flags.FlagGasPrices, "0.25uscrt")

	txCmd.AddCommand(
		bankcmd.NewSendTxCmd(),
		flags.LineBreak,
		authcmd.GetSignCommand(),
		authcmd.GetMultiSignCommand(),
		flags.LineBreak,
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		flags.LineBreak,
		S20GetTxCmd(),
	)

	// add modules' tx commands
	app.ModuleBasics.AddTxCommands(txCmd)

	// remove auth and bank commands as they're mounted under the root tx command
	var cmdsToRemove []*cobra.Command

	for _, cmd := range txCmd.Commands() {
		if cmd.Use == authtypes.ModuleName || cmd.Use == banktypes.ModuleName {
			cmdsToRemove = append(cmdsToRemove, cmd)
		}
	}

	txCmd.RemoveCommand(cmdsToRemove...)

	return txCmd
}

func initConfig(cmd *cobra.Command) error {
	oldHDPath, err := cmd.PersistentFlags().GetBool(flagLegacyHdPath)
	if err != nil {
		return err
	}

	// Read in the configuration file for the sdk
	config := sdk.GetConfig()

	if !oldHDPath {
		config.SetCoinType(529)
		config.SetFullFundraiserPath("44'/529'/0'/0/0")
	}

	config.SetBech32PrefixForAccount(scrt.Bech32PrefixAccAddr, scrt.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(scrt.Bech32PrefixValAddr, scrt.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(scrt.Bech32PrefixConsAddr, scrt.Bech32PrefixConsPub)
	config.Seal()

	home, err := cmd.PersistentFlags().GetString(cli.HomeFlag)
	if err != nil {
		return err
	}

	cfgFile := path.Join(home, "config", "config.toml")
	if _, err := os.Stat(cfgFile); err == nil {
		viper.SetConfigFile(cfgFile)

		if err := viper.ReadInConfig(); err != nil {
			return err
		}
	}
	if err := viper.BindPFlag(flags.FlagChainID, cmd.PersistentFlags().Lookup(flags.FlagChainID)); err != nil {
		return err
	}
	if err := viper.BindPFlag(cli.EncodingFlag, cmd.PersistentFlags().Lookup(cli.EncodingFlag)); err != nil {
		return err
	}
	return viper.BindPFlag(cli.OutputFlag, cmd.PersistentFlags().Lookup(cli.OutputFlag))
}
