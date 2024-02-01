package app

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	packetforwardtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v4/router/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v4/modules/apps/29-fee/types"
	gocosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
	ibcswitchtypes "github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/kv"
	"github.com/cosmos/cosmos-sdk/types/module"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	icatypes "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	ibchost "github.com/cosmos/ibc-go/v4/modules/core/24-host"
	ibckeeper "github.com/cosmos/ibc-go/v4/modules/core/keeper"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
	v1_10 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.10"
	v1_11 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.11"
	v1_12 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.12"
	v1_3 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.3"
	v1_4 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.4"
	v1_5 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.5"
	v1_6 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.6"
	v1_7 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.7"
	v1_8 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.8"
	v1_9 "github.com/scrtlabs/SecretNetwork/app/upgrades/v1.9"

	icaauthtypes "github.com/scrtlabs/SecretNetwork/x/mauth/types"

	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authrest "github.com/cosmos/cosmos-sdk/x/auth/client/rest"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	reg "github.com/scrtlabs/SecretNetwork/x/registration"
	"github.com/spf13/cast"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/tmhash"
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

	// Module accounts that are allowed to receive tokens
	allowedReceivingModAcc = map[string]bool{
		distrtypes.ModuleName: true,
	}

	Upgrades = []upgrades.Upgrade{
		v1_3.Upgrade,
		v1_4.Upgrade,
		v1_5.Upgrade,
		v1_6.Upgrade,
		v1_7.Upgrade,
		v1_8.Upgrade,
		v1_9.Upgrade,
		v1_10.Upgrade,
		v1_11.Upgrade,
		v1_12.Upgrade,
	}
)

// Verify app interface at compile time
var (
	_ simapp.App                          = (*SecretNetworkApp)(nil)
	_ servertypes.Application             = (*SecretNetworkApp)(nil)
	_ servertypes.ApplicationQueryService = (*SecretNetworkApp)(nil)
)

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

func (app *SecretNetworkApp) GetInterfaceRegistry() types.InterfaceRegistry {
	return app.interfaceRegistry
}

func (app *SecretNetworkApp) GetCodec() codec.Codec {
	return app.appCodec
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

func (app *SecretNetworkApp) RegisterNodeService(clientCtx client.Context) {
	nodeservice.RegisterNodeService(clientCtx, app.GRPCQueryRouter())
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

	app.AppKeepers.InitSdkKeepers(appCodec, legacyAmino, bApp, ModuleAccountPermissions, app.BlockedAddrs(), invCheckPeriod, skipUpgradeHeights, homePath)
	app.AppKeepers.InitCustomKeepers(appCodec, legacyAmino, bApp, bootstrap, homePath, computeConfig)
	app.setupUpgradeStoreLoaders()

	// NOTE: we may consider parsing `appOpts` inside module constructors. For the moment
	// we prefer to be more strict in what arguments the modules expect.
	skipGenesisInvariants := cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.
	app.mm = module.NewManager(Modules(app, encodingConfig, skipGenesisInvariants)...)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.

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
			compute.NewWasmSnapshotter(app.BaseApp.CommitMultiStore(), app.AppKeepers.ComputeKeeper, filepath.Join(homePath, ".compute", "wasm", "wasm")),
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
	multiStore := app.BaseApp.CommitMultiStore()
	rootMulti, ok := multiStore.(*rootmulti.Store)
	if !ok {
		panic("app's multi store is not of type *rootmulti.Store")
	} else {
		storeRoots := storesRootsFromMultiStore(rootMulti)
		rootsBytes, err := storeRoots.Marshal()
		if err != nil {
			panic(err)
		}

		computeKv := rootMulti.GetCommitKVStore(sdk.NewKVStoreKey(compute.StoreKey))
		computeRoot := computeKv.LastCommitID().Hash

		err = gocosmwasm.SubmitModulesStoreRoots(rootsBytes, computeRoot)
		if err != nil {
			panic(err)
		}
	}

	return app.mm.BeginBlock(ctx, req)
}

func storesRootsFromMultiStore(rootMulti *rootmulti.Store) kv.Pairs { //[][]byte {
	stores := rootMulti.GetStores()
	kvs := kv.Pairs{}

	for k, v := range stores {
		// Stores of type StoreTypeTransient don't participate in AppHash calculation
		if v.GetStoreType() == sdk.StoreTypeTransient {
			continue
		}

		kvs.Pairs = append(kvs.Pairs, kv.Pair{Key: []byte(k.Name()), Value: tmhash.Sum(v.LastCommitID().Hash)})
	}

	// Have to sort in order to calculate the correct AppHash
	sort.Sort(kvs)

	return kvs
}

// This is a copy of an internal cosmos-sdk function: https://github.com/scrtlabs/cosmos-sdk/blob/1b9278476b3ac897d8ebb90241008476850bf212/store/internal/maps/maps.go#LL152C1-L152C1
// pairBytes returns key || value, with both the
// key and value length prefixed.
func pairBytes(kv kv.Pair) []byte {
	// In the worst case:
	// * 8 bytes to Uvarint encode the length of the key
	// * 8 bytes to Uvarint encode the length of the value
	// So preallocate for the worst case, which will in total
	// be a maximum of 14 bytes wasted, if len(key)=1, len(value)=1,
	// but that's going to rare.
	buf := make([]byte, 8+len(kv.Key)+8+len(kv.Value))

	// Encode the key, prefixed with its length.
	nlk := binary.PutUvarint(buf, uint64(len(kv.Key)))
	nk := copy(buf[nlk:], kv.Key)

	// Encode the value, prefixing with its length.
	nlv := binary.PutUvarint(buf[nlk+nk:], uint64(len(kv.Value)))
	nv := copy(buf[nlk+nk+nlv:], kv.Value)

	return buf[:nlk+nk+nlv+nv]
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
	rpc.RegisterRoutes(clientCtx, apiSvr.Router)
	// Register legacy tx routes
	authrest.RegisterTxRoutes(clientCtx, apiSvr.Router)
	// Register new tx routes from grpc-gateway
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register new tendermint queries routes from grpc-gateway.
	tmservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register node gRPC service for grpc-gateway.
	nodeservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

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
		ibchost.ModuleName,
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
		icaauthtypes.ModuleName,
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
		icaauthtypes.ModuleName,

		authz.ModuleName,
		minttypes.ModuleName,
		crisistypes.ModuleName,
		ibchost.ModuleName,
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
		ibchost.ModuleName,
		ibctransfertypes.ModuleName,
		icatypes.ModuleName,
		icaauthtypes.ModuleName,
		ibcfeetypes.ModuleName,
		packetforwardtypes.ModuleName,
		compute.ModuleName,
		reg.ModuleName,
		ibcswitchtypes.ModuleName,
	)
}
