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

	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"

	"github.com/cosmos/cosmos-sdk/runtime"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	authz "github.com/cosmos/cosmos-sdk/x/authz/module"

	"github.com/cosmos/gogoproto/proto"
	// "github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
	scrt "github.com/scrtlabs/SecretNetwork/types"

	cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"

	v010cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"

	"cosmossdk.io/math"

	evidencetypes "cosmossdk.io/x/evidence/types"
	feegrant "cosmossdk.io/x/feegrant"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"

	ibchost "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	"github.com/stretchr/testify/require"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	// tmenclave "github.com/scrtlabs/tm-secret-enclave"

	dbm "github.com/cosmos/cosmos-db"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	crypto "github.com/cosmos/cosmos-sdk/crypto/types"

	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	sdksigning "github.com/cosmos/cosmos-sdk/types/tx/signing"

	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	"github.com/cosmos/ibc-go/modules/capability"

	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"cosmossdk.io/x/evidence"
	"github.com/cosmos/cosmos-sdk/types/tx"
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

	"cosmossdk.io/x/upgrade"

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
		[]govclient.ProposalHandler{
			paramsclient.ProposalHandler,
		},
	),
	params.AppModuleBasic{},
	slashing.AppModuleBasic{},
	upgrade.AppModuleBasic{},
	evidence.AppModuleBasic{},
	transfer.AppModuleBasic{},
	vesting.AppModuleBasic{},
	feegrantmodule.AppModuleBasic{},

	registration.AppModuleBasic{},
)

func MakeTestCodec() codec.Codec {
	return MakeEncodingConfig().Codec
}

func MakeEncodingConfig() moduletestutil.TestEncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry, _ := types.NewInterfaceRegistryWithOptions(types.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec: address.Bech32Codec{
				Bech32Prefix: sdk.GetConfig().GetBech32AccountAddrPrefix(),
			},
			ValidatorAddressCodec: address.Bech32Codec{
				Bech32Prefix: sdk.GetConfig().GetBech32ValidatorAddrPrefix(),
			},
		},
	})

	codec := codec.NewProtoCodec(interfaceRegistry)
	txCfg := authtx.NewTxConfig(codec, authtx.DefaultSignModes)

	std.RegisterInterfaces(interfaceRegistry)
	std.RegisterLegacyAminoCodec(amino)

	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(amino)
	wasmtypes.RegisterInterfaces(interfaceRegistry)
	wasmtypes.RegisterLegacyAminoCodec(amino)
	return moduletestutil.TestEncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             codec,
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

	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, ibchost.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, ibctransfertypes.StoreKey,
		capabilitytypes.StoreKey, feegrant.StoreKey, authzkeeper.StoreKey,
		wasmtypes.StoreKey,
	)

	db := dbm.NewMemDB()

	ms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	for _, v := range keys {
		ms.MountStoreWithDB(v, storetypes.StoreTypeIAVL, db)
	}

	tkeys := storetypes.NewTransientStoreKeys(paramstypes.TStoreKey)
	for _, v := range tkeys {
		ms.MountStoreWithDB(v, storetypes.StoreTypeTransient, db)
	}

	memKeys := storetypes.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
	for _, v := range memKeys {
		ms.MountStoreWithDB(v, storetypes.StoreTypeMemory, db)
	}

	require.NoError(t, ms.LoadLatestVersion())

	ctx := sdk.NewContext(ms, tmproto.Header{
		Height:  1234567,
		Time:    time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		ChainID: TestConfig.ChainID,
	}, isCheckTx, log.NewNopLogger())
	encodingConfig := MakeEncodingConfig()
	paramsKeeper := paramskeeper.NewKeeper(
		encodingConfig.Codec,
		encodingConfig.Amino,
		keys[paramstypes.StoreKey],
		tkeys[paramstypes.TStoreKey],
	)
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
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
	authKeeper := authkeeper.NewAccountKeeper(
		encodingConfig.Codec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]), // target store
		authtypes.ProtoBaseAccount,                          // prototype
		maccPerms,
		authcodec.NewBech32Codec(scrt.Bech32PrefixAccAddr),
		scrt.Bech32PrefixAccAddr,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	blockedAddrs := make(map[string]bool)
	for acc := range maccPerms {
		allowReceivingFunds := acc != distrtypes.ModuleName
		blockedAddrs[authtypes.NewModuleAddress(acc).String()] = allowReceivingFunds
	}

	bankSubsp, _ := paramsKeeper.GetSubspace(banktypes.ModuleName)
	bankKeeper := bankkeeper.NewBaseKeeper(
		encodingConfig.Codec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		authKeeper,
		blockedAddrs,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		log.NewNopLogger(),
	)

	// bankParams = bankParams.SetSendEnabledParam(sdk.DefaultBondDenom, true)
	err = bankKeeper.SetParams(ctx, banktypes.DefaultParams())
	require.NoError(t, err)

	stakingSubsp, _ := paramsKeeper.GetSubspace(stakingtypes.ModuleName)
	stakingKeeper := stakingkeeper.NewKeeper(
		encodingConfig.Codec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		authKeeper,
		bankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		authcodec.NewBech32Codec(scrt.Bech32PrefixValAddr),
		authcodec.NewBech32Codec(scrt.Bech32PrefixConsAddr),
	)
	err = stakingKeeper.SetParams(ctx, TestingStakeParams)
	require.NoError(t, err)
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
		encodingConfig.Codec,
		runtime.NewKVStoreService(keys[distrtypes.StoreKey]),
		authKeeper,
		bankKeeper,
		stakingKeeper,
		authtypes.FeeCollectorName,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	// set genesis items required for distribution
	err = distKeeper.Params.Set(ctx, distrtypes.DefaultParams())
	require.NoError(t, err)
	err = distKeeper.FeePool.Set(ctx, distrtypes.InitialFeePool())
	require.NoError(t, err)
	stakingKeeper.SetHooks(stakingtypes.NewMultiStakingHooks(distKeeper.Hooks()))

	// set some funds ot pay out validatores, based on code from:
	// https://github.com/cosmos/cosmos-sdk/blob/fea231556aee4d549d7551a6190389c4328194eb/x/distribution/keeper/keeper_test.go#L50-L57
	// distrAcc := distKeeper.GetDistributionAccount(ctx)
	distrAcc := authtypes.NewEmptyModuleAccount(distrtypes.ModuleName)

	totalSupply := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(2000000)))
	err = bankKeeper.MintCoins(ctx, faucetAccountName, totalSupply)
	require.NoError(t, err)

	// err = bankKeeper.SendCoinsFromModuleToAccount(ctx, faucetAccountName, distrAcc.GetAddress(), totalSupply)
	// require.NoError(t, err)

	notBondedPool := authtypes.NewEmptyModuleAccount(stakingtypes.NotBondedPoolName, authtypes.Burner, authtypes.Staking)
	bondPool := authtypes.NewEmptyModuleAccount(stakingtypes.BondedPoolName, authtypes.Burner, authtypes.Staking)
	feeCollectorAcc := authtypes.NewEmptyModuleAccount(authtypes.FeeCollectorName)

	err = distrAcc.SetAccountNumber(authKeeper.NextAccountNumber(ctx))
	require.NoError(t, err)
	err = bondPool.SetAccountNumber(authKeeper.NextAccountNumber(ctx))
	require.NoError(t, err)
	err = notBondedPool.SetAccountNumber(authKeeper.NextAccountNumber(ctx))
	require.NoError(t, err)
	err = feeCollectorAcc.SetAccountNumber(authKeeper.NextAccountNumber(ctx))
	require.NoError(t, err)

	authKeeper.SetModuleAccount(ctx, distrAcc)
	authKeeper.SetModuleAccount(ctx, bondPool)
	authKeeper.SetModuleAccount(ctx, notBondedPool)
	authKeeper.SetModuleAccount(ctx, feeCollectorAcc)

	err = bankKeeper.SendCoinsFromModuleToModule(ctx, faucetAccountName, stakingtypes.NotBondedPoolName, totalSupply)
	require.NoError(t, err)

	govRouter := govv1beta1.NewRouter().
		AddRoute(govtypes.RouterKey, govv1beta1.ProposalHandler).
		AddRoute(paramproposal.RouterKey, params.NewParamChangeProposalHandler(paramsKeeper))

	queryRouter := baseapp.NewGRPCQueryRouter()
	queryRouter.SetInterfaceRegistry(encodingConfig.InterfaceRegistry)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(encodingConfig.InterfaceRegistry)

	govKeeper := govkeeper.NewKeeper(
		encodingConfig.Codec, runtime.NewKVStoreService(keys[govtypes.StoreKey]), authKeeper, bankKeeper, stakingKeeper, distKeeper, msgRouter, govtypes.DefaultConfig(), authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	govKeeper.SetLegacyRouter(govRouter)

	// bank := bankKeeper.
	// bk := bank.Keeper(bankKeeper)

	mintKeeper := mintkeeper.NewKeeper(encodingConfig.Codec, runtime.NewKVStoreService(keys[minttypes.StoreKey]), stakingKeeper, authKeeper, bankKeeper, authtypes.FeeCollectorName, authtypes.NewModuleAddress(govtypes.ModuleName).String())
	err = mintKeeper.Minter.Set(ctx, minttypes.DefaultInitialMinter())
	require.NoError(t, err)
	// keeper := NewKeeper(cdc, keyContract, accountKeeper, &bk, &govKeeper, &distKeeper, &mintKeeper, &stakingKeeper, router, tempDir, wasmConfig, supportedFeatures, encoders, queriers)
	//// add wasm handler so we can loop-back (contracts calling contracts)
	// router.AddRoute(wasmtypes.RouterKey, TestHandler(keeper))

	err = govKeeper.ProposalID.Set(ctx, govv1.DefaultStartingProposalID)
	require.NoError(t, err)
	err = govKeeper.Params.Set(ctx, govv1.DefaultParams())
	require.NoError(t, err)

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
		runtime.NewKVStoreService(keys[upgradetypes.StoreKey]),
		encodingConfig.Codec,
		tempDir,
		nil,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	capabilityKeeper := capabilitykeeper.NewKeeper(
		encodingConfig.Codec,
		keys[capabilitytypes.StoreKey],
		memKeys[capabilitytypes.MemStoreKey],
	)

	scopedIBCKeeper := capabilityKeeper.ScopeToModule(ibchost.ModuleName)
	scopedWasmKeeper := capabilityKeeper.ScopeToModule(wasmtypes.ModuleName)

	ibchostSubSp, _ := paramsKeeper.GetSubspace(ibchost.ModuleName)
	ibcKeeper := ibckeeper.NewKeeper(
		encodingConfig.Codec,
		keys[ibchost.StoreKey],
		ibchostSubSp,
		stakingKeeper,
		upgradeKeeper,
		scopedIBCKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
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

	bappTxMngr := baseapp.LastMsgMarkerContainer{}

	keeper := NewKeeper(
		encodingConfig.Codec,
		*encodingConfig.Amino,
		runtime.NewKVStoreService(keys[wasmtypes.StoreKey]),
		authKeeper,
		bankKeeper,
		*govKeeper,
		distKeeper,
		mintKeeper,
		*stakingKeeper,
		// serviceRouter,
		scopedWasmKeeper,
		*ibcKeeper.PortKeeper,
		MockIBCTransferKeeper{},
		ibcKeeper.ChannelKeeper,
		nil,
		msgRouter,
		queryRouter,
		tempDir,
		wasmConfig,
		supportedFeatures,
		encoders,
		queriers,
		&bappTxMngr,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	// keeper.setParams(ctx, wasmtypes.DefaultParams())
	// add wasm handler so we can loop-back (contracts calling contracts)

	random := make([]byte, 32)
	_, _ = rand.Read(random)
	keeper.SetRandomSeed(ctx, random, random)
	_ = keeper.SetParams(ctx, wasmtypes.DefaultParams())

	govSubSp, _ := paramsKeeper.GetSubspace(govtypes.ModuleName)

	am := module.NewManager( // minimal module set that we use for message/ query tests
		bank.NewAppModule(encodingConfig.Codec, bankKeeper, authKeeper, bankSubsp),
		staking.NewAppModule(encodingConfig.Codec, stakingKeeper, authKeeper, bankKeeper, stakingSubsp),
		distribution.NewAppModule(encodingConfig.Codec, distKeeper, authKeeper, bankKeeper, stakingKeeper, distSubsp),
		gov.NewAppModule(encodingConfig.Codec, govKeeper, authKeeper, bankKeeper, govSubSp),
	)
	err = am.RegisterServices(module.NewConfigurator(encodingConfig.Codec, msgRouter, queryRouter))
	require.NoError(t, err)
	wasmtypes.RegisterMsgServer(msgRouter, NewMsgServerImpl(keeper))
	wasmtypes.RegisterQueryServer(queryRouter, NewGrpcQuerier(keeper))

	keepers := TestKeepers{
		AccountKeeper: authKeeper,
		StakingKeeper: *stakingKeeper,
		DistKeeper:    distKeeper,
		WasmKeeper:    keeper,
		GovKeeper:     *govKeeper,
		BankKeeper:    bankKeeper,
		MintKeeper:    mintKeeper,
	}

	return ctx, keepers
}

// TestHandler returns a wasm handler for tests (to avoid circular imports)
func TestHandler(k Keeper) baseapp.MsgServiceHandler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		case *wasmtypes.MsgInstantiateContract:
			return handleInstantiate(ctx, k, msg)
		case *wasmtypes.MsgExecuteContract:
			return handleExecute(ctx, k, msg)
		default:
			errMsg := fmt.Sprintf("unrecognized wasm message type: %T", msg)
			return nil, sdkerrors.ErrUnknownRequest.Wrap(errMsg)
		}
	}
}

func handleInstantiate(ctx sdk.Context, k Keeper, msg *wasmtypes.MsgInstantiateContract) (*sdk.Result, error) {
	var admin sdk.AccAddress
	var err error
	if msg.Admin != "" {
		admin, err = sdk.AccAddressFromBech32(msg.Admin)
		if err != nil {
			return nil, errorsmod.Wrap(err, "admin")
		}
	}

	contractAddr, data, err := k.Instantiate(ctx, msg.CodeID, []byte(msg.Sender), admin, msg.InitMsg, msg.Label, msg.InitFunds, msg.CallbackSig)
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

	creatorAccs := make([]sdk.AccountI, len(encryptedMsgs))
	senderPrivKeys := make([]crypto.PrivKey, len(encryptedMsgs))

	for i := range encryptedMsgs {
		creatorAccs[i] = creatorAcc
		senderPrivKeys[i] = senderPrivKey
	}

	preparedTx := NewTestTxMultiple(ctx, encryptedMsgs, creatorAccs, senderPrivKeys)

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
	newTx := NewTestTx(ctx, &executeMsg, creatorAcc, privKey)

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
	newTx := NewTestTx(ctx, &initMsg, creatorAcc, privKey)

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
	newTx := NewTestTx(ctx, &migrateMsg, creatorAcc, privKey)
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
	newTx := NewTestTx(ctx, &sdkMsg, senderAccount, privKey)
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
	newTx := NewTestTx(ctx, &sdkMsg, senderAccount, privKey)
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

	newTx := NewTestTx(ctx, msg, senderAccount, snederPrivkey)

	txBytes, err := newTx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = wasmtypes.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)
	return ctx
}

func NewTestTx(ctx sdk.Context, msg sdk.Msg, creatorAcc sdk.AccountI, privKey crypto.PrivKey) *tx.Tx {
	return NewTestTxMultiple(ctx, []sdk.Msg{msg}, []sdk.AccountI{creatorAcc}, []crypto.PrivKey{privKey})
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
//	tx := NewTestTxMultiple([]sdk.Msg{&executeMsg, &executeMsg, &bankMsg}, []sdk.AccountI{creatorAcc, creatorAcc, creatorAcc}, []crypto.PrivKey{privKey, privKey, privKey})
//
//	txBytes, err := tx.Marshal()
//	require.NoError(t, err)
//
//  ctx = wasmtypes.WithTXCounter(ctx, 1)
//	return ctx.WithTxBytes(txBytes)
//}

func NewTestTxMultiple(ctx sdk.Context, msgs []sdk.Msg, creatorAccs []sdk.AccountI, privKeys []crypto.PrivKey) *tx.Tx {
	if len(msgs) != len(creatorAccs) || len(msgs) != len(privKeys) {
		panic("length of `msgs` `creatorAccs` and `privKeys` must be the same")
	}

	// There's no need to pass values to `NewTxConfig` because they get ignored by `NewTxBuilder` anyways,
	// and we just need the builder, which can not be created any other way, apparently.
	txConfig := authtx.NewTxConfig(MakeTestCodec(), authtx.DefaultSignModes)
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
			PubKey:        creatorAcc.GetPubKey(),
		}
		bytesToSign, err := authsigning.GetSignBytesAdapter(ctx, signModeHandler, sdksigning.SignMode_SIGN_MODE_DIRECT, signerData, builder.GetTx())
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

func CreateFakeFundedAccount(ctx sdk.Context, am authkeeper.AccountKeeper, bk bankkeeper.Keeper, coins sdk.Coins) (sdk.AccAddress, crypto.PrivKey, crypto.PubKey) {
	priv, pub, addr := keyPubAddr()
	baseAcct := authtypes.NewBaseAccountWithAddress(addr)
	_ = baseAcct.SetPubKey(pub)
	err := baseAcct.SetAccountNumber(am.NextAccountNumber(ctx))
	if err != nil {
		ctx.Logger().Error("SetAccountNumber", "account", err.Error())
	}
	am.SetAccount(ctx, baseAcct)

	fundAccounts(ctx, am, bk, addr, coins)
	return addr, priv, pub
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

func MakeCommit(blockID tmtypes.BlockID, height int64, round int32, valSet *tmtypes.ValidatorSet, privVals []tmtypes.PrivValidator, chainID string, now time.Time) (*tmtypes.Commit, error) {
	sigs := make([]tmtypes.CommitSig, len(valSet.Validators))
	for i := 0; i < len(valSet.Validators); i++ {
		sigs[i] = tmtypes.NewCommitSigAbsent()
	}

	for _, privVal := range privVals {
		pk, err := privVal.GetPubKey()
		if err != nil {
			return nil, err
		}
		addr := pk.Address()

		idx, _ := valSet.GetByAddress(addr)
		if idx < 0 {
			return nil, fmt.Errorf("validator with address %s not in validator set", addr)
		}

		vote := &tmtypes.Vote{
			ValidatorAddress: addr,
			ValidatorIndex:   idx,
			Height:           height,
			Round:            round,
			Type:             tmproto.PrecommitType,
			BlockID:          blockID,
			Timestamp:        now,
		}

		v := vote.ToProto()

		if err := privVal.SignVote(chainID, v); err != nil {
			return nil, err
		}

		sigs[idx] = tmtypes.CommitSig{
			BlockIDFlag:      tmtypes.BlockIDFlagCommit,
			ValidatorAddress: addr,
			Timestamp:        now,
			Signature:        v.Signature,
		}
	}

	return &tmtypes.Commit{Height: height, Round: round, BlockID: blockID, Signatures: sigs}, nil
}

/*
func updateLightClientHelper(t *testing.T, ctx sdk.Context) {
	blockData := tmproto.Data{
		Txs: [][]byte{ctx.TxBytes()},
	}
	dataBz, err := blockData.Marshal()
	require.NoError(t, err)

	blockHeader := ctx.BlockHeader()

	blockId := makeBlockIDRandom()

	valSet, privValidators := tmtypes.RandValidatorSet(1, 1)
	commit, err := MakeCommit(blockId, blockHeader.Height, 0, valSet, privValidators, ctx.ChainID(), time.Now())
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

	randomAndProofBz := append(random, proof...)

	_, _, err = api.SubmitBlockSignatures(headerBz, commitBz, dataBz, randomAndProofBz)
	require.NoError(t, err)
}*/

func makeBlockIDRandom() tmtypes.BlockID {
	var (
		blockHash   = make([]byte, sha256.Size)
		partSetHash = make([]byte, sha256.Size)
	)
	rand.Read(blockHash)
	rand.Read(partSetHash)
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
