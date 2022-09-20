package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/scrtlabs/SecretNetwork/app/keepers"
	icaauth "github.com/scrtlabs/SecretNetwork/x/mauth"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/authz"
	ica "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts"
	icatypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/types"
	"github.com/cosmos/ibc-go/v3/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v3/modules/core"
	ibchost "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	ibckeeper "github.com/cosmos/ibc-go/v3/modules/core/keeper"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
	v1_3 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.3"
	v1_4 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.4"
	icaauthtypes "github.com/scrtlabs/SecretNetwork/x/mauth/types"

	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authrest "github.com/cosmos/cosmos-sdk/x/auth/client/rest"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/capability"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	feegrantmodule "github.com/cosmos/cosmos-sdk/x/feegrant/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	reg "github.com/scrtlabs/SecretNetwork/x/registration"
	"github.com/spf13/cast"

	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	abci "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	dbm "github.com/tendermint/tm-db"

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

	// module account permissions
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
		icatypes.ModuleName:            nil,
	}

	// Module accounts that are allowed to receive tokens
	allowedReceivingModAcc = map[string]bool{
		distrtypes.ModuleName: true,
	}

	Upgrades = []upgrades.Upgrade{v1_3.Upgrade, v1_4.Upgrade}
)

// Verify app interface at compile time
var _ simapp.App = (*SecretNetworkApp)(nil)

// SecretNetworkApp extended ABCI application
type SecretNetworkApp struct {
	*baseapp.BaseApp
	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	interfaceRegistry types.InterfaceRegistry

	invCheckPeriod uint
	bootstrap      bool

	// keepers
	AppKeepers keepers.SecretAppKeepers

	// the module manager
	mm *module.Manager

	// simulation manager
	sm *module.SimulationManager

	configurator module.Configurator
}

func (app *SecretNetworkApp) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

func (app *SecretNetworkApp) GetStakingKeeper() stakingkeeper.Keeper {
	return *app.AppKeepers.StakingKeeper
}

func (app *SecretNetworkApp) GetIBCKeeper() *ibckeeper.Keeper {
	return app.AppKeepers.IbcKeeper
}

func (app *SecretNetworkApp) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper {
	return app.AppKeepers.ScopedIBCKeeper
}

func (app *SecretNetworkApp) GetTxConfig() client.TxConfig {
	return MakeEncodingConfig().TxConfig
}

func (app *SecretNetworkApp) AppCodec() codec.Codec {
	return app.appCodec
}

func (app *SecretNetworkApp) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

func (app *SecretNetworkApp) RegisterTendermintService(clientCtx client.Context) {
	tmservice.RegisterTendermintService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.interfaceRegistry)
}

// WasmWrapper allows us to use namespacing in the config file
// This is only used for parsing in the app, x/compute expects WasmConfig
type WasmWrapper struct {
	Wasm compute.WasmConfig `mapstructure:"wasm"`
}

// NewSecretNetworkApp is a constructor function for enigmaChainApp
func NewSecretNetworkApp(
	logger tmlog.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	skipUpgradeHeights map[int64]bool,
	homePath string,
	invCheckPeriod uint,
	bootstrap bool,
	appOpts servertypes.AppOptions,
	computeConfig *compute.WasmConfig,
	baseAppOptions ...func(*baseapp.BaseApp),
) *SecretNetworkApp {
	encodingConfig := MakeEncodingConfig()
	appCodec, legacyAmino := encodingConfig.Marshaler, encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry

	// BaseApp handles interactions with Tendermint through the ABCI protocol
	bApp := baseapp.NewBaseApp(appName, logger, db, encodingConfig.TxConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	// bApp.GRPCQueryRouter().RegisterSimulateService(bApp.Simulate, interfaceRegistry)

	// Initialize our application with the store keys it requires
	app := &SecretNetworkApp{
		BaseApp:           bApp,
		legacyAmino:       legacyAmino,
		appCodec:          appCodec,
		interfaceRegistry: interfaceRegistry,
		invCheckPeriod:    invCheckPeriod,
		bootstrap:         bootstrap,
	}

	app.AppKeepers.InitKeys()

	app.AppKeepers.InitSdkKeepers(appCodec, legacyAmino, bApp, maccPerms, app.BlockedAddrs(), invCheckPeriod, skipUpgradeHeights, homePath)
	app.AppKeepers.InitCustomKeepers(appCodec, legacyAmino, bApp, bootstrap, homePath, computeConfig)
	app.setupUpgradeStoreLoaders()

	// NOTE: we may consider parsing `appOpts` inside module constructors. For the moment
	// we prefer to be more strict in what arguments the modules expect.
	skipGenesisInvariants := cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	icaModule := ica.NewAppModule(app.AppKeepers.ICAControllerKeeper, app.AppKeepers.ICAHostKeeper)
	icaAuthModule := icaauth.NewAppModule(appCodec, *app.AppKeepers.ICAAuthKeeper)

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.
	app.mm = module.NewManager(
		genutil.NewAppModule(app.AppKeepers.AccountKeeper, app.AppKeepers.StakingKeeper, app.BaseApp.DeliverTx, encodingConfig.TxConfig),
		auth.NewAppModule(appCodec, *app.AppKeepers.AccountKeeper, authsims.RandomGenesisAccounts),
		bank.NewAppModule(appCodec, *app.AppKeepers.BankKeeper, app.AppKeepers.AccountKeeper),
		capability.NewAppModule(appCodec, *app.AppKeepers.CapabilityKeeper),
		crisis.NewAppModule(app.AppKeepers.CrisisKeeper, skipGenesisInvariants),
		feegrantmodule.NewAppModule(appCodec, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, *app.AppKeepers.FeegrantKeeper, app.interfaceRegistry),
		gov.NewAppModule(app.appCodec, *app.AppKeepers.GovKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper),
		mint.NewAppModule(appCodec, *app.AppKeepers.MintKeeper, app.AppKeepers.AccountKeeper),
		slashing.NewAppModule(appCodec, *app.AppKeepers.SlashingKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, *app.AppKeepers.StakingKeeper),
		distr.NewAppModule(appCodec, *app.AppKeepers.DistrKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, *app.AppKeepers.StakingKeeper),
		staking.NewAppModule(appCodec, *app.AppKeepers.StakingKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper),
		upgrade.NewAppModule(*app.AppKeepers.UpgradeKeeper),
		evidence.NewAppModule(*app.AppKeepers.EvidenceKeeper),
		compute.NewAppModule(*app.AppKeepers.ComputeKeeper),
		params.NewAppModule(*app.AppKeepers.ParamsKeeper),
		authzmodule.NewAppModule(appCodec, *app.AppKeepers.AuthzKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, app.interfaceRegistry),
		reg.NewAppModule(*app.AppKeepers.RegKeeper),
		ibc.NewAppModule(app.AppKeepers.IbcKeeper),
		transfer.NewAppModule(*app.AppKeepers.TransferKeeper),
		icaModule,
		icaAuthModule,
	)
	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.

	app.mm.SetOrderBeginBlockers(
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		ibchost.ModuleName,
		ibctransfertypes.ModuleName,
		feegrant.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		govtypes.ModuleName,
		crisistypes.ModuleName,
		genutiltypes.ModuleName,
		authz.ModuleName,
		paramstypes.ModuleName,
		icatypes.ModuleName,
		icaauthtypes.ModuleName,
		// custom modules
		compute.ModuleName,
		reg.ModuleName,
	)

	// NOTE: Capability module must occur first so that it can initialize any capabilities
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	app.mm.SetOrderEndBlockers(
		crisistypes.ModuleName,
		govtypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		stakingtypes.ModuleName,
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		ibchost.ModuleName,
		ibctransfertypes.ModuleName,
		icatypes.ModuleName,
		icaauthtypes.ModuleName,
		compute.ModuleName,
		reg.ModuleName,
	)

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// Sets the order of Genesis - Order matters, genutil is to always come last
	app.mm.SetOrderInitGenesis(
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
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

		icatypes.ModuleName,
		icaauthtypes.ModuleName,

		authz.ModuleName,
		minttypes.ModuleName,
		crisistypes.ModuleName,
		ibchost.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		ibctransfertypes.ModuleName,
		feegrant.ModuleName,
	)

	// register all module routes and module queriers
	app.mm.RegisterInvariants(app.AppKeepers.CrisisKeeper)
	app.mm.RegisterRoutes(app.BaseApp.Router(), app.BaseApp.QueryRouter(), encodingConfig.Amino)

	app.configurator = module.NewConfigurator(app.appCodec, app.BaseApp.MsgServiceRouter(), app.BaseApp.GRPCQueryRouter())
	app.mm.RegisterServices(app.configurator)

	// setupUpgradeHandlers() shoulbe be called after app.mm is configured
	app.setupUpgradeHandlers()

	// initialize stores
	app.BaseApp.MountKVStores(app.AppKeepers.GetKeys())
	app.BaseApp.MountTransientStores(app.AppKeepers.GetTransientStoreKeys())
	app.BaseApp.MountMemoryStores(app.AppKeepers.GetMemoryStoreKeys())

	anteHandler, err := NewAnteHandler(HandlerOptions{
		HandlerOptions: ante.HandlerOptions{
			AccountKeeper:   app.AppKeepers.AccountKeeper,
			BankKeeper:      *app.AppKeepers.BankKeeper,
			FeegrantKeeper:  app.AppKeepers.FeegrantKeeper,
			SignModeHandler: encodingConfig.TxConfig.SignModeHandler(),
			SigGasConsumer:  ante.DefaultSigVerificationGasConsumer,
		},
		IBCKeeper:         app.AppKeepers.IbcKeeper,
		WasmConfig:        computeConfig,
		TXCounterStoreKey: app.AppKeepers.GetKey(compute.StoreKey),
	})
	if err != nil {
		panic(fmt.Errorf("failed to create AnteHandler: %s", err))
	}

	// The AnteHandler handles signature verification and transaction pre-processing
	app.BaseApp.SetAnteHandler(anteHandler)
	// The initChainer handles translating the genesis.json file into initial state for the network
	app.BaseApp.SetInitChainer(app.InitChainer)
	app.BaseApp.SetBeginBlocker(app.BeginBlocker)
	app.BaseApp.SetEndBlocker(app.EndBlocker)

	if manager := app.BaseApp.SnapshotManager(); manager != nil {
		err := manager.RegisterExtensions(
			compute.NewWasmSnapshotter(app.BaseApp.CommitMultiStore(), app.AppKeepers.ComputeKeeper),
		)
		if err != nil {
			panic(fmt.Errorf("failed to register snapshot extension: %s", err))
		}
	}

	// This seals the app
	if loadLatest {
		err := app.BaseApp.LoadLatestVersion()
		if err != nil {
			tmos.Exit(err.Error())
		}
	}

	return app
}

// Name returns the name of the App
func (app *SecretNetworkApp) Name() string { return app.BaseApp.Name() }

// BeginBlocker application updates every begin block
func (app *SecretNetworkApp) BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	return app.mm.BeginBlock(ctx, req)
}

// EndBlocker application updates every end block
func (app *SecretNetworkApp) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
	return app.mm.EndBlock(ctx, req)
}

// InitChainer application update at chain initialization
func (app *SecretNetworkApp) InitChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
	var genesisState simapp.GenesisState
	if err := tmjson.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	app.AppKeepers.UpgradeKeeper.SetModuleVersionMap(ctx, app.mm.GetVersionMap())

	return app.mm.InitGenesis(ctx, app.appCodec, genesisState)
}

// LoadHeight loads a particular height
func (app *SecretNetworkApp) LoadHeight(height int64) error {
	return app.BaseApp.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *SecretNetworkApp) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
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
	rpc.RegisterRoutes(clientCtx, apiSvr.Router)
	// Register legacy tx routes
	authrest.RegisterTxRoutes(clientCtx, apiSvr.Router)
	// Register new tx routes from grpc-gateway
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register new tendermint queries routes from grpc-gateway.
	tmservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register legacy and grpc-gateway routes for all modules.
	ModuleBasics().RegisterRESTRoutes(clientCtx, apiSvr.Router)
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
	for acc := range maccPerms {
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

	for _, upgradeDetails := range Upgrades {
		if upgradeInfo.Name == upgradeDetails.UpgradeName {
			app.BaseApp.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &upgradeDetails.StoreUpgrades))
		}
	}
}

// LegacyAmino returns the application's sealed codec.
func (app *SecretNetworkApp) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}
