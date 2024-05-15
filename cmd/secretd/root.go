package main

import (
	// "context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	"github.com/cosmos/cosmos-sdk/types/module"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/crisis"

	// "github.com/rs/zerolog"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	scrt "github.com/scrtlabs/SecretNetwork/types"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	rosettacmd "github.com/cosmos/rosetta/cmd"

	//"github.com/cometbft/cometbft/libs/cli"

	"cosmossdk.io/store/snapshots"
	snapshottypes "cosmossdk.io/store/snapshots/types"
	storetypes "cosmossdk.io/store/types"
	"github.com/scrtlabs/SecretNetwork/app"

	confixcmd "cosmossdk.io/tools/confix/cmd"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	// tmcfg "github.com/cometbft/cometbft/config"
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

// thanks @terra-project for this fix
const flagLegacyHdPath = "legacy-hd-path"

const (
	flagIsBootstrap = "bootstrap"
	cfgFileName     = "config.toml"
)

var bootstrap bool

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent
		// keys with underscores, e.g. --favorite-color to STING_FAVORITE_COLOR
		_ = v.BindEnv(f.Name, fmt.Sprintf("%s_%s", "SECRET_NETWORK", strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))))
		_ = v.BindPFlag(f.Name, f)

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%+v", val))
		}
	})
}

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

	encodingConfig := app.MakeEncodingConfig()

	// cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		// WithBroadcastMode(flags.BroadcastBlock).
		// WithHomeDir(app.DefaultNodeHome).
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
			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			secretAppTemplate, secretAppConfig := initAppConfig()

			// ctx := server.GetServerContextFromCmd(cmd)

			// bindFlags(cmd, ctx.Viper)

			secretCMTConfig := initCometBFTConfig()

			return server.InterceptConfigsPreRunHandler(cmd, secretAppTemplate, secretAppConfig, secretCMTConfig)
			// return initConfig(&initClientCtx, cmd)
		},
		SilenceUsage: true,
	}

	tempDir := func() string {
		dir, err := os.MkdirTemp("", "secretd")
		if err != nil {
			dir = app.DefaultNodeHome
		}
		defer os.RemoveAll(dir)

		return dir
	}

	tempApp := app.NewSecretNetworkApp(log.NewNopLogger(), dbm.NewMemDB(), nil, true, true, simtestutil.NewAppOptionsWithFlagHome(tempDir()), compute.DefaultWasmConfig())

	initRootCmd(rootCmd, encodingConfig, app.ModuleBasics())

	autoCliOpts := tempApp.AutoCliOpts()
	initClientCtx, _ = clientconfig.ReadFromClientConfig(initClientCtx)
	autoCliOpts.Keyring, _ = keyring.NewAutoCLIKeyring(initClientCtx.Keyring)
	autoCliOpts.ClientCtx = initClientCtx

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	return rootCmd, encodingConfig
}

func initRootCmd(rootCmd *cobra.Command, encodingConfig app.EncodingConfig, basicManager module.BasicManager) {
	rootCmd.AddCommand(
		InitCmd(app.ModuleBasics(), app.DefaultNodeHome),
		// genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, app.ModuleBasics()[genutiltypes.ModuleName].(genutil.AppModuleBasic).GenTxValidator, encodingConfig.TxConfig.SigningContext().ValidatorAddressCodec()),
		secretlegacy.MigrateGenesisCmd(),
		// genutilcli.GenTxCmd(app.ModuleBasics(), encodingConfig.TxConfig, banktypes.GenesisBalancesIterator{}, app.DefaultNodeHome, encodingConfig.TxConfig.SigningContext().ValidatorAddressCodec()),
		// genutilcli.ValidateGenesisCmd(app.ModuleBasics()),
		// AddGenesisAccountCmd(app.DefaultNodeHome, encodingConfig),
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
	// cmd := genutilcli.Commands(txConfig, basicManager, app.DefaultNodeHome)

	// for _, subCmd := range cmds {
	// 	cmd.AddCommand(subCmd)
	// }
	// return cmd

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
		// genutilcli.MigrateGenesisCmd(migrationMap),
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
	crisis.AddModuleInitFlags(startCmd)
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
		// authcmd.GetAccountCmd(),
		rpc.ValidatorCommand(),
		// rpc.BlockCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
		// S20GetQueryCmd(),
	)

	// app.ModuleBasics().AddQueryCommands(cmd)
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
		// vestingcli.GetTxCmd(),
		// S20GetTxCmd(),
	)

	// app.ModuleBasics().AddTxCommands(cmd)
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

	// fmt.Printf("bootstrap: %s\n", cast.ToString(bootstrap))

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

	// encCfg := app.MakeEncodingConfig()
	// encCfg.Marshaler = codec.NewProtoCodec(encCfg.InterfaceRegistry)
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

// writeParamsAndConfigCmd patches the write-params cmd to additionally update the app pruning config.
func updateTmParamsAndInit(mbm module.BasicManager, defaultNodeHome string) *cobra.Command {
	cmd := genutilcli.InitCmd(mbm, defaultNodeHome)
	originalFunc := cmd.RunE

	wrappedFunc := func(cmd *cobra.Command, args []string) error {
		ctx := server.GetServerContextFromCmd(cmd)

		// time is in NS
		ctx.Config.Consensus.TimeoutPrecommit = 2_000_000_000

		appConfigFilePath := filepath.Join(defaultNodeHome, "config/app.toml")
		appConf, _ := serverconfig.ParseConfig(viper.GetViper())
		appConf.MinGasPrices = "0.25uscrt"

		serverconfig.WriteConfigFile(appConfigFilePath, appConf)

		err := originalFunc(cmd, args)
		return err
	}

	cmd.RunE = wrappedFunc
	return cmd
}

func initConfig(ctx *client.Context, cmd *cobra.Command) error {
	cmd.PersistentFlags().Bool(flagLegacyHdPath, false, "Flag to specify the command uses old HD path - use this for ledger compatibility")

	_, err := cmd.PersistentFlags().GetBool(flagLegacyHdPath)
	if err != nil {
		return err
	}
	//if !oldHDPath {
	//	config.SetPurpose(44)
	//	config.SetCoinType(529)
	//	//config.SetFullFundraiserPath("44'/529'/0'/0/0")
	//}
	//
	//config.Seal()

	cfgFilePath := filepath.Join(app.DefaultCLIHome, "config", cfgFileName)
	if _, err := os.Stat(cfgFilePath); err == nil {
		viper.SetConfigFile(cfgFilePath)

		if err := viper.ReadInConfig(); err != nil {
			return err
		}
	}

	cfgFlags := []string{flags.FlagChainID, flags.FlagKeyringBackend}
	for _, flag := range cfgFlags {
		err = setFlagFromConfig(cmd, flag)
		if err != nil {
			return err
		}
	}

	return client.SetCmdClientContextHandler(*ctx, cmd)
}

func setFlagFromConfig(cmd *cobra.Command, flag string) error {
	if viper.GetString(flag) != "" && cmd.Flags().Lookup(flag) != nil {
		err := cmd.Flags().Set(flag, viper.GetString(flag))
		if err != nil {
			return err
		}
	}
	return nil
}
