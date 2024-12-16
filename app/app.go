package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	packetforwardtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	ibcswitchtypes "github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"

	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/core/appmodule"
	txsigning "cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/tx/signing/aminojson"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	eip191 "github.com/scrtlabs/SecretNetwork/eip191"
	scrt "github.com/scrtlabs/SecretNetwork/types"

	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	sigtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	"github.com/cosmos/gogoproto/proto"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
	v1_10 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.10"
	v1_11 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.11"
	v1_12 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.12"
	v1_13 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.13"
	v1_14 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.14"
	v1_15 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.15"
	v1_4 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.4"
	v1_5 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.5"
	v1_6 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.6"
	v1_7 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.7"
	v1_8 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.8"

	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"

	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"

	"cosmossdk.io/log"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	abci "github.com/cometbft/cometbft/abci/types"
	tmjson "github.com/cometbft/cometbft/libs/json"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	stakingkeeper "github.com/cosmos/ibc-go/v8/testing/types"

	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	reg "github.com/scrtlabs/SecretNetwork/x/registration"
	"github.com/spf13/cast"

	// unnamed import of statik for swagger UI support
	_ "github.com/scrtlabs/SecretNetwork/client/docs/statik"
)

const appName = "secret"

var (
	// DefaultCLIHome default home directories for the application CLI
	homeDir, _     = os.UserHomeDir()
	DefaultCLIHome = filepath.Join(homeDir, ".secretd")

	// DefaultNodeHome sets the folder where the applcation data and configuration will be stored
	DefaultNodeHome = filepath.Join(homeDir, ".secretd")

	// Module accounts that are allowed to receive tokens
	allowedReceivingModAcc = map[string]bool{
		distrtypes.ModuleName: true,
	}

	Upgrades = []upgrades.Upgrade{
		v1_4.Upgrade,
		v1_5.Upgrade,
		v1_6.Upgrade,
		v1_7.Upgrade,
		v1_8.Upgrade,
		v1_10.Upgrade,
		v1_11.Upgrade,
		v1_12.Upgrade,
		v1_13.Upgrade,
		v1_14.Upgrade,
		v1_15.Upgrade,
	}
)

// Verify app interface at compile time
var (
	_ runtime.AppI            = (*SecretNetworkApp)(nil)
	_ servertypes.Application = (*SecretNetworkApp)(nil)
)

// SecretNetworkApp extended ABCI application
type SecretNetworkApp struct {
	*baseapp.BaseApp
	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	interfaceRegistry types.InterfaceRegistry
	txConfig          client.TxConfig

	invCheckPeriod uint
	bootstrap      bool

	// keepers
	AppKeepers keepers.SecretAppKeepers

	// the module manager
	mm *module.Manager

	// simulation manager
	sm *module.SimulationManager

	configurator module.Configurator

	event runtime.EventService
}

func (app *SecretNetworkApp) GetInterfaceRegistry() types.InterfaceRegistry {
	return app.interfaceRegistry
}

func (app *SecretNetworkApp) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

func (app *SecretNetworkApp) GetStakingKeeper() stakingkeeper.StakingKeeper {
	return *app.AppKeepers.StakingKeeper
}

func (app *SecretNetworkApp) GetIBCKeeper() *ibckeeper.Keeper {
	return app.AppKeepers.IbcKeeper
}

func (app *SecretNetworkApp) GetGovKeeper() *govkeeper.Keeper {
	return app.AppKeepers.GovKeeper
}

func (app *SecretNetworkApp) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper {
	return app.AppKeepers.ScopedIBCKeeper
}

func (app *SecretNetworkApp) AppCodec() codec.Codec {
	return app.appCodec
}

func (app *SecretNetworkApp) AutoCliOpts() autocli.AppOptions {
	modules := make(map[string]appmodule.AppModule, 0)
	for _, m := range app.mm.Modules {
		if moduleWithName, ok := m.(module.HasName); ok {
			moduleName := moduleWithName.Name()
			if appModule, ok := moduleWithName.(appmodule.AppModule); ok {
				modules[moduleName] = appModule
			}
		}
	}

	return autocli.AppOptions{
		Modules:               modules,
		ModuleOptions:         runtimeservices.ExtractAutoCLIOptions(app.mm.Modules),
		AddressCodec:          authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		ValidatorAddressCodec: authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		ConsensusAddressCodec: authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	}
}

func (app *SecretNetworkApp) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

func (app *SecretNetworkApp) RegisterTendermintService(clientCtx client.Context) {
	cmtApp := server.NewCometABCIWrapper(app)
	cmtservice.RegisterTendermintService(clientCtx, app.BaseApp.GRPCQueryRouter(), app.interfaceRegistry, cmtApp.Query)
}

func (app *SecretNetworkApp) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	nodeservice.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg)
}

func (app *SecretNetworkApp) TxConfig() client.TxConfig {
	return app.txConfig
}

// WasmWrapper allows us to use namespacing in the config file
// This is only used for parsing in the app, x/compute expects WasmConfig
type WasmWrapper struct {
	Wasm compute.WasmConfig `mapstructure:"wasm"`
}

// NewSecretNetworkApp is a constructor function for enigmaChainApp
func NewSecretNetworkApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	bootstrap bool,
	appOpts servertypes.AppOptions,
	computeConfig *compute.WasmConfig,
	baseAppOptions ...func(*baseapp.BaseApp),
) *SecretNetworkApp {
	legacyAmino := codec.NewLegacyAmino()
	interfaceRegistry := CodecOptions{
		AccAddressPrefix: scrt.Bech32PrefixAccAddr,
		ValAddressPrefix: scrt.Bech32PrefixValAddr,
	}.NewInterfaceRegistry()

	appCodec := codec.NewProtoCodec(interfaceRegistry)
	txCfg := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)

	sdk.RegisterLegacyAminoCodec(legacyAmino)
	sdk.RegisterInterfaces(interfaceRegistry)
	txtypes.RegisterInterfaces(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)

	ModuleBasics().RegisterLegacyAminoCodec(legacyAmino)
	ModuleBasics().RegisterInterfaces(interfaceRegistry)

	// BaseApp handles interactions with Tendermint through the ABCI protocol
	bApp := baseapp.NewBaseApp(appName, logger, db, txCfg.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txCfg.TxEncoder())

	// Initialize our application with the store keys it requires
	app := &SecretNetworkApp{
		BaseApp:           bApp,
		legacyAmino:       legacyAmino,
		appCodec:          appCodec,
		interfaceRegistry: interfaceRegistry,
		bootstrap:         bootstrap,
		txConfig:          txCfg,
	}

	app.AppKeepers.InitKeys()

	skipUpgradeHeights := map[int64]bool{}
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}
	homePath := cast.ToString(appOpts.Get(flags.FlagHome))
	invCheckPeriod := cast.ToUint(appOpts.Get(server.FlagInvCheckPeriod))

	app.AppKeepers.InitSdkKeepers(appCodec, legacyAmino, bApp, ModuleAccountPermissions, app.BlockedAddrs(), invCheckPeriod, skipUpgradeHeights, homePath, logger, &app.event)

	enabledSignModes := authtx.DefaultSignModes
	enabledSignModes = append(enabledSignModes, sigtypes.SignMode_SIGN_MODE_TEXTUAL)

	signingOpts, err := authtx.NewDefaultSigningOptions()
	if err != nil {
		return nil
	}
	signingOpts.FileResolver = appCodec.InterfaceRegistry()

	aminoHandler := aminojson.NewSignModeHandler(aminojson.SignModeHandlerOptions{
		FileResolver: signingOpts.FileResolver,
		TypeResolver: signingOpts.TypeResolver,
	})
	eip191Handler := eip191.NewSignModeHandler(eip191.SignModeHandlerOptions{
		AminoJsonSignModeHandler: aminoHandler,
	})

	txConfigOpts := authtx.ConfigOptions{
		EnabledSignModes:           enabledSignModes,
		TextualCoinMetadataQueryFn: txmodule.NewBankKeeperCoinMetadataQueryFn(app.AppKeepers.BankKeeper),
		CustomSignModes:            [](txsigning.SignModeHandler){*eip191Handler},
	}
	txConfig, err := authtx.NewTxConfigWithOptions(
		appCodec,
		txConfigOpts,
	)
	if err != nil {
		panic(err)
	}
	app.txConfig = txConfig

	app.AppKeepers.InitCustomKeepers(appCodec, legacyAmino, bApp, bootstrap, homePath, computeConfig)
	app.setupUpgradeStoreLoaders()

	// NOTE: we may consider parsing `appOpts` inside module constructors. For the moment
	// we prefer to be more strict in what arguments the modules expect.
	skipGenesisInvariants := cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.
	app.mm = module.NewManager(Modules(app, appCodec, skipGenesisInvariants)...)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.

	// NOTE: upgrade module is required to be prioritized
	app.mm.SetOrderPreBlockers(
		upgradetypes.ModuleName,
	)

	SetOrderBeginBlockers(app)

	// NOTE: Capability module must occur first so that it can initialize any capabilities
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	SetOrderEndBlockers(app)

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// Sets the order of Genesis - Order matters, genutil is to always come last
	SetOrderInitGenesis(app)

	// register all module routes and module queriers
	app.mm.RegisterInvariants(app.AppKeepers.CrisisKeeper)

	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	err = app.mm.RegisterServices(app.configurator)
	if err != nil {
		panic(err)
	}

	// setupUpgradeHandlers() shoulbe be called after app.mm is configured
	app.setupUpgradeHandlers()

	// initialize stores
	app.MountKVStores(app.AppKeepers.GetKeys())
	app.MountTransientStores(app.AppKeepers.GetTransientStoreKeys())
	app.MountMemoryStores(app.AppKeepers.GetMemoryStoreKeys())

	anteHandler, err := NewAnteHandler(HandlerOptions{
		HandlerOptions: ante.HandlerOptions{
			AccountKeeper:   app.AppKeepers.AccountKeeper,
			BankKeeper:      *app.AppKeepers.BankKeeper,
			FeegrantKeeper:  app.AppKeepers.FeegrantKeeper,
			SignModeHandler: app.txConfig.SignModeHandler(),
			SigGasConsumer:  ante.DefaultSigVerificationGasConsumer,
		},
		appCodec:              app.appCodec,
		govkeeper:             *app.AppKeepers.GovKeeper,
		IBCKeeper:             app.AppKeepers.IbcKeeper,
		WasmConfig:            computeConfig,
		TXCounterStoreService: app.AppKeepers.ComputeKeeper.GetStoreService(),
	})
	if err != nil {
		panic(fmt.Errorf("failed to create AnteHandler: %s", err))
	}

	// The AnteHandler handles signature verification and transaction pre-processing
	app.SetAnteHandler(anteHandler)
	// The initChainer handles translating the genesis.json file into initial state for the network
	app.SetInitChainer(app.InitChainer)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetPreBlocker(app.PreBlocker)
	app.SetEndBlocker(app.EndBlocker)

	if manager := app.BaseApp.SnapshotManager(); manager != nil {
		err := manager.RegisterExtensions(
			compute.NewWasmSnapshotter(app.BaseApp.CommitMultiStore(), app.AppKeepers.ComputeKeeper, filepath.Join(homePath, ".compute", "wasm", "wasm")),
		)
		if err != nil {
			panic(fmt.Errorf("failed to register snapshot extension: %s", err))
		}
	}

	// At startup, after all modules have been registered, check that all prot
	// annotations are correct.
	protoFiles, err := proto.MergedRegistry()
	if err != nil {
		panic(err)
	}
	err = msgservice.ValidateProtoAnnotations(protoFiles)
	if err != nil {
		// Once we switch to using protoreflect-based antehandlers, we might
		// want to panic here instead of logging a warning.
		fmt.Fprintln(os.Stderr, err.Error())
	}

	// This seals the app
	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			panic(fmt.Errorf("error loading last version: %w", err))
		}
	}

	return app
}

func (app *SecretNetworkApp) Initialize() {

	ms := app.BaseApp.CommitMultiStore() // cms is the CommitMultiStore in Cosmos SDK apps

	ctx := sdk.NewContext(ms, cmtproto.Header{}, false, app.Logger())

	_ = app.AppKeepers.ComputeKeeper.SetValidatorSetEvidence(ctx)
	//nolint:errcheck
}


// Name returns the name of the App
func (app *SecretNetworkApp) Name() string { return app.BaseApp.Name() }

// BeginBlocker application updates every begin block
func (app *SecretNetworkApp) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	return app.mm.BeginBlock(ctx)
}

func (app *SecretNetworkApp) PreBlocker(ctx sdk.Context, _ *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	return app.mm.PreBlock(ctx)
}

// EndBlocker application updates every end block
func (app *SecretNetworkApp) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.mm.EndBlock(ctx)
}

// InitChainer application update at chain initialization
func (app *SecretNetworkApp) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState GenesisState
	if err := tmjson.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	if err := app.AppKeepers.UpgradeKeeper.SetModuleVersionMap(ctx, app.mm.GetVersionMap()); err != nil {
		panic(err)
	}

	return app.mm.InitGenesis(ctx, app.appCodec, genesisState)
}

// LoadHeight loads a particular height
func (app *SecretNetworkApp) LoadHeight(height int64) error {
	return app.BaseApp.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *SecretNetworkApp) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range ModuleAccountPermissions {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	return modAccAddrs
}

// SimulationManager implements the SimulationApp interface
func (app *SecretNetworkApp) SimulationManager() *module.SimulationManager {
	return app.sm
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *SecretNetworkApp) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	// Register legacy tx routes
	// authrest.RegisterTxRoutes(clientCtx, apiSvr.Router)
	// Register new tx routes from grpc-gateway
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register new tendermint queries routes from grpc-gateway.
	cmtservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register node gRPC service for grpc-gateway.
	nodeservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register legacy and grpc-gateway routes for all modules.
	ModuleBasics().RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// register swagger API from root so that other applications can override easily
	if apiConfig.Swagger {
		RegisterSwaggerAPI(clientCtx, apiSvr.Router)
	}
}

// RegisterSwaggerAPI registers swagger route with API Server
func RegisterSwaggerAPI(_ client.Context, rtr *mux.Router) {
	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	statikServer := http.FileServer(statikFS)
	rtr.PathPrefix("/static/").Handler(http.StripPrefix("/static/", statikServer))
	rtr.PathPrefix("/swagger/").Handler(statikServer)
	rtr.PathPrefix("/openapi/").Handler(statikServer)
}

// BlockedAddrs returns all the app's module account addresses that are not
// allowed to receive external tokens.
func (app *SecretNetworkApp) BlockedAddrs() map[string]bool {
	blockedAddrs := make(map[string]bool)
	for acc := range ModuleAccountPermissions {
		blockedAddrs[authtypes.NewModuleAddress(acc).String()] = !allowedReceivingModAcc[acc]
	}

	return blockedAddrs
}

func (app *SecretNetworkApp) setupUpgradeHandlers() {
	for _, upgradeDetails := range Upgrades {
		app.AppKeepers.UpgradeKeeper.SetUpgradeHandler(
			upgradeDetails.UpgradeName,
			upgradeDetails.CreateUpgradeHandler(
				app.mm,
				&app.AppKeepers,
				app.configurator,
			),
		)
	}
}

func (app *SecretNetworkApp) setupUpgradeStoreLoaders() {
	upgradeInfo, err := app.AppKeepers.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("Failed to read upgrade info from disk %s", err))
	}

	if app.AppKeepers.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		return
	}

	for i := range Upgrades {
		if upgradeInfo.Name == Upgrades[i].UpgradeName {
			app.Logger().Info(fmt.Sprintf("Upgrade store loader for %s at height %d", upgradeInfo.Name, upgradeInfo.Height))
			app.BaseApp.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &Upgrades[i].StoreUpgrades))
		}
	}
}

// LegacyAmino returns the application's sealed codec.
func (app *SecretNetworkApp) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

func SetOrderBeginBlockers(app *SecretNetworkApp) {
	app.mm.SetOrderBeginBlockers(
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		ibcexported.ModuleName,
		ibctransfertypes.ModuleName,
		feegrant.ModuleName,
		authtypes.ModuleName,
		vestingtypes.ModuleName,
		banktypes.ModuleName,
		govtypes.ModuleName,
		crisistypes.ModuleName,
		genutiltypes.ModuleName,
		authz.ModuleName,
		paramstypes.ModuleName,
		icatypes.ModuleName,
		packetforwardtypes.ModuleName,
		ibcfeetypes.ModuleName,
		// custom modules
		compute.ModuleName,
		reg.ModuleName,
		ibcswitchtypes.ModuleName,
	)
}

func SetOrderInitGenesis(app *SecretNetworkApp) {
	app.mm.SetOrderInitGenesis(
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		vestingtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		// custom modules
		compute.ModuleName,
		reg.ModuleName,
		ibcswitchtypes.ModuleName,

		icatypes.ModuleName,

		authz.ModuleName,
		minttypes.ModuleName,
		crisistypes.ModuleName,
		ibcexported.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		ibctransfertypes.ModuleName,
		packetforwardtypes.ModuleName,

		ibcfeetypes.ModuleName,
		feegrant.ModuleName,
	)
}

func SetOrderEndBlockers(app *SecretNetworkApp) {
	app.mm.SetOrderEndBlockers(
		crisistypes.ModuleName,
		govtypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		stakingtypes.ModuleName,
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		vestingtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		ibcexported.ModuleName,
		ibctransfertypes.ModuleName,
		icatypes.ModuleName,
		ibcfeetypes.ModuleName,
		packetforwardtypes.ModuleName,
		compute.ModuleName,
		reg.ModuleName,
		ibcswitchtypes.ModuleName,
	)
}
