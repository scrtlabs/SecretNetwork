package keeper

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	authz "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
	cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"

	v010cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"

	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	feegrant "github.com/cosmos/cosmos-sdk/x/feegrant"
	feegrantmodule "github.com/cosmos/cosmos-sdk/x/feegrant/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/cosmos/ibc-go/v4/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	ibcclient "github.com/cosmos/ibc-go/v4/modules/core/02-client/client"
	ibchost "github.com/cosmos/ibc-go/v4/modules/core/24-host"
	ibckeeper "github.com/cosmos/ibc-go/v4/modules/core/keeper"

	"github.com/stretchr/testify/require"

	tmenclave "github.com/scrtlabs/tm-secret-enclave"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"

	dbm "github.com/tendermint/tm-db"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/module"
	sdksigning "github.com/cosmos/cosmos-sdk/types/tx/signing"

	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/cosmos/cosmos-sdk/x/capability"

	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"

	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/cosmos/cosmos-sdk/x/mint"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	paramproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"

	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"

	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"

	wasmtypes "github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
	"github.com/scrtlabs/SecretNetwork/x/registration"
)

//const (
//	flagLRUCacheSize  = "lru_size"
//	flagQueryGasLimit = "query_gas_limit"
//)

const (
	hackAtomContract            = "hackatom.wasm"
	v010Contract                = "contract.wasm"
	v010MigratedContract        = "contract-v2.wasm"
	v1Contract                  = "v1-contract.wasm"
	v1MigratedContract          = "v1-contract-v2.wasm"
	plaintextLogsContract       = "plaintext_logs.wasm"
	ibcContract                 = "ibc.wasm"
	v010WithFloats              = "contract_with_floats.wasm"
	tooHighMemoryContract       = "too-high-initial-memory.wasm"
	staticTooHighMemoryContract = "static-too-high-initial-memory.wasm"
	evaporateContract           = "evaporate.wasm"
	randomContract              = "v1_random_test.wasm"
	benchContract               = "bench_contract.wasm"
	migrateContractV1           = "migrate_contract_v1.wasm"
	migrateContractV2           = "migrate_contract_v2.wasm"
)

const contractPath = "testdata"

var TestContractPaths = map[string]string{
	hackAtomContract:            filepath.Join(".", contractPath, hackAtomContract),
	v010Contract:                filepath.Join(".", contractPath, v010Contract),
	v010MigratedContract:        filepath.Join(".", contractPath, v010MigratedContract),
	v1Contract:                  filepath.Join(".", contractPath, v1Contract),
	v1MigratedContract:          filepath.Join(".", contractPath, v1MigratedContract),
	plaintextLogsContract:       filepath.Join(".", contractPath, plaintextLogsContract),
	ibcContract:                 filepath.Join(".", contractPath, ibcContract),
	v010WithFloats:              filepath.Join(".", contractPath, v010WithFloats),
	tooHighMemoryContract:       filepath.Join(".", contractPath, tooHighMemoryContract),
	staticTooHighMemoryContract: filepath.Join(".", contractPath, staticTooHighMemoryContract),
	benchContract:               filepath.Join(".", contractPath, benchContract),
	evaporateContract:           filepath.Join(".", contractPath, evaporateContract),
	randomContract:              filepath.Join(".", contractPath, randomContract),
	migrateContractV1:           filepath.Join(".", contractPath, migrateContractV1),
	migrateContractV2:           filepath.Join(".", contractPath, migrateContractV2),
}

// _                                   = sdkerrors.Wrap(wasmtypes.ErrExecuteFailed, "Out of gas")
var _ wasmtypes.ICS20TransferPortSource = &MockIBCTransferKeeper{}

type ContractEvent []v010cosmwasm.LogAttribute

type MigrateResult struct {
	Nonce      []byte
	Ctx        sdk.Context
	Data       []byte
	WasmEvents []ContractEvent
	GasUsed    uint64
}

type UpdateAdminResult struct {
	Ctx     sdk.Context
	GasUsed uint64
}

type ExecResult struct {
	Nonce      []byte
	Ctx        sdk.Context
	Data       []byte
	WasmEvents []ContractEvent
	GasUsed    uint64
}

type ErrorResult struct {
	CosmWasm *cosmwasm.StdError
	Generic  error
}

func (a ErrorResult) Error() string {
	switch {
	case a.CosmWasm != nil:
		return a.CosmWasm.Error()
	case a.Generic != nil:
		return a.Generic.Error()
	default:
		panic("unknown error variant")
	}
}

type MockIBCTransferKeeper struct {
	GetPortFn func(ctx sdk.Context) string
}

func (m MockIBCTransferKeeper) GetPort(ctx sdk.Context) string {
	if m.GetPortFn == nil {
		panic("not expected to be called")
	}
	return m.GetPortFn(ctx)
}

var ModuleBasics = module.NewBasicManager(
	authz.AppModuleBasic{},
	auth.AppModuleBasic{},
	genutil.AppModuleBasic{},
	bank.AppModuleBasic{},
	capability.AppModuleBasic{},
	staking.AppModuleBasic{},
	mint.AppModuleBasic{},
	distribution.AppModuleBasic{},
	gov.NewAppModuleBasic(
		paramsclient.ProposalHandler,
		distrclient.ProposalHandler,
		upgradeclient.ProposalHandler,
		upgradeclient.CancelProposalHandler,
		ibcclient.UpdateClientProposalHandler,
		ibcclient.UpgradeProposalHandler,
	),
	params.AppModuleBasic{},
	crisis.AppModuleBasic{},
	slashing.AppModuleBasic{},
	upgrade.AppModuleBasic{},
	evidence.AppModuleBasic{},
	transfer.AppModuleBasic{},
	vesting.AppModuleBasic{},
	feegrantmodule.AppModuleBasic{},

	registration.AppModuleBasic{},
)

func MakeTestCodec() codec.Codec {
	return MakeEncodingConfig().Marshaler
}

func MakeEncodingConfig() simappparams.EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txCfg := authtx.NewTxConfig(marshaler, authtx.DefaultSignModes)

	std.RegisterInterfaces(interfaceRegistry)
	std.RegisterLegacyAminoCodec(amino)

	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(amino)
	wasmtypes.RegisterInterfaces(interfaceRegistry)
	wasmtypes.RegisterLegacyAminoCodec(amino)
	return simappparams.EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          txCfg,
		Amino:             amino,
	}
}

var TestingStakeParams = stakingtypes.Params{
	UnbondingTime:     100,
	MaxValidators:     10,
	MaxEntries:        10,
	HistoricalEntries: 10,
	BondDenom:         sdk.DefaultBondDenom,
}

type TestKeepers struct {
	AccountKeeper authkeeper.AccountKeeper
	StakingKeeper stakingkeeper.Keeper
	WasmKeeper    Keeper
	DistKeeper    distrkeeper.Keeper
	GovKeeper     govkeeper.Keeper
	BankKeeper    bankkeeper.Keeper
	MintKeeper    mintkeeper.Keeper
}

var TestConfig = TestConfigType{
	ChainID: "test-secret-X",
}

type TestConfigType struct {
	ChainID string
}

// encoders can be nil to accept the defaults, or set it to override some of the message handlers (like default)
func CreateTestInput(t *testing.T, isCheckTx bool, supportedFeatures string, encoders *MessageEncoders, queriers *QueryPlugins) (sdk.Context, TestKeepers) {
	tempDir, err := os.MkdirTemp("", "wasm")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	// keyContract := sdk.NewKVStoreKey(wasmtypes.StoreKey)
	// keyAcc := sdk.NewKVStoreKey(authtypes.StoreKey)
	// keyStaking := sdk.NewKVStoreKey(stakingtypes.StoreKey)
	// keyDistro := sdk.NewKVStoreKey(distrtypes.StoreKey)
	// mintStore := sdk.NewKVStoreKey(minttypes.StoreKey)
	// keyParams := sdk.NewKVStoreKey(paramstypes.StoreKey)
	// tkeyParams := sdk.NewTransientStoreKey(paramstypes.TStoreKey)
	// keyGov := sdk.NewKVStoreKey(govtypes.StoreKey)
	// keyBank := sdk.NewKVStoreKey(banktypes.StoreKey)

	keys := sdk.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, ibchost.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, ibctransfertypes.StoreKey,
		capabilitytypes.StoreKey, feegrant.StoreKey, authzkeeper.StoreKey,
		wasmtypes.StoreKey,
	)

	db := dbm.NewMemDB()

	ms := store.NewCommitMultiStore(db)
	for _, v := range keys {
		ms.MountStoreWithDB(v, sdk.StoreTypeIAVL, db)
	}

	tkeys := sdk.NewTransientStoreKeys(paramstypes.TStoreKey)
	for _, v := range tkeys {
		ms.MountStoreWithDB(v, sdk.StoreTypeTransient, db)
	}

	memKeys := sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
	for _, v := range memKeys {
		ms.MountStoreWithDB(v, sdk.StoreTypeMemory, db)
	}

	require.NoError(t, ms.LoadLatestVersion())

	ctx := sdk.NewContext(ms, tmproto.Header{
		Height:  1234567,
		Time:    time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		ChainID: TestConfig.ChainID,
	}, isCheckTx, log.NewNopLogger())
	encodingConfig := MakeEncodingConfig()
	paramsKeeper := paramskeeper.NewKeeper(
		encodingConfig.Marshaler,
		encodingConfig.Amino,
		keys[paramstypes.StoreKey],
		tkeys[paramstypes.TStoreKey],
	)
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(crisistypes.ModuleName)
	paramsKeeper.Subspace(ibchost.ModuleName)

	// this is also used to initialize module accounts (so nil is meaningful here)
	maccPerms := map[string][]string{
		faucetAccountName:              {authtypes.Burner, authtypes.Minter},
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
	}
	authSubsp, _ := paramsKeeper.GetSubspace(authtypes.ModuleName)
	authKeeper := authkeeper.NewAccountKeeper(
		encodingConfig.Marshaler,
		keys[authtypes.StoreKey], // target store
		authSubsp,
		authtypes.ProtoBaseAccount, // prototype
		maccPerms,
	)
	blockedAddrs := make(map[string]bool)
	for acc := range maccPerms {
		allowReceivingFunds := acc != distrtypes.ModuleName
		blockedAddrs[authtypes.NewModuleAddress(acc).String()] = allowReceivingFunds
	}

	bankSubsp, _ := paramsKeeper.GetSubspace(banktypes.ModuleName)
	bankKeeper := bankkeeper.NewBaseKeeper(
		encodingConfig.Marshaler,
		keys[banktypes.StoreKey],
		authKeeper,
		bankSubsp,
		blockedAddrs,
	)

	// bankParams = bankParams.SetSendEnabledParam(sdk.DefaultBondDenom, true)
	bankKeeper.SetParams(ctx, banktypes.DefaultParams())

	stakingSubsp, _ := paramsKeeper.GetSubspace(stakingtypes.ModuleName)
	stakingKeeper := stakingkeeper.NewKeeper(
		encodingConfig.Marshaler,
		keys[stakingtypes.StoreKey],
		authKeeper,
		bankKeeper,
		stakingSubsp,
	)
	stakingKeeper.SetParams(ctx, TestingStakeParams)

	// mintSubsp, _ := paramsKeeper.GetSubspace(minttypes.ModuleName)

	// mintKeeper := mintkeeper.NewKeeper(encodingConfig.Marshaler,
	//	keyBank,
	//	mintSubsp,
	//	stakingKeeper,
	//	authKeeper,
	//	bankKeeper,
	//	authtypes.FeeCollectorName,
	//	)
	//
	// bankkeeper.SetSupply(ctx, banktypes.NewSupply(sdk.NewCoins((sdk.NewInt64Coin("stake", 1)))))

	distSubsp, _ := paramsKeeper.GetSubspace(distrtypes.ModuleName)
	distKeeper := distrkeeper.NewKeeper(
		encodingConfig.Marshaler,
		keys[distrtypes.StoreKey],
		distSubsp,
		authKeeper,
		bankKeeper,
		stakingKeeper,
		authtypes.FeeCollectorName,
		nil,
	)

	// set genesis items required for distribution
	distKeeper.SetParams(ctx, distrtypes.DefaultParams())
	distKeeper.SetFeePool(ctx, distrtypes.InitialFeePool())
	stakingKeeper.SetHooks(stakingtypes.NewMultiStakingHooks(distKeeper.Hooks()))

	// set some funds ot pay out validatores, based on code from:
	// https://github.com/cosmos/cosmos-sdk/blob/fea231556aee4d549d7551a6190389c4328194eb/x/distribution/keeper/keeper_test.go#L50-L57
	// distrAcc := distKeeper.GetDistributionAccount(ctx)
	distrAcc := authtypes.NewEmptyModuleAccount(distrtypes.ModuleName)

	totalSupply := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(2000000)))
	err = bankKeeper.MintCoins(ctx, faucetAccountName, totalSupply)
	require.NoError(t, err)

	// err = bankKeeper.SendCoinsFromModuleToAccount(ctx, faucetAccountName, distrAcc.GetAddress(), totalSupply)
	// require.NoError(t, err)

	notBondedPool := authtypes.NewEmptyModuleAccount(stakingtypes.NotBondedPoolName, authtypes.Burner, authtypes.Staking)
	bondPool := authtypes.NewEmptyModuleAccount(stakingtypes.BondedPoolName, authtypes.Burner, authtypes.Staking)
	feeCollectorAcc := authtypes.NewEmptyModuleAccount(authtypes.FeeCollectorName)

	authKeeper.SetModuleAccount(ctx, distrAcc)
	authKeeper.SetModuleAccount(ctx, bondPool)
	authKeeper.SetModuleAccount(ctx, notBondedPool)
	authKeeper.SetModuleAccount(ctx, feeCollectorAcc)

	err = bankKeeper.SendCoinsFromModuleToModule(ctx, faucetAccountName, stakingtypes.NotBondedPoolName, totalSupply)
	require.NoError(t, err)

	router := baseapp.NewRouter()
	bh := bank.NewHandler(bankKeeper)
	router.AddRoute(sdk.NewRoute(banktypes.RouterKey, bh))
	sh := staking.NewHandler(stakingKeeper)
	router.AddRoute(sdk.NewRoute(stakingtypes.RouterKey, sh))
	dh := distribution.NewHandler(distKeeper)
	router.AddRoute(sdk.NewRoute(distrtypes.RouterKey, dh))

	govRouter := govtypes.NewRouter().
		AddRoute(govtypes.RouterKey, govtypes.ProposalHandler).
		AddRoute(paramproposal.RouterKey, params.NewParamChangeProposalHandler(paramsKeeper)).
		AddRoute(distrtypes.RouterKey, distribution.NewCommunityPoolSpendProposalHandler(distKeeper))
	// AddRoute(wasmtypes.RouterKey, NewWasmProposalHandler(keeper, wasmtypes.EnableAllProposals))

	govKeeper := govkeeper.NewKeeper(
		encodingConfig.Marshaler, keys[govtypes.StoreKey], paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable()), authKeeper, bankKeeper, stakingKeeper, govRouter,
	)

	// bank := bankKeeper.
	// bk := bank.Keeper(bankKeeper)

	mintSubsp, _ := paramsKeeper.GetSubspace(minttypes.ModuleName)
	mintKeeper := mintkeeper.NewKeeper(encodingConfig.Marshaler, keys[minttypes.StoreKey], mintSubsp, stakingKeeper, authKeeper, bankKeeper, authtypes.FeeCollectorName)
	mintKeeper.SetMinter(ctx, minttypes.DefaultInitialMinter())

	// keeper := NewKeeper(cdc, keyContract, accountKeeper, &bk, &govKeeper, &distKeeper, &mintKeeper, &stakingKeeper, router, tempDir, wasmConfig, supportedFeatures, encoders, queriers)
	//// add wasm handler so we can loop-back (contracts calling contracts)
	// router.AddRoute(wasmtypes.RouterKey, TestHandler(keeper))

	govKeeper.SetProposalID(ctx, govtypes.DefaultStartingProposalID)
	govKeeper.SetDepositParams(ctx, govtypes.DefaultDepositParams())
	govKeeper.SetVotingParams(ctx, govtypes.DefaultVotingParams())
	govKeeper.SetTallyParams(ctx, govtypes.DefaultTallyParams())
	gh := gov.NewHandler(govKeeper)
	router.AddRoute(sdk.NewRoute(govtypes.RouterKey, gh))

	// Load default wasm config
	wasmConfig := wasmtypes.DefaultWasmConfig()

	//keys := sdk.NewKVStoreKeys(
	//	authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
	//	minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
	//	govtypes.StoreKey, paramstypes.StoreKey, ibchost.StoreKey, upgradetypes.StoreKey,
	//	evidencetypes.StoreKey, ibctransfertypes.StoreKey, capabilitytypes.StoreKey, "compute",
	//	feegrant.StoreKey, authzkeeper.StoreKey, icahosttypes.StoreKey,
	//)
	//memKeys := sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
	//for _, v := range memKeys {
	//	ms.MountStoreWithDB(v, sdk.StoreTypeMemory, db)
	//}

	upgradeKeeper := upgradekeeper.NewKeeper(
		map[int64]bool{},
		keys[upgradetypes.StoreKey],
		encodingConfig.Marshaler,
		tempDir,
		nil,
	)

	capabilityKeeper := capabilitykeeper.NewKeeper(
		encodingConfig.Marshaler,
		keys[capabilitytypes.StoreKey],
		memKeys[capabilitytypes.MemStoreKey],
	)

	scopedIBCKeeper := capabilityKeeper.ScopeToModule(ibchost.ModuleName)
	scopedWasmKeeper := capabilityKeeper.ScopeToModule(wasmtypes.ModuleName)

	ibchostSubSp, _ := paramsKeeper.GetSubspace(ibchost.ModuleName)
	ibcKeeper := ibckeeper.NewKeeper(
		encodingConfig.Marshaler,
		keys[ibchost.StoreKey],
		ibchostSubSp,
		stakingKeeper,
		upgradeKeeper,
		scopedIBCKeeper,
	)

	// todo: new grpc routing
	// serviceRouter := baseapp.NewMsgServiceRouter()

	// serviceRouter.SetInterfaceRegistry(encodingConfig.InterfaceRegistry)
	// bankMsgServer := bankkeeper.NewMsgServerImpl(bankKeeper)
	// stakingMsgServer := stakingkeeper.NewMsgServerImpl(stakingKeeper)
	// distrMsgServer := distrkeeper.NewMsgServerImpl(distKeeper)
	// wasmMsgServer := NewMsgServerImpl(keeper)

	// banktypes.RegisterMsgServer(serviceRouter, bankMsgServer)
	// stakingtypes.RegisterMsgServer(serviceRouter, stakingMsgServer)
	// distrtypes.RegisterMsgServer(serviceRouter, distrMsgServer)

	queryRouter := baseapp.NewGRPCQueryRouter()
	queryRouter.SetInterfaceRegistry(encodingConfig.InterfaceRegistry)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(encodingConfig.InterfaceRegistry)

	bappTxMngr := baseapp.LastMsgMarkerContainer{}

	keeper := NewKeeper(
		encodingConfig.Marshaler,
		*encodingConfig.Amino,
		keys[wasmtypes.StoreKey],
		authKeeper,
		bankKeeper,
		govKeeper,
		distKeeper,
		mintKeeper,
		stakingKeeper,
		// serviceRouter,
		scopedWasmKeeper,
		ibcKeeper.PortKeeper,
		MockIBCTransferKeeper{},
		ibcKeeper.ChannelKeeper,
		nil,
		router,
		msgRouter,
		queryRouter,
		tempDir,
		wasmConfig,
		supportedFeatures,
		encoders,
		queriers,
		&bappTxMngr,
	)
	// keeper.setParams(ctx, wasmtypes.DefaultParams())
	// add wasm handler so we can loop-back (contracts calling contracts)
	router.AddRoute(sdk.NewRoute(wasmtypes.RouterKey, TestHandler(keeper)))

	random := make([]byte, 32)
	_, _ = rand.Read(random)
	keeper.SetRandomSeed(ctx, random)

	am := module.NewManager( // minimal module set that we use for message/ query tests
		bank.NewAppModule(encodingConfig.Marshaler, bankKeeper, authKeeper),
		staking.NewAppModule(encodingConfig.Marshaler, stakingKeeper, authKeeper, bankKeeper),
		distribution.NewAppModule(encodingConfig.Marshaler, distKeeper, authKeeper, bankKeeper, stakingKeeper),
		gov.NewAppModule(encodingConfig.Marshaler, govKeeper, authKeeper, bankKeeper),
	)
	am.RegisterServices(module.NewConfigurator(encodingConfig.Marshaler, msgRouter, queryRouter))
	wasmtypes.RegisterMsgServer(msgRouter, NewMsgServerImpl(keeper))
	wasmtypes.RegisterQueryServer(queryRouter, NewGrpcQuerier(keeper))

	keepers := TestKeepers{
		AccountKeeper: authKeeper,
		StakingKeeper: stakingKeeper,
		DistKeeper:    distKeeper,
		WasmKeeper:    keeper,
		GovKeeper:     govKeeper,
		BankKeeper:    bankKeeper,
		MintKeeper:    mintKeeper,
	}

	return ctx, keepers
}

// TestHandler returns a wasm handler for tests (to avoid circular imports)
func TestHandler(k Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		case *wasmtypes.MsgInstantiateContract:
			return handleInstantiate(ctx, k, msg)
		case *wasmtypes.MsgExecuteContract:
			return handleExecute(ctx, k, msg)
		default:
			errMsg := fmt.Sprintf("unrecognized wasm message type: %T", msg)
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, errMsg)
		}
	}
}

func handleInstantiate(ctx sdk.Context, k Keeper, msg *wasmtypes.MsgInstantiateContract) (*sdk.Result, error) {
	var admin sdk.AccAddress
	var err error
	if msg.Admin != "" {
		admin, err = sdk.AccAddressFromBech32(msg.Admin)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "admin")
		}
	}

	contractAddr, data, err := k.Instantiate(ctx, msg.CodeID, msg.Sender, admin, msg.InitMsg, msg.Label, msg.InitFunds, msg.CallbackSig)
	if err != nil {
		result := sdk.Result{}
		result.Data = data
		return &result, err
	}

	if data != nil {
		return &sdk.Result{
			Data:   data,
			Events: ctx.EventManager().Events().ToABCIEvents(),
		}, nil
	}

	return &sdk.Result{
		Data:   contractAddr,
		Events: ctx.EventManager().Events().ToABCIEvents(),
	}, nil
}

func handleExecute(ctx sdk.Context, k Keeper, msg *wasmtypes.MsgExecuteContract) (*sdk.Result, error) {
	res, err := k.Execute(ctx, msg.Contract, msg.Sender, msg.Msg, msg.SentFunds, msg.CallbackSig, cosmwasm.HandleTypeExecute)
	if err != nil {
		return res, err
	}

	res.Events = ctx.EventManager().Events().ToABCIEvents()
	return res, nil
}

func PrepareExecSignedTxWithMultipleMsgs(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	sender sdk.AccAddress, senderPrivKey crypto.PrivKey, secretMsgs [][]byte, contractAddress sdk.AccAddress, coins sdk.Coins,
) sdk.Context {
	creatorAcc, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, sender)
	require.NoError(t, err)

	var encryptedMsgs []sdk.Msg
	for _, msg := range secretMsgs {
		executeMsg := wasmtypes.MsgExecuteContract{
			Sender:    sender,
			Contract:  contractAddress,
			Msg:       msg,
			SentFunds: coins,
		}
		encryptedMsgs = append(encryptedMsgs, &executeMsg)
	}

	creatorAccs := make([]authtypes.AccountI, len(encryptedMsgs))
	senderPrivKeys := make([]crypto.PrivKey, len(encryptedMsgs))

	for i := range encryptedMsgs {
		creatorAccs[i] = creatorAcc
		senderPrivKeys[i] = senderPrivKey
	}

	preparedTx := NewTestTxMultiple(encryptedMsgs, creatorAccs, senderPrivKeys)

	txBytes, err := preparedTx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = wasmtypes.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)
	return ctx
}

func PrepareExecSignedTx(t *testing.T, keeper Keeper, ctx sdk.Context, sender sdk.AccAddress, privKey crypto.PrivKey, encMsg []byte, contract sdk.AccAddress, funds sdk.Coins) sdk.Context {
	creatorAcc, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, sender)
	require.NoError(t, err)

	executeMsg := wasmtypes.MsgExecuteContract{
		Sender:    sender,
		Contract:  contract,
		Msg:       encMsg,
		SentFunds: funds,
	}
	newTx := NewTestTx(&executeMsg, creatorAcc, privKey)

	txBytes, err := newTx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = wasmtypes.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)
	return ctx
}

func PrepareInitSignedTx(t *testing.T, keeper Keeper, ctx sdk.Context, creator, admin sdk.AccAddress, privKey crypto.PrivKey, encMsg []byte, codeID uint64, funds sdk.Coins) sdk.Context {
	creatorAcc, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, creator)
	require.NoError(t, err)

	initMsg := wasmtypes.MsgInstantiateContract{
		Sender:    creator,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   encMsg,
		InitFunds: funds,
		Admin:     admin.String(),
	}
	newTx := NewTestTx(&initMsg, creatorAcc, privKey)

	txBytes, err := newTx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = wasmtypes.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)
	return ctx
}

func prepareMigrateSignedTx(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress string, creator sdk.AccAddress, privKey crypto.PrivKey, encMsg []byte, codeID uint64) sdk.Context {
	creatorAcc, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, creator)
	require.NoError(t, err)

	migrateMsg := wasmtypes.MsgMigrateContract{
		Sender:   creator.String(),
		CodeID:   codeID,
		Contract: contractAddress,
		Msg:      encMsg,
	}
	newTx := NewTestTx(&migrateMsg, creatorAcc, privKey)
	txBytes, err := newTx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = wasmtypes.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)
	return ctx
}

func prepareUpdateAdminSignedTx(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress string, sender sdk.AccAddress, privKey crypto.PrivKey, newAdmin sdk.AccAddress) sdk.Context {
	senderAccount, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, sender)
	require.NoError(t, err)

	sdkMsg := wasmtypes.MsgUpdateAdmin{
		Sender:   sender.String(),
		Contract: contractAddress,
		NewAdmin: newAdmin.String(),
	}
	newTx := NewTestTx(&sdkMsg, senderAccount, privKey)
	txBytes, err := newTx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = wasmtypes.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)
	return ctx
}

func prepareClearAdminSignedTx(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress string, sender sdk.AccAddress, privKey crypto.PrivKey) sdk.Context {
	senderAccount, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, sender)
	require.NoError(t, err)

	sdkMsg := wasmtypes.MsgClearAdmin{
		Sender:   sender.String(),
		Contract: contractAddress,
	}
	newTx := NewTestTx(&sdkMsg, senderAccount, privKey)
	txBytes, err := newTx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = wasmtypes.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)
	return ctx
}

func PrepareSignedTx(t *testing.T,
	keeper Keeper,
	ctx sdk.Context,
	sender sdk.AccAddress,
	snederPrivkey crypto.PrivKey,
	msg sdk.Msg,
) sdk.Context {
	senderAccount, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, sender)
	require.NoError(t, err)

	newTx := NewTestTx(msg, senderAccount, snederPrivkey)

	txBytes, err := newTx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = wasmtypes.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)
	return ctx
}

func NewTestTx(msg sdk.Msg, creatorAcc authtypes.AccountI, privKey crypto.PrivKey) *tx.Tx {
	return NewTestTxMultiple([]sdk.Msg{msg}, []authtypes.AccountI{creatorAcc}, []crypto.PrivKey{privKey})
}

//func PrepareMultipleExecSignedTx(t *testing.T, keeper Keeper, ctx sdk.Context, sender sdk.AccAddress, privKey crypto.PrivKey, encMsg []byte, contract sdk.AccAddress, funds sdk.Coins) sdk.Context {
//	creatorAcc, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, sender)
//	require.NoError(t, err)
//
//	executeMsg := wasmtypes.MsgExecuteContract{
//		Sender:    sender,
//		Contract:  contract,
//		Msg:       encMsg,
//		SentFunds: funds,
//	}
//
//	bankMsg := banktypes.MsgSend{
//		FromAddress: sender.String(),
//		ToAddress:   sender.String(),
//		Amount:      funds,
//	}
//
//	tx := NewTestTxMultiple([]sdk.Msg{&executeMsg, &executeMsg, &bankMsg}, []authtypes.AccountI{creatorAcc, creatorAcc, creatorAcc}, []crypto.PrivKey{privKey, privKey, privKey})
//
//	txBytes, err := tx.Marshal()
//	require.NoError(t, err)
//
//  ctx = wasmtypes.WithTXCounter(ctx, 1)
//	return ctx.WithTxBytes(txBytes)
//}

func NewTestTxMultiple(msgs []sdk.Msg, creatorAccs []authtypes.AccountI, privKeys []crypto.PrivKey) *tx.Tx {
	if len(msgs) != len(creatorAccs) || len(msgs) != len(privKeys) {
		panic("length of `msgs` `creatorAccs` and `privKeys` must be the same")
	}

	// There's no need to pass values to `NewTxConfig` because they get ignored by `NewTxBuilder` anyways,
	// and we just need the builder, which can not be created any other way, apparently.
	txConfig := authtx.NewTxConfig(nil, authtx.DefaultSignModes)
	signModeHandler := txConfig.SignModeHandler()
	builder := txConfig.NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	err := builder.SetMsgs(msgs...)
	if err != nil {
		panic(err)
	}

	// This code is based on `cosmos-sdk/client/tx/tx.go::Sign()`
	var sigs []sdksigning.SignatureV2
	for _, creatorAcc := range creatorAccs {
		sig := sdksigning.SignatureV2{
			PubKey: creatorAcc.GetPubKey(),
			Data: &sdksigning.SingleSignatureData{
				SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
				Signature: nil,
			},
			Sequence: creatorAcc.GetSequence(),
		}
		sigs = append(sigs, sig)
	}
	err = builder.SetSignatures(sigs...)
	if err != nil {
		panic(err)
	}

	sigs = []sdksigning.SignatureV2{}
	for i, creatorAcc := range creatorAccs {
		privKey := privKeys[i]
		signerData := authsigning.SignerData{
			ChainID:       TestConfig.ChainID,
			AccountNumber: creatorAcc.GetAccountNumber(),
			Sequence:      creatorAcc.GetSequence(),
		}
		bytesToSign, err := signModeHandler.GetSignBytes(sdksigning.SignMode_SIGN_MODE_DIRECT, signerData, builder.GetTx())
		if err != nil {
			panic(err)
		}

		signBytes, err := privKey.Sign(bytesToSign)
		if err != nil {
			panic(err)
		}
		sig := sdksigning.SignatureV2{
			PubKey: creatorAcc.GetPubKey(),
			Data: &sdksigning.SingleSignatureData{
				SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
				Signature: signBytes,
			},
			Sequence: creatorAcc.GetSequence(),
		}
		sigs = append(sigs, sig)
	}

	err = builder.SetSignatures(sigs...)
	if err != nil {
		panic(err)
	}

	newTx, ok := builder.(protoTxProvider)
	if !ok {
		panic("failed to unwrap tx builder to protobuf tx")
	}
	return newTx.GetProtoTx()
}

func CreateFakeFundedAccount(ctx sdk.Context, am authkeeper.AccountKeeper, bk bankkeeper.Keeper, coins sdk.Coins) (sdk.AccAddress, crypto.PrivKey) {
	priv, pub, addr := keyPubAddr()
	baseAcct := authtypes.NewBaseAccountWithAddress(addr)
	_ = baseAcct.SetPubKey(pub)
	am.SetAccount(ctx, baseAcct)

	fundAccounts(ctx, am, bk, addr, coins)
	return addr, priv
}

// StoreRandomOnNewBlock is used when height is incremented in tests, the random value for the new block needs to be
// generated too (to pass as env)
//func StoreRandomOnNewBlock(ctx sdk.Context, wasmKeeper Keeper) {
//	random := make([]byte, 32)
//	rand.Read(random)
//	wasmKeeper.SetRandomSeed(ctx, random)
//}

const faucetAccountName = "faucet"

func fundAccounts(ctx sdk.Context, am authkeeper.AccountKeeper, bk bankkeeper.Keeper, addr sdk.AccAddress, coins sdk.Coins) {
	baseAcct := am.GetAccount(ctx, addr)
	if err := bk.MintCoins(ctx, faucetAccountName, coins); err != nil {
		panic(err)
	}

	_ = bk.SendCoinsFromModuleToAccount(ctx, faucetAccountName, addr, coins)

	am.SetAccount(ctx, baseAcct)
}

var keyCounter uint64

// we need to make this deterministic (same every test run), as encoded address size and thus gas cost,
// depends on the actual bytes (due to ugly CanonicalAddress encoding)
func keyPubAddr() (crypto.PrivKey, crypto.PubKey, sdk.AccAddress) {
	keyCounter++
	seed := make([]byte, 8)
	binary.BigEndian.PutUint64(seed, keyCounter)

	key := secp256k1.GenPrivKeyFromSecret(seed)
	pub := key.PubKey()
	addr := sdk.AccAddress(pub.Address())
	return key, pub, addr
}

type protoTxProvider interface {
	GetProtoTx() *tx.Tx
}

//func txBuilderToProtoTx(txBuilder client.TxBuilder) (*tx.Tx, error) {
//	protoProvider, ok := txBuilder.(protoTxProvider)
//	if !ok {
//		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidType, "expected proto tx builder, got %T", txBuilder)
//	}
//
//	return protoProvider.GetProtoTx(), nil
//}

func updateLightClientHelper(t *testing.T, ctx sdk.Context) {
	blockData := tmproto.Data{
		Txs: [][]byte{ctx.TxBytes()},
	}
	dataBz, err := blockData.Marshal()
	require.NoError(t, err)

	blockHeader := ctx.BlockHeader()

	blockId := makeBlockIDRandom()

	voteSet, valSet, vals := randVoteSet(ctx, 0, tmproto.PrecommitType, 1, 1)
	commit, err := tmtypes.MakeCommit(blockId, blockHeader.Height, 0, voteSet, vals, time.Now())
	require.NoError(t, err)

	commitBz, err := commit.ToProto().Marshal()
	require.NoError(t, err)

	blockHeader.ProposerAddress = valSet.Proposer.Address

	blockHeader.DataHash = tmtypes.Txs{ctx.TxBytes()}.Hash()
	blockHeader.AppHash = make([]byte, sha256.Size) // make it up just to pass the length check
	blockHeader.ValidatorsHash = valSet.Hash()      // unnecessary really

	headerBz, err := blockHeader.Marshal()
	require.NoError(t, err)

	valSetProto, err := valSet.ToProto()
	require.NoError(t, err)

	valSetBytes, err := valSetProto.Marshal()
	require.NoError(t, err)

	// Note: SubmitValidatorSet must come before GetRandom, as the valSetHash is used
	// in the random number encryption, and later on in the verification
	err = tmenclave.SubmitValidatorSet(valSetBytes, uint64(blockHeader.Height))
	require.NoError(t, err)

	random, proof, err := tmenclave.GetRandom(blockHeader.AppHash, uint64(blockHeader.Height))
	require.NoError(t, err)

	randomAndProofBz := append(random, proof...) //nolint:all

	_, err = api.SubmitBlockSignatures(headerBz, commitBz, dataBz, randomAndProofBz)
	require.NoError(t, err)
}

func randVoteSet(
	ctx sdk.Context,
	round int32,
	signedMsgType tmproto.SignedMsgType,
	numValidators int,
	votingPower int64,
) (*tmtypes.VoteSet, *tmtypes.ValidatorSet, []tmtypes.PrivValidator) {
	valSet, privValidators := tmtypes.RandValidatorSet(numValidators, votingPower)
	return tmtypes.NewVoteSet(ctx.ChainID(), ctx.BlockHeight(), round, signedMsgType, valSet), valSet, privValidators
}

func makeBlockIDRandom() tmtypes.BlockID {
	var (
		blockHash   = make([]byte, sha256.Size)
		partSetHash = make([]byte, sha256.Size)
	)
	rand.Read(blockHash)   //nolint: errcheck // ignore errcheck for read
	rand.Read(partSetHash) //nolint: errcheck // ignore errcheck for read
	return tmtypes.BlockID{
		Hash: blockHash,
		PartSetHeader: tmtypes.PartSetHeader{
			Total: 123,
			Hash:  partSetHash,
		},
	}
}

func txhash(t *testing.T, ctx sdk.Context) string {
	require.NotEmpty(t, ctx.TxBytes())
	txhashBz := sha256.Sum256(ctx.TxBytes())
	txhash := strings.ToUpper(hex.EncodeToString(txhashBz[:]))
	return txhash
}
