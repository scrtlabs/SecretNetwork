package keepers

import (
	"path/filepath"

	ibcfee "github.com/cosmos/ibc-go/v4/modules/apps/29-fee"
	ibcpacketforward "github.com/strangelove-ventures/packet-forward-middleware/v4/router"
	ibcpacketforwardkeeper "github.com/strangelove-ventures/packet-forward-middleware/v4/router/keeper"
	ibcpacketforwardtypes "github.com/strangelove-ventures/packet-forward-middleware/v4/router/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	evidencekeeper "github.com/cosmos/cosmos-sdk/x/evidence/keeper"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	feegrantkeeper "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	paramproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	icacontroller "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/controller"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/host/types"
	"github.com/cosmos/ibc-go/v4/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v4/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	ibcclient "github.com/cosmos/ibc-go/v4/modules/core/02-client"
	ibcclienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	porttypes "github.com/cosmos/ibc-go/v4/modules/core/05-port/types"
	ibchost "github.com/cosmos/ibc-go/v4/modules/core/24-host"
	ibckeeper "github.com/cosmos/ibc-go/v4/modules/core/keeper"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	icaauthkeeper "github.com/scrtlabs/SecretNetwork/x/mauth/keeper"
	icaauthtypes "github.com/scrtlabs/SecretNetwork/x/mauth/types"
	reg "github.com/scrtlabs/SecretNetwork/x/registration"

	ibcfeekeeper "github.com/cosmos/ibc-go/v4/modules/apps/29-fee/keeper"
	ibcfeetypes "github.com/cosmos/ibc-go/v4/modules/apps/29-fee/types"

	ibchooks "github.com/scrtlabs/SecretNetwork/x/ibc-hooks"
	ibchookskeeper "github.com/scrtlabs/SecretNetwork/x/ibc-hooks/keeper"
	ibchookstypes "github.com/scrtlabs/SecretNetwork/x/ibc-hooks/types"
)

type SecretAppKeepers struct {
	// keepers
	AccountKeeper    *authkeeper.AccountKeeper
	AuthzKeeper      *authzkeeper.Keeper
	BankKeeper       *bankkeeper.BaseKeeper
	CapabilityKeeper *capabilitykeeper.Keeper
	StakingKeeper    *stakingkeeper.Keeper
	SlashingKeeper   *slashingkeeper.Keeper
	MintKeeper       *mintkeeper.Keeper
	DistrKeeper      *distrkeeper.Keeper
	GovKeeper        *govkeeper.Keeper
	CrisisKeeper     *crisiskeeper.Keeper
	UpgradeKeeper    *upgradekeeper.Keeper
	ParamsKeeper     *paramskeeper.Keeper
	EvidenceKeeper   *evidencekeeper.Keeper
	FeegrantKeeper   *feegrantkeeper.Keeper
	ComputeKeeper    *compute.Keeper
	RegKeeper        *reg.Keeper
	IbcKeeper        *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	TransferKeeper   ibctransferkeeper.Keeper

	IbcHooksKeeper      *ibchookskeeper.Keeper
	IbcFeeKeeper        ibcfeekeeper.Keeper
	PacketForwardKeeper *ibcpacketforwardkeeper.Keeper

	ICAControllerKeeper *icacontrollerkeeper.Keeper
	ICAHostKeeper       *icahostkeeper.Keeper
	ICAAuthKeeper       *icaauthkeeper.Keeper

	// make scoped keepers public for test purposes
	ScopedIBCKeeper      capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper capabilitykeeper.ScopedKeeper

	ScopedICAControllerKeeper capabilitykeeper.ScopedKeeper
	ScopedICAHostKeeper       capabilitykeeper.ScopedKeeper
	ScopedICAAuthKeeper       capabilitykeeper.ScopedKeeper

	ScopedComputeKeeper capabilitykeeper.ScopedKeeper
	//

	// keys to access the substores
	keys    map[string]*sdk.KVStoreKey
	tKeys   map[string]*sdk.TransientStoreKey
	memKeys map[string]*sdk.MemoryStoreKey
}

func (ak *SecretAppKeepers) GetKeys() map[string]*sdk.KVStoreKey {
	return ak.keys
}

func (ak *SecretAppKeepers) GetTransientStoreKeys() map[string]*sdk.TransientStoreKey {
	return ak.tKeys
}

func (ak *SecretAppKeepers) GetMemoryStoreKeys() map[string]*sdk.MemoryStoreKey {
	return ak.memKeys
}

func (ak *SecretAppKeepers) GetKey(key string) *sdk.KVStoreKey {
	return ak.keys[key]
}

// getSubspace returns a param subspace for a given module name.
func (ak *SecretAppKeepers) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := ak.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

func (ak *SecretAppKeepers) InitSdkKeepers(
	appCodec codec.Codec,
	legacyAmino *codec.LegacyAmino,
	app *baseapp.BaseApp,
	maccPerms map[string][]string,
	blockedAddresses map[string]bool,
	invCheckPeriod uint,
	skipUpgradeHeights map[int64]bool,
	homePath string,
) {
	paramsKeeper := initParamsKeeper(appCodec, legacyAmino, ak.keys[paramstypes.StoreKey], ak.tKeys[paramstypes.TStoreKey])
	ak.ParamsKeeper = &paramsKeeper

	// set the BaseApp's parameter store
	app.SetParamStore(ak.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramskeeper.ConsensusParamsKeyTable()))

	// add keepers
	accountKeeper := authkeeper.NewAccountKeeper(
		appCodec, ak.keys[authtypes.StoreKey], ak.GetSubspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, maccPerms,
	)
	ak.AccountKeeper = &accountKeeper

	bankKeeper := bankkeeper.NewBaseKeeper(
		appCodec, ak.keys[banktypes.StoreKey], ak.AccountKeeper, ak.GetSubspace(banktypes.ModuleName), blockedAddresses,
	)
	ak.BankKeeper = &bankKeeper

	stakingKeeper := stakingkeeper.NewKeeper(
		appCodec, ak.keys[stakingtypes.StoreKey], ak.AccountKeeper, *ak.BankKeeper, ak.GetSubspace(stakingtypes.ModuleName),
	)
	ak.StakingKeeper = &stakingKeeper

	mintKeeper := mintkeeper.NewKeeper(
		appCodec, ak.keys[minttypes.StoreKey], ak.GetSubspace(minttypes.ModuleName), ak.StakingKeeper,
		ak.AccountKeeper, *ak.BankKeeper, authtypes.FeeCollectorName,
	)
	ak.MintKeeper = &mintKeeper

	distrKeeper := distrkeeper.NewKeeper(
		appCodec, ak.keys[distrtypes.StoreKey], ak.GetSubspace(distrtypes.ModuleName), ak.AccountKeeper, *ak.BankKeeper,
		ak.StakingKeeper, authtypes.FeeCollectorName, blockedAddresses,
	)
	ak.DistrKeeper = &distrKeeper

	slashkingKeeper := slashingkeeper.NewKeeper(
		appCodec, ak.keys[slashingtypes.StoreKey], ak.StakingKeeper, ak.GetSubspace(slashingtypes.ModuleName),
	)
	ak.SlashingKeeper = &slashkingKeeper

	crisisKeeper := crisiskeeper.NewKeeper(
		ak.GetSubspace(crisistypes.ModuleName), invCheckPeriod, *ak.BankKeeper, authtypes.FeeCollectorName,
	)
	ak.CrisisKeeper = &crisisKeeper

	feegrantKeeper := feegrantkeeper.NewKeeper(appCodec, ak.keys[feegrant.StoreKey], ak.AccountKeeper)
	ak.FeegrantKeeper = &feegrantKeeper

	authzKeeper := authzkeeper.NewKeeper(ak.keys[authzkeeper.StoreKey], appCodec, app.MsgServiceRouter())
	ak.AuthzKeeper = &authzKeeper

	upgradeKeeper := upgradekeeper.NewKeeper(skipUpgradeHeights, ak.keys[upgradetypes.StoreKey], appCodec, homePath, app)
	ak.UpgradeKeeper = &upgradeKeeper

	// add capability keeper and ScopeToModule for ibc module
	ak.CapabilityKeeper = capabilitykeeper.NewKeeper(appCodec, ak.keys[capabilitytypes.StoreKey], ak.memKeys[capabilitytypes.MemStoreKey])
	ak.CreateScopedKeepers()

	// Create IBC Keeper
	ak.IbcKeeper = ibckeeper.NewKeeper(
		appCodec, ak.keys[ibchost.StoreKey], ak.GetSubspace(ibchost.ModuleName), ak.StakingKeeper, ak.UpgradeKeeper, ak.ScopedIBCKeeper,
	)

	// Register the proposal types
	govRouter := govtypes.NewRouter()
	govRouter.AddRoute(govtypes.RouterKey, govtypes.ProposalHandler).
		AddRoute(paramproposal.RouterKey, params.NewParamChangeProposalHandler(*ak.ParamsKeeper)).
		AddRoute(distrtypes.RouterKey, distr.NewCommunityPoolSpendProposalHandler(*ak.DistrKeeper)).
		AddRoute(upgradetypes.RouterKey, upgrade.NewSoftwareUpgradeProposalHandler(*ak.UpgradeKeeper)).
		AddRoute(ibcclienttypes.RouterKey, ibcclient.NewClientProposalHandler(ak.IbcKeeper.ClientKeeper))

	govKeeper := govkeeper.NewKeeper(
		appCodec,
		ak.keys[govtypes.StoreKey],
		ak.GetSubspace(govtypes.ModuleName),
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.StakingKeeper,
		govRouter,
	)
	ak.GovKeeper = &govKeeper

	// Create evidence keeper with router
	ak.EvidenceKeeper = evidencekeeper.NewKeeper(
		appCodec, ak.keys[evidencetypes.StoreKey], ak.StakingKeeper, ak.SlashingKeeper,
	)

	// Register the staking hooks
	// NOTE: StakingKeeper above is passed by reference, so that it will contain these hooks
	ak.StakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(
			ak.DistrKeeper.Hooks(),
			ak.SlashingKeeper.Hooks()),
	)
}

func (ak *SecretAppKeepers) CreateScopedKeepers() {
	ak.ScopedIBCKeeper = ak.CapabilityKeeper.ScopeToModule(ibchost.ModuleName)
	ak.ScopedTransferKeeper = ak.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	ak.ScopedICAControllerKeeper = ak.CapabilityKeeper.ScopeToModule(icacontrollertypes.SubModuleName)
	ak.ScopedICAHostKeeper = ak.CapabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
	ak.ScopedICAAuthKeeper = ak.CapabilityKeeper.ScopeToModule(icaauthtypes.ModuleName)
	ak.ScopedComputeKeeper = ak.CapabilityKeeper.ScopeToModule(compute.ModuleName)

	// Applications that wish to enforce statically created ScopedKeepers should call `Seal` after creating
	// their scoped modules in `NewApp` with `ScopeToModule`
	ak.CapabilityKeeper.Seal()
}

func (ak *SecretAppKeepers) InitCustomKeepers(
	appCodec codec.Codec,
	legacyAmino *codec.LegacyAmino,
	app *baseapp.BaseApp,
	bootstrap bool,
	homePath string,
	computeConfig *compute.WasmConfig,
) {
	// Just re-use the full router - do we want to limit this more?
	regRouter := app.Router()

	// Replace with bootstrap flag when we figure out how to test properly and everything works
	regKeeper := reg.NewKeeper(appCodec, ak.keys[reg.StoreKey], regRouter, reg.EnclaveApi{}, homePath, bootstrap)
	ak.RegKeeper = &regKeeper

	computeDir := filepath.Join(homePath, ".compute")
	// The last arguments can contain custom message handlers, and custom query handlers,
	// if we want to allow any custom callbacks
	supportedFeatures := "staking,stargate,ibc3,random"

	computeKeeper := compute.NewKeeper(
		appCodec,
		*legacyAmino,
		ak.keys[compute.StoreKey],
		*ak.AccountKeeper,
		ak.BankKeeper,
		*ak.GovKeeper,
		*ak.DistrKeeper,
		*ak.MintKeeper,
		*ak.StakingKeeper,
		ak.ScopedComputeKeeper,
		ak.IbcKeeper.PortKeeper,
		ak.TransferKeeper,
		ak.IbcKeeper.ChannelKeeper,
		app.Router(),
		app.MsgServiceRouter(),
		app.GRPCQueryRouter(),
		computeDir,
		computeConfig,
		supportedFeatures,
		nil,
		nil,
		&app.LastTxManager,
	)
	ak.ComputeKeeper = &computeKeeper

	// The order stuff happen:
	// 1. WASM Hooks
	// 2. Fee
	// 3. PFM
	// 4. Transfer
	//
	// Assaf: I think PFM and WASM hoosk are mutually exclusive, and I'm not sure what happens if we have both in a packet. That's also the order Osmosis uses, but they don't have the Fee middleware. It would be interesting to test this behavior. Juno does it in the same order that we do (Fee middleware included).

	// Setup the ICS4Wrapper used by the hooks middleware
	// Configure the hooks keeper
	hooksKeeper := ibchookskeeper.NewKeeper(
		ak.keys[ibchookstypes.StoreKey],
	)
	ak.IbcHooksKeeper = &hooksKeeper

	secretPrefix := sdk.GetConfig().GetBech32AccountAddrPrefix()
	wasmHooks := ibchooks.NewWasmHooks(&hooksKeeper, &computeKeeper, secretPrefix)
	ibcHooksICS4Wrapper := ibchooks.NewICS4Middleware(
		ak.IbcKeeper.ChannelKeeper,
		wasmHooks,
	)

	ak.IbcFeeKeeper = ibcfeekeeper.NewKeeper(
		appCodec,
		ak.keys[ibcfeetypes.StoreKey],
		ak.GetSubspace(ibcfeetypes.ModuleName), // this isn't even used in the keeper but is required?
		ibcHooksICS4Wrapper,
		ak.IbcKeeper.ChannelKeeper,
		&ak.IbcKeeper.PortKeeper,
		ak.AccountKeeper,
		ak.BankKeeper,
	)

	// Initialize packet forward middleware router
	ak.PacketForwardKeeper = ibcpacketforwardkeeper.NewKeeper(
		appCodec,
		ak.keys[ibcpacketforwardtypes.StoreKey],
		ak.GetSubspace(ibcpacketforwardtypes.ModuleName),
		ak.TransferKeeper,
		ak.IbcKeeper.ChannelKeeper,
		ak.DistrKeeper,
		ak.BankKeeper,
		// ak.IbcKeeper.ChannelKeeper,
		&ak.IbcFeeKeeper,
	)

	icaControllerKeeper := icacontrollerkeeper.NewKeeper(
		appCodec,
		ak.keys[icacontrollertypes.StoreKey],
		ak.GetSubspace(icacontrollertypes.SubModuleName),
		ak.IbcFeeKeeper, // integrate fee keeper with ica
		ak.IbcKeeper.ChannelKeeper,
		&ak.IbcKeeper.PortKeeper,
		ak.ScopedICAControllerKeeper,
		app.MsgServiceRouter(),
	)
	ak.ICAControllerKeeper = &icaControllerKeeper

	icaHostKeeper := icahostkeeper.NewKeeper(
		appCodec,
		ak.keys[icahosttypes.StoreKey],
		ak.GetSubspace(icahosttypes.SubModuleName),
		ak.IbcKeeper.ChannelKeeper,
		&ak.IbcKeeper.PortKeeper,
		ak.AccountKeeper,
		ak.ScopedICAHostKeeper,
		app.MsgServiceRouter(),
	)
	ak.ICAHostKeeper = &icaHostKeeper

	icaHostIBCModule := icahost.NewIBCModule(*ak.ICAHostKeeper)

	// Create Transfer Keepers
	transferKeeper := ibctransferkeeper.NewKeeper(
		appCodec,
		ak.keys[ibctransfertypes.StoreKey],
		ak.GetSubspace(ibctransfertypes.ModuleName),
		ak.PacketForwardKeeper,
		ak.IbcKeeper.ChannelKeeper,
		&ak.IbcKeeper.PortKeeper,
		ak.AccountKeeper,
		ak.BankKeeper,
		ak.ScopedTransferKeeper,
	)
	ak.TransferKeeper = transferKeeper

	ak.PacketForwardKeeper.SetTransferKeeper(ak.TransferKeeper)

	var transferStack porttypes.IBCModule
	transferStack = transfer.NewIBCModule(ak.TransferKeeper)
	transferStack = ibcpacketforward.NewIBCMiddleware(
		transferStack,
		ak.PacketForwardKeeper,
		0,
		// 10 minutes
		ibcpacketforwardkeeper.DefaultForwardTransferPacketTimeoutTimestamp,
		// 28 days
		ibcpacketforwardkeeper.DefaultRefundTransferPacketTimeoutTimestamp,
	)
	transferStack = ibcfee.NewIBCMiddleware(transferStack, ak.IbcFeeKeeper)
	transferStack = ibchooks.NewIBCMiddleware(transferStack, &ibcHooksICS4Wrapper)

	icaHostStack := ibcfee.NewIBCMiddleware(icaHostIBCModule, ak.IbcFeeKeeper)

	// initialize ICA module with mock module as the authentication module on the controller side
	var icaControllerStack porttypes.IBCModule
	icaControllerStack = icacontroller.NewIBCMiddleware(icaControllerStack, *ak.ICAControllerKeeper)
	icaControllerStack = ibcfee.NewIBCMiddleware(icaControllerStack, ak.IbcFeeKeeper)

	// Create fee enabled wasm ibc Stack
	var computeStack porttypes.IBCModule
	computeStack = compute.NewIBCHandler(ak.ComputeKeeper, ak.IbcKeeper.ChannelKeeper, ak.IbcFeeKeeper)
	computeStack = ibcfee.NewIBCMiddleware(computeStack, ak.IbcFeeKeeper)

	// Create static IBC router, add ibc-transfer module route, then set and seal it
	ibcRouter := porttypes.NewRouter()
	ibcRouter.
		AddRoute(ibctransfertypes.ModuleName, transferStack).
		AddRoute(compute.ModuleName, computeStack).
		AddRoute(icacontrollertypes.SubModuleName, icaControllerStack).
		AddRoute(icahosttypes.SubModuleName, icaHostStack)

	// Setting Router will finalize all routes by sealing router
	// No more routes can be added
	ak.IbcKeeper.SetRouter(ibcRouter)
}

func (ak *SecretAppKeepers) InitKeys() {
	ak.keys = sdk.NewKVStoreKeys(
		authtypes.StoreKey,
		banktypes.StoreKey,
		stakingtypes.StoreKey,
		minttypes.StoreKey,
		distrtypes.StoreKey,
		slashingtypes.StoreKey,
		govtypes.StoreKey,
		paramstypes.StoreKey,
		ibchost.StoreKey,
		upgradetypes.StoreKey,
		evidencetypes.StoreKey,
		ibctransfertypes.StoreKey,
		capabilitytypes.StoreKey,
		compute.StoreKey,
		reg.StoreKey,
		feegrant.StoreKey,
		authzkeeper.StoreKey,
		icahosttypes.StoreKey,
		icacontrollertypes.StoreKey,
		ibcpacketforwardtypes.StoreKey,
		ibcfeetypes.StoreKey,
		ibchookstypes.StoreKey,
	)

	ak.tKeys = sdk.NewTransientStoreKeys(paramstypes.TStoreKey)
	ak.memKeys = sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
}

// initParamsKeeper init params keeper and its subspaces
func initParamsKeeper(appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey sdk.StoreKey) paramskeeper.Keeper {
	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
	paramsKeeper.Subspace(ibchost.ModuleName)
	paramsKeeper.Subspace(icacontrollertypes.SubModuleName)
	paramsKeeper.Subspace(icahosttypes.SubModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable())
	paramsKeeper.Subspace(crisistypes.ModuleName)
	paramsKeeper.Subspace(compute.ModuleName)
	paramsKeeper.Subspace(reg.ModuleName)
	paramsKeeper.Subspace(ibcpacketforwardtypes.ModuleName).WithKeyTable(ibcpacketforwardtypes.ParamKeyTable())

	return paramsKeeper
}
