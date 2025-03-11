package main

import (
	// "context"
	"io"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/types/module"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"

	eip191 "github.com/scrtlabs/SecretNetwork/eip191"
	scrt "github.com/scrtlabs/SecretNetwork/types"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	"github.com/spf13/viper"

	txsigning "cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/tx/signing/aminojson"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"

	rosettacmd "github.com/cosmos/rosetta/cmd"

	"cosmossdk.io/store/snapshots"
	snapshottypes "cosmossdk.io/store/snapshots/types"
	storetypes "cosmossdk.io/store/types"
	"github.com/scrtlabs/SecretNetwork/app"

	confixcmd "cosmossdk.io/tools/confix/cmd"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	tmcli "github.com/cometbft/cometbft/libs/cli"
	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/store"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	clientconfig "github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	secretlegacy "github.com/scrtlabs/SecretNetwork/app/migrations"
)

const (
	flagIsBootstrap = "bootstrap"
	cfgFileName     = "config.toml"
)

var bootstrap bool

// NewRootCmd creates a new root command for simd. It is called once in the
// main function.
func NewRootCmd() (*cobra.Command, app.EncodingConfig) {
	config := sdk.GetConfig()
	config.SetCoinType(scrt.CoinType)
	config.SetPurpose(scrt.CoinPurpose)
	config.SetBech32PrefixForAccount(scrt.Bech32PrefixAccAddr, scrt.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(scrt.Bech32PrefixValAddr, scrt.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(scrt.Bech32PrefixConsAddr, scrt.Bech32PrefixConsPub)
	config.SetAddressVerifier(scrt.AddressVerifier)
	config.Seal()

	tempDir := func() string {
		dir, err := os.MkdirTemp("", "secretd")
		if err != nil {
			dir = app.DefaultNodeHome
		}
		defer os.RemoveAll(dir)

		return dir
	}

	wasmConfig := compute.DefaultWasmConfig()
	wasmConfig.InitEnclave = false
	tempApp := app.NewSecretNetworkApp(log.NewNopLogger(), dbm.NewMemDB(), nil, true, true, simtestutil.NewAppOptionsWithFlagHome(tempDir()), wasmConfig)

	encodingConfig := app.EncodingConfig{
		InterfaceRegistry: tempApp.GetInterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.TxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}

	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithViper("SECRET")

	rootCmd := &cobra.Command{
		Use:   "secretd",
		Short: "The Secret Network App Daemon (server)",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx = initClientCtx.WithCmdContext(cmd.Context())
			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}
			initClientCtx, err = clientconfig.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}
			initClientCtx.WithKeyringDir(initClientCtx.HomeDir)
			if !initClientCtx.Offline {
				signingOpts, err := authtx.NewDefaultSigningOptions()
				if err != nil {
					return err
				}
				signingOpts.FileResolver = encodingConfig.Codec.InterfaceRegistry()
				aminoHandler := aminojson.NewSignModeHandler(aminojson.SignModeHandlerOptions{
					FileResolver: signingOpts.FileResolver,
					TypeResolver: signingOpts.TypeResolver,
				})
				eip191Handler := eip191.NewSignModeHandler(eip191.SignModeHandlerOptions{
					AminoJsonSignModeHandler: aminoHandler,
				})

				enabledSignModes := authtx.DefaultSignModes
				enabledSignModes = append(enabledSignModes, signing.SignMode_SIGN_MODE_TEXTUAL)
				txConfigOpts := authtx.ConfigOptions{
					EnabledSignModes:           enabledSignModes,
					TextualCoinMetadataQueryFn: txmodule.NewGRPCCoinMetadataQueryFn(initClientCtx),
					CustomSignModes:            [](txsigning.SignModeHandler){*eip191Handler},
				}
				txConfig, err := authtx.NewTxConfigWithOptions(
					initClientCtx.Codec,
					txConfigOpts,
				)
				if err != nil {
					return err
				}

				initClientCtx = initClientCtx.WithTxConfig(txConfig)
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			secretAppTemplate, secretAppConfig := initAppConfig()

			secretCMTConfig := initCometBFTConfig()

			return server.InterceptConfigsPreRunHandler(cmd, secretAppTemplate, secretAppConfig, secretCMTConfig)
		},
		SilenceUsage: true,
	}

	initRootCmd(rootCmd, encodingConfig, app.ModuleBasics())

	autoCliOpts := tempApp.AutoCliOpts()
	autoCliOpts.ClientCtx = initClientCtx

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	return rootCmd, encodingConfig
}

func initRootCmd(rootCmd *cobra.Command, encodingConfig app.EncodingConfig, basicManager module.BasicManager) {
	rootCmd.AddCommand(
		InitCmd(app.ModuleBasics(), app.DefaultNodeHome),
		secretlegacy.MigrateGenesisCmd(),
		tmcli.NewCompletionCmd(rootCmd, true),
		// testnetCmd(app.ModuleBasics, banktypes.GenesisBalancesIterator{}),
		debug.Cmd(),
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome, newApp, exportAppStateAndTMValidators, addModuleInitFlags)

	// add keybase, auxiliary RPC, query, and tx child commands
	rootCmd.AddCommand(
		server.StatusCommand(),
		genesisCommand(encodingConfig, basicManager),
		queryCommand(),
		txCommand(),
		InitAttestation(),
		InitBootstrapCmd(),
		ParseCert(),
		DumpBin(),
		MigrateSealings(),
		ConfigureSecret(),
		HealthCheck(),
		ResetEnclave(),
		AutoRegisterNode(),
		confixcmd.ConfigCommand(),
		keys.Commands(),
	)

	// add rosetta commands
	rootCmd.AddCommand(rosettacmd.RosettaCommand(encodingConfig.InterfaceRegistry, encodingConfig.Codec))

	// This is needed for `newApp` and `exportAppStateAndTMValidators`
	rootCmd.PersistentFlags().BoolVar(&bootstrap, flagIsBootstrap,
		false, "Start the node as the bootstrap node for the network (only used when starting a new network)")
}

// genesisCommand builds genesis-related `simd genesis` command. Users may provide application specific commands as a parameter
func genesisCommand(eCfg app.EncodingConfig, basicManager module.BasicManager, cmds ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "genesis",
		Short:                      "SecretNetwork genesis-related subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	gentxModule := basicManager[genutiltypes.ModuleName].(genutil.AppModuleBasic)

	cmd.AddCommand(
		genutilcli.GenTxCmd(
			basicManager,
			eCfg.TxConfig,
			banktypes.GenesisBalancesIterator{},
			app.DefaultNodeHome,
			eCfg.InterfaceRegistry.SigningContext().ValidatorAddressCodec()),
		genutilcli.CollectGenTxsCmd(
			banktypes.GenesisBalancesIterator{},
			app.DefaultNodeHome,
			gentxModule.GenTxValidator,
			eCfg.InterfaceRegistry.SigningContext().ValidatorAddressCodec()),
		genutilcli.ValidateGenesisCmd(basicManager),
		AddGenesisAccountCmd(app.DefaultNodeHome, eCfg),
	)

	for _, subCmd := range cmds {
		cmd.AddCommand(subCmd)
	}
	return cmd
}

func addModuleInitFlags(startCmd *cobra.Command) {
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
		SilenceUsage:               true,
	}

	cmd.AddCommand(
		rpc.WaitTxCmd(),
		server.QueryBlockCmd(),
		authcmd.QueryTxsByEventsCmd(),
		server.QueryBlocksCmd(),
		authcmd.QueryTxCmd(),
		server.QueryBlockResultsCmd(),
		rpc.ValidatorCommand(),
		S20GetQueryCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")
	cmd.PersistentFlags().String(tmcli.OutputFlag, "text", "Output format (text|json)")

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignDocCommand(),
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetValidateSignaturesCommand(),
		flags.LineBreak,
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		flags.LineBreak,
		S20GetTxCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")
	cmd.PersistentFlags().String(tmcli.OutputFlag, "text", "Output format (text|json)")

	return cmd
}

func newApp(logger log.Logger, db dbm.DB, traceStore io.Writer, appOpts servertypes.AppOptions) servertypes.Application {
	var cache storetypes.MultiStorePersistentCache

	if cast.ToBool(appOpts.Get(server.FlagInterBlockCache)) {
		cache = store.NewCommitKVStoreCacheManager()
	}

	pruningOpts, err := server.GetPruningOptionsFromFlags(appOpts)
	if err != nil {
		panic(err)
	}

	snapshotDir := filepath.Join(cast.ToString(appOpts.Get(flags.FlagHome)), "data", "snapshots")
	snapshotDB, err := dbm.NewDB("metadata", dbm.GoLevelDBBackend, snapshotDir)
	if err != nil {
		panic(err)
	}
	snapshotStore, err := snapshots.NewStore(snapshotDB, snapshotDir)
	if err != nil {
		panic(err)
	}

	bootstrap := cast.ToBool(appOpts.Get("bootstrap"))

	appGenesis, err := genutiltypes.AppGenesisFromFile(filepath.Join(cast.ToString(appOpts.Get(flags.FlagHome)), "config", "genesis.json"))
	if err != nil {
		panic(err)
	}

	return app.NewSecretNetworkApp(logger, db, traceStore, true,
		bootstrap,
		appOpts,
		compute.GetConfig(appOpts),
		baseapp.SetPruning(pruningOpts),
		baseapp.SetMinGasPrices(cast.ToString(appOpts.Get(server.FlagMinGasPrices))),
		baseapp.SetHaltHeight(cast.ToUint64(appOpts.Get(server.FlagHaltHeight))),
		baseapp.SetHaltTime(cast.ToUint64(appOpts.Get(server.FlagHaltTime))),
		baseapp.SetMinRetainBlocks(cast.ToUint64(appOpts.Get(server.FlagMinRetainBlocks))),
		baseapp.SetInterBlockCache(cache),
		baseapp.SetTrace(cast.ToBool(appOpts.Get(server.FlagTrace))),
		baseapp.SetIndexEvents(cast.ToStringSlice(appOpts.Get(server.FlagIndexEvents))),
		baseapp.SetSnapshot(snapshotStore, snapshottypes.NewSnapshotOptions(cast.ToUint64(appOpts.Get(server.FlagStateSyncSnapshotInterval)), cast.ToUint32(appOpts.Get(server.FlagStateSyncSnapshotKeepRecent)))),
		baseapp.SetIAVLCacheSize(cast.ToInt(appOpts.Get(server.FlagIAVLCacheSize))),
		baseapp.SetIAVLDisableFastNode(cast.ToBool(appOpts.Get(server.FlagDisableIAVLFastNode))),
		baseapp.SetChainID(appGenesis.ChainID),
	)
}

func exportAppStateAndTMValidators(
	logger log.Logger, db dbm.DB, traceStore io.Writer, height int64, forZeroHeight bool, jailWhiteList []string, appOpts servertypes.AppOptions, modulesToExport []string,
) (servertypes.ExportedApp, error) {
	bootstrap := viper.GetBool("bootstrap")

	var wasmApp *app.SecretNetworkApp
	if height != -1 {
		wasmApp = app.NewSecretNetworkApp(logger, db, traceStore, false, bootstrap, appOpts, compute.DefaultWasmConfig())

		if err := wasmApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		wasmApp = app.NewSecretNetworkApp(logger, db, traceStore, true, bootstrap, appOpts, compute.DefaultWasmConfig())
	}

	return wasmApp.ExportAppStateAndValidators(forZeroHeight, jailWhiteList, modulesToExport)
}
