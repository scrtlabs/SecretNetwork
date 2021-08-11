package keeper

import (
	"encoding/binary"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
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

	wasmtypes "github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	"github.com/enigmampc/SecretNetwork/x/registration"
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
	encodingConfig := MakeEncodingConfig()
	paramsKeeper := paramskeeper.NewKeeper(encodingConfig.Marshaler, encodingConfig.Amino, keyParams, tkeyParams)
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(crisistypes.ModuleName)

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
		encodingConfig.Marshaler,
		keyBank,
		authKeeper,
		bankSubsp,
		blockedAddrs,
	)

	//bankParams = bankParams.SetSendEnabledParam(sdk.DefaultBondDenom, true)
	bankKeeper.SetParams(ctx, banktypes.DefaultParams())

	stakingSubsp, _ := paramsKeeper.GetSubspace(stakingtypes.ModuleName)
	stakingKeeper := stakingkeeper.NewKeeper(encodingConfig.Marshaler, keyStaking, authKeeper, bankKeeper, stakingSubsp)
	stakingKeeper.SetParams(ctx, TestingStakeParams)

	//mintSubsp, _ := paramsKeeper.GetSubspace(minttypes.ModuleName)

	//mintKeeper := mintkeeper.NewKeeper(encodingConfig.Marshaler,
	//	keyBank,
	//	mintSubsp,
	//	stakingKeeper,
	//	authKeeper,
	//	bankKeeper,
	//	authtypes.FeeCollectorName,
	//	)
	//
	//bankkeeper.SetSupply(ctx, banktypes.NewSupply(sdk.NewCoins((sdk.NewInt64Coin("stake", 1)))))

	distSubsp, _ := paramsKeeper.GetSubspace(distrtypes.ModuleName)
	distKeeper := distrkeeper.NewKeeper(
		encodingConfig.Marshaler,
		keyDistro,
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

	//err = bankKeeper.SendCoinsFromModuleToAccount(ctx, faucetAccountName, distrAcc.GetAddress(), totalSupply)
	//require.NoError(t, err)

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
		//AddRoute(wasmtypes.RouterKey, NewWasmProposalHandler(keeper, wasmtypes.EnableAllProposals))

	govKeeper := govkeeper.NewKeeper(
		encodingConfig.Marshaler, keyGov, paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable()), authKeeper, bankKeeper, stakingKeeper, govRouter,
	)

	// bank := bankKeeper.
	//bk := bank.Keeper(bankKeeper)

	mintSubsp, _ := paramsKeeper.GetSubspace(minttypes.ModuleName)
	mintKeeper := mintkeeper.NewKeeper(encodingConfig.Marshaler, mintStore, mintSubsp, stakingKeeper, authKeeper, bankKeeper, authtypes.FeeCollectorName)
	mintKeeper.SetMinter(ctx, minttypes.DefaultInitialMinter())

	//keeper := NewKeeper(cdc, keyContract, accountKeeper, &bk, &govKeeper, &distKeeper, &mintKeeper, &stakingKeeper, router, tempDir, wasmConfig, supportedFeatures, encoders, queriers)
	//// add wasm handler so we can loop-back (contracts calling contracts)
	//router.AddRoute(wasmtypes.RouterKey, TestHandler(keeper))

	govKeeper.SetProposalID(ctx, govtypes.DefaultStartingProposalID)
	govKeeper.SetDepositParams(ctx, govtypes.DefaultDepositParams())
	govKeeper.SetVotingParams(ctx, govtypes.DefaultVotingParams())
	govKeeper.SetTallyParams(ctx, govtypes.DefaultTallyParams())
	gh := gov.NewHandler(govKeeper)
	router.AddRoute(sdk.NewRoute(govtypes.RouterKey, gh))

	// Load default wasm config
	wasmConfig := wasmtypes.DefaultWasmConfig()

	// todo: new grpc routing
	//serviceRouter := baseapp.NewMsgServiceRouter()

	//serviceRouter.SetInterfaceRegistry(encodingConfig.InterfaceRegistry)
	//bankMsgServer := bankkeeper.NewMsgServerImpl(bankKeeper)
	//stakingMsgServer := stakingkeeper.NewMsgServerImpl(stakingKeeper)
	//distrMsgServer := distrkeeper.NewMsgServerImpl(distKeeper)
	//wasmMsgServer := NewMsgServerImpl(keeper)

	//banktypes.RegisterMsgServer(serviceRouter, bankMsgServer)
	//stakingtypes.RegisterMsgServer(serviceRouter, stakingMsgServer)
	//distrtypes.RegisterMsgServer(serviceRouter, distrMsgServer)

	keeper := NewKeeper(
		encodingConfig.Marshaler,
		*encodingConfig.Amino,
		keyContract,
		authKeeper,
		bankKeeper,
		govKeeper,
		distKeeper,
		mintKeeper,
		stakingKeeper,
		// serviceRouter,
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

	initMsg := wasmtypes.MsgInstantiateContract{
		Sender: creator,
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   encMsg,
		InitFunds: funds,
	}
	tx := NewTestTx(&initMsg, creatorAcc, privKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	return ctx.WithTxBytes(txBytes)
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
	tx := NewTestTx(&executeMsg, creatorAcc, privKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	return ctx.WithTxBytes(txBytes)
}

func NewTestTx(msg sdk.Msg, creatorAcc authtypes.AccountI, privKey crypto.PrivKey) *sdktx.Tx {
	return NewTestTxMultiple([]sdk.Msg{msg}, []authtypes.AccountI{creatorAcc}, []crypto.PrivKey{privKey})
}

func NewTestTxMultiple(msgs []sdk.Msg, creatorAccs []authtypes.AccountI, privKeys []crypto.PrivKey) *sdktx.Tx {
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
			ChainID:       "",
			AccountNumber: creatorAcc.GetAccountNumber(),
			Sequence:      creatorAcc.GetSequence(),
		}
		bytesToSign, err := signModeHandler.GetSignBytes(sdksigning.SignMode_SIGN_MODE_DIRECT, signerData, builder.GetTx())

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

	tx, ok := builder.(protoTxProvider)
	if !ok {
		panic("failed to unwrap tx builder to protobuf tx")
	}
	return tx.GetProtoTx()
}

func CreateFakeFundedAccount(ctx sdk.Context, am authkeeper.AccountKeeper, bk bankkeeper.Keeper, coins sdk.Coins) (sdk.AccAddress, crypto.PrivKey) {
	priv, pub, addr := keyPubAddr()
	baseAcct := authtypes.NewBaseAccountWithAddress(addr)
	_ = baseAcct.SetPubKey(pub)
	am.SetAccount(ctx, baseAcct)

	fundAccounts(ctx, am, bk, addr, coins)
	return addr, priv
}

const faucetAccountName = "faucet"

func fundAccounts(ctx sdk.Context, am authkeeper.AccountKeeper, bk bankkeeper.Keeper, addr sdk.AccAddress, coins sdk.Coins) {
	baseAcct := am.GetAccount(ctx, addr)
	if err := bk.MintCoins(ctx, faucetAccountName, coins); err != nil {
		panic(err)
	}

	_ = bk.SendCoinsFromModuleToAccount(ctx, faucetAccountName, addr, coins)

	am.SetAccount(ctx, baseAcct)
}

var keyCounter uint64 = 0

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

func txBuilderToProtoTx(txBuilder client.TxBuilder) (*tx.Tx, error) { // nolint
	protoProvider, ok := txBuilder.(protoTxProvider)
	if !ok {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidType, "expected proto tx builder, got %T", txBuilder)
	}

	return protoProvider.GetProtoTx(), nil
}
