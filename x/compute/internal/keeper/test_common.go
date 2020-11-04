package keeper

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/codec/types"
	params2 "github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authlegacy "github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	paramproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"
	"github.com/enigmampc/SecretNetwork/x/registration"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"io/ioutil"
	"os"
	"testing"
	"time"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/tendermint/tendermint/crypto"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/staking"

	wasmtypes "github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

const flagLRUCacheSize = "lru_size"
const flagQueryGasLimit = "query_gas_limit"

var ModuleBasics = module.NewBasicManager(
	auth.AppModuleBasic{},
	bank.AppModuleBasic{},
	capability.AppModuleBasic{},
	staking.AppModuleBasic{},
	mint.AppModuleBasic{},
	distribution.AppModuleBasic{},
	gov.NewAppModuleBasic(
		paramsclient.ProposalHandler, distrclient.ProposalHandler, upgradeclient.ProposalHandler,
	),
	params.AppModuleBasic{},
	crisis.AppModuleBasic{},
	slashing.AppModuleBasic{},
	//ibc.AppModuleBasic{},
	upgrade.AppModuleBasic{},
	evidence.AppModuleBasic{},
	//transfer.AppModuleBasic{},
	registration.AppModuleBasic{},
)

func MakeTestCodec() codec.Marshaler {
	return MakeEncodingConfig().Marshaler
}
func MakeEncodingConfig() params2.EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txCfg := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	std.RegisterInterfaces(interfaceRegistry)
	std.RegisterLegacyAminoCodec(amino)

	ModuleBasics.RegisterLegacyAminoCodec(amino)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	return params2.EncodingConfig{
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
	BondDenom:         "stake",
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

// encoders can be nil to accept the defaults, or set it to override some of the message handlers (like default)
func CreateTestInput(t *testing.T, isCheckTx bool, supportedFeatures string, encoders *MessageEncoders, queriers *QueryPlugins) (sdk.Context, TestKeepers) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	keyContract := sdk.NewKVStoreKey(wasmtypes.StoreKey)
	keyAcc := sdk.NewKVStoreKey(authtypes.StoreKey)
	keyStaking := sdk.NewKVStoreKey(stakingtypes.StoreKey)
	keyDistro := sdk.NewKVStoreKey(distrtypes.StoreKey)
	mintStore := sdk.NewKVStoreKey(minttypes.StoreKey)
	keyParams := sdk.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := sdk.NewTransientStoreKey(paramstypes.TStoreKey)
	keyGov := sdk.NewKVStoreKey(govtypes.StoreKey)
	keyBank := sdk.NewKVStoreKey(banktypes.StoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyContract, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyAcc, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyStaking, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(mintStore, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyDistro, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, sdk.StoreTypeTransient, db)
	ms.MountStoreWithDB(keyGov, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyBank, sdk.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())

	ctx := sdk.NewContext(ms, tmproto.Header{
		Height: 1234567,
		Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
	}, isCheckTx, log.NewNopLogger())
	encodingConfig := MakeEncodingConfig
	paramsKeeper := paramskeeper.NewKeeper(encodingConfig().Marshaler, encodingConfig().Amino, keyParams, tkeyParams)
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(crisistypes.ModuleName)

	// this is also used to initialize module accounts (so nil is meaningful here)
	maccPerms := map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
	}
	authSubsp, _ := paramsKeeper.GetSubspace(authtypes.ModuleName)
	authKeeper := authkeeper.NewAccountKeeper(
		encodingConfig().Marshaler,
		keyAcc, // target store
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
		encodingConfig().Marshaler,
		keyBank,
		authKeeper,
		bankSubsp,
		blockedAddrs,
	)
	bankParams := banktypes.DefaultParams()
	bankParams = bankParams.SetSendEnabledParam("stake", true)
	bankKeeper.SetParams(ctx, bankParams)

	stakingSubsp, _ := paramsKeeper.GetSubspace(stakingtypes.ModuleName)
	stakingKeeper := stakingkeeper.NewKeeper(encodingConfig().Marshaler, keyStaking, authKeeper, bankKeeper, stakingSubsp)
	stakingKeeper.SetParams(ctx, TestingStakeParams)

	distSubsp, _ := paramsKeeper.GetSubspace(distrtypes.ModuleName)
	distKeeper := distrkeeper.NewKeeper(encodingConfig().Marshaler, keyDistro, distSubsp, authKeeper, bankKeeper, stakingKeeper, authtypes.FeeCollectorName, nil)
	distKeeper.SetParams(ctx, distrtypes.DefaultParams())
	stakingKeeper.SetHooks(distKeeper.Hooks())

	// set genesis items required for distribution
	distKeeper.SetFeePool(ctx, distrtypes.InitialFeePool())

	// set some funds ot pay out validatores, based on code from:
	// https://github.com/cosmos/cosmos-sdk/blob/fea231556aee4d549d7551a6190389c4328194eb/x/distribution/keeper/keeper_test.go#L50-L57
	distrAcc := distKeeper.GetDistributionAccount(ctx)
	err = bankKeeper.SetBalances(ctx, distrAcc.GetAddress(), sdk.NewCoins(
		sdk.NewCoin("stake", sdk.NewInt(2000000)),
	))
	require.NoError(t, err)
	authKeeper.SetModuleAccount(ctx, distrAcc)

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
		//AddRoute(wasmtypes.RouterKey, NewWasmProposalHandler(keeper, wasmtypes.EnableAllProposals))

	govKeeper := govkeeper.NewKeeper(
		encodingConfig().Marshaler, keyGov, paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable()), authKeeper, bankKeeper, stakingKeeper, govRouter,
	)

	govKeeper.SetProposalID(ctx, govtypes.DefaultStartingProposalID)
	govKeeper.SetDepositParams(ctx, govtypes.DefaultDepositParams())
	govKeeper.SetVotingParams(ctx, govtypes.DefaultVotingParams())
	govKeeper.SetTallyParams(ctx, govtypes.DefaultTallyParams())
	// bank := bankKeeper.
	//bk := bank.Keeper(bankKeeper)

	mintSubsp, _ := paramsKeeper.GetSubspace(minttypes.ModuleName)
	mintKeeper := mintkeeper.NewKeeper(encodingConfig().Marshaler, mintStore, mintSubsp, stakingKeeper, authKeeper, bankKeeper, authtypes.FeeCollectorName)
	mintKeeper.SetMinter(ctx, minttypes.DefaultInitialMinter())

	//keeper := NewKeeper(cdc, keyContract, accountKeeper, &bk, &govKeeper, &distKeeper, &mintKeeper, &stakingKeeper, router, tempDir, wasmConfig, supportedFeatures, encoders, queriers)
	//// add wasm handler so we can loop-back (contracts calling contracts)
	//router.AddRoute(wasmtypes.RouterKey, TestHandler(keeper))

	// Load default wasm config
	wasmConfig := wasmtypes.DefaultWasmConfig()

	keeper := NewKeeper(
		encodingConfig().Marshaler,
		*encodingConfig().Amino,
		keyContract,
		authKeeper,
		bankKeeper,
		govKeeper,
		distKeeper,
		mintKeeper,
		stakingKeeper,
		router,
		tempDir,
		wasmConfig,
		supportedFeatures,
		encoders,
		queriers,
	)
	//keeper.setParams(ctx, wasmtypes.DefaultParams())
	// add wasm handler so we can loop-back (contracts calling contracts)
	router.AddRoute(sdk.NewRoute(wasmtypes.RouterKey, TestHandler(keeper)))

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
	contractAddr, err := k.Instantiate(ctx, msg.CodeID, msg.Sender /* msg.Admin, */, msg.InitMsg, msg.Label, msg.InitFunds, msg.CallbackSig)
	if err != nil {
		return nil, err
	}

	return &sdk.Result{
		Data:   contractAddr,
		Events: ctx.EventManager().Events().ToABCIEvents(),
	}, nil
}

func handleExecute(ctx sdk.Context, k Keeper, msg *wasmtypes.MsgExecuteContract) (*sdk.Result, error) {
	res, err := k.Execute(ctx, msg.Contract, msg.Sender, msg.Msg, msg.SentFunds, msg.CallbackSig)
	if err != nil {
		return nil, err
	}

	res.Events = ctx.EventManager().Events().ToABCIEvents()
	return res, nil
}

func PrepareInitSignedTx(t *testing.T, keeper Keeper, ctx sdk.Context, creator sdk.AccAddress, privKey crypto.PrivKey, encMsg []byte, codeID uint64, funds sdk.Coins) sdk.Context {
	creatorAcc, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, creator)
	require.NoError(t, err)

	tx := NewTestTx(ctx, []sdk.Msg{&wasmtypes.MsgInstantiateContract{
		Sender: creator,
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   encMsg,
		InitFunds: funds,
	}}, []crypto.PrivKey{privKey}, []uint64{creatorAcc.GetAccountNumber()}, []uint64{creatorAcc.GetSequence() - 1}, 0, authlegacy.StdFee{
		Amount: nil,
		Gas:    0,
	})

	txBytes, err := keeper.legacyAmino.MarshalBinaryLengthPrefixed(tx)
	require.NoError(t, err)

	return ctx.WithTxBytes(txBytes)
}

func PrepareExecSignedTx(t *testing.T, keeper Keeper, ctx sdk.Context, sender sdk.AccAddress, privKey crypto.PrivKey, encMsg []byte, contract sdk.AccAddress, funds sdk.Coins) sdk.Context {
	creatorAcc, err := ante.GetSignerAcc(ctx, keeper.accountKeeper, sender)
	require.NoError(t, err)

	tx := NewTestTx(ctx, []sdk.Msg{&wasmtypes.MsgExecuteContract{
		Sender:    sender,
		Contract:  contract,
		Msg:       encMsg,
		SentFunds: funds,
	}}, []crypto.PrivKey{privKey}, []uint64{creatorAcc.GetAccountNumber()}, []uint64{creatorAcc.GetSequence() - 1}, 0, authlegacy.StdFee{
		Amount: nil,
		Gas:    0,
	})

	txBytes, err := keeper.legacyAmino.MarshalBinaryLengthPrefixed(tx)
	require.NoError(t, err)

	return ctx.WithTxBytes(txBytes)
}

func NewTestTx(ctx sdk.Context, msgs []sdk.Msg, privs []crypto.PrivKey, accNums []uint64, seqs []uint64, timeout uint64, fee authlegacy.StdFee) sdk.Tx {
	sigs := make([]authlegacy.StdSignature, len(privs))
	for i, priv := range privs {
		signBytes := authlegacy.StdSignBytes(ctx.ChainID(), accNums[i], seqs[i], timeout, fee, msgs, "")

		sig, err := priv.Sign(signBytes)
		if err != nil {
			panic(err)
		}

		sigs[i] = authlegacy.StdSignature{PubKey: priv.PubKey(), Signature: sig}
	}

	tx := authlegacy.NewStdTx(msgs, fee, sigs, "")
	return tx
}
