package keeper

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	"github.com/enigmampc/cosmos-sdk/baseapp"
	"github.com/enigmampc/cosmos-sdk/codec"
	"github.com/enigmampc/cosmos-sdk/store"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	sdkerrors "github.com/enigmampc/cosmos-sdk/types/errors"
	"github.com/enigmampc/cosmos-sdk/x/auth"
	"github.com/enigmampc/cosmos-sdk/x/bank"
	"github.com/enigmampc/cosmos-sdk/x/params"
	"github.com/enigmampc/cosmos-sdk/x/staking"
	"github.com/enigmampc/cosmos-sdk/x/supply"

	wasmTypes "github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

const flagLRUCacheSize = "lru_size"
const flagQueryGasLimit = "query_gas_limit"

func MakeTestCodec() *codec.Codec {
	var cdc = codec.New()

	// Register AppAccount
	// cdc.RegisterInterface((*authexported.Account)(nil), nil)
	// cdc.RegisterConcrete(&auth.BaseAccount{}, "test/wasm/BaseAccount", nil)
	auth.AppModuleBasic{}.RegisterCodec(cdc)
	bank.AppModuleBasic{}.RegisterCodec(cdc)
	supply.AppModuleBasic{}.RegisterCodec(cdc)
	staking.AppModuleBasic{}.RegisterCodec(cdc)
	distribution.AppModuleBasic{}.RegisterCodec(cdc)
	gov.RegisterCodec(cdc)
	wasmtypes.RegisterCodec(cdc)
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	params.RegisterCodec(cdc)
	return cdc
}

var TestingStakeParams = staking.Params{
	UnbondingTime:     100,
	MaxValidators:     10,
	MaxEntries:        10,
	HistoricalEntries: 10,
	BondDenom:         "stake",
}

type TestKeepers struct {
	AccountKeeper auth.AccountKeeper
	StakingKeeper staking.Keeper
	WasmKeeper    Keeper
	DistKeeper    distribution.Keeper
	SupplyKeeper  supply.Keeper
	GovKeeper     gov.Keeper
	BankKeeper    bank.Keeper
}

// encoders can be nil to accept the defaults, or set it to override some of the message handlers (like default)
func CreateTestInput(t *testing.T, isCheckTx bool, tempDir string, supportedFeatures string, encoders *MessageEncoders, queriers *QueryPlugins) (sdk.Context, TestKeepers) {
	keyContract := sdk.NewKVStoreKey(wasmtypes.StoreKey)
	keyAcc := sdk.NewKVStoreKey(auth.StoreKey)
	keyStaking := sdk.NewKVStoreKey(staking.StoreKey)
	keySupply := sdk.NewKVStoreKey(supply.StoreKey)
	keyDistro := sdk.NewKVStoreKey(distribution.StoreKey)
	keyParams := sdk.NewKVStoreKey(params.StoreKey)
	tkeyParams := sdk.NewTransientStoreKey(params.TStoreKey)
	keyGov := sdk.NewKVStoreKey(govtypes.StoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyContract, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyAcc, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyStaking, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keySupply, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyDistro, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, sdk.StoreTypeTransient, db)
	ms.MountStoreWithDB(keyGov, sdk.StoreTypeIAVL, db)
	err := ms.LoadLatestVersion()
	require.Nil(t, err)

	ctx := sdk.NewContext(ms, abci.Header{
		Height: 1234567,
		Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
	}, isCheckTx, log.NewNopLogger())
	cdc := MakeTestCodec()

	paramsKeeper := params.NewKeeper(cdc, keyParams, tkeyParams)

	accountKeeper := auth.NewAccountKeeper(
		cdc,    // amino codec
		keyAcc, // target store
		paramsKeeper.Subspace(auth.DefaultParamspace),
		auth.ProtoBaseAccount, // prototype
	)

	// this is also used to initialize module accounts (so nil is meaningful here)
	maccPerms := map[string][]string{
		auth.FeeCollectorName:   nil,
		distribution.ModuleName: nil,
		//mint.ModuleName:           {supply.Minter},
		staking.BondedPoolName:    {supply.Burner, supply.Staking},
		staking.NotBondedPoolName: {supply.Burner, supply.Staking},
		gov.ModuleName:            {supply.Burner},
	}
	blockedAddr := make(map[string]bool, len(maccPerms))
	for acc := range maccPerms {
		blockedAddr[supply.NewModuleAddress(acc).String()] = true
	}
	bankKeeper := bank.NewBaseKeeper(
		accountKeeper,
		paramsKeeper.Subspace(bank.DefaultParamspace),
		blockedAddr,
	)
	bankKeeper.SetSendEnabled(ctx, true)

	supplyKeeper := supply.NewKeeper(cdc, keySupply, accountKeeper, bankKeeper, maccPerms)
	stakingKeeper := staking.NewKeeper(cdc, keyStaking, supplyKeeper, paramsKeeper.Subspace(staking.DefaultParamspace))
	stakingKeeper.SetParams(ctx, TestingStakeParams)

	distKeeper := distribution.NewKeeper(cdc, keyDistro, paramsKeeper.Subspace(distribution.DefaultParamspace), stakingKeeper, supplyKeeper, auth.FeeCollectorName, nil)
	distKeeper.SetParams(ctx, distribution.DefaultParams())
	stakingKeeper.SetHooks(distKeeper.Hooks())

	// set genesis items required for distribution
	distKeeper.SetFeePool(ctx, distribution.InitialFeePool())

	// total supply to track this
	totalSupply := sdk.NewCoins(sdk.NewInt64Coin("stake", 100000000))
	supplyKeeper.SetSupply(ctx, supply.NewSupply(totalSupply))

	// set up initial accounts
	for name, perms := range maccPerms {
		mod := supply.NewEmptyModuleAccount(name, perms...)
		if name == staking.NotBondedPoolName {
			err = mod.SetCoins(totalSupply)
			require.NoError(t, err)
		} else if name == distribution.ModuleName {
			// some big pot to pay out
			err = mod.SetCoins(sdk.NewCoins(sdk.NewInt64Coin("stake", 500000)))
			require.NoError(t, err)
		}
		supplyKeeper.SetModuleAccount(ctx, mod)
	}

	stakeAddr := supply.NewModuleAddress(staking.BondedPoolName)
	moduleAcct := accountKeeper.GetAccount(ctx, stakeAddr)
	require.NotNil(t, moduleAcct)

	router := baseapp.NewRouter()
	bh := bank.NewHandler(bankKeeper)
	router.AddRoute(bank.RouterKey, bh)
	sh := staking.NewHandler(stakingKeeper)
	router.AddRoute(staking.RouterKey, sh)
	dh := distribution.NewHandler(distKeeper)
	router.AddRoute(distribution.RouterKey, dh)

	// Load default wasm config
	wasmConfig := wasmtypes.DefaultWasmConfig()
	keeper := NewKeeper(cdc, keyContract, paramsKeeper.Subspace(wasmtypes.DefaultParamspace),
		accountKeeper, bankKeeper, stakingKeeper, router, tempDir, wasmConfig,
		supportedFeatures, encoders, queriers,
	)
	keeper.setParams(ctx, wasmtypes.DefaultParams())
	// add wasm handler so we can loop-back (contracts calling contracts)
	router.AddRoute(wasmtypes.RouterKey, TestHandler(keeper))

	govRouter := gov.NewRouter().
		AddRoute(params.RouterKey, params.NewParamChangeProposalHandler(paramsKeeper)).
		AddRoute(govtypes.RouterKey, govtypes.ProposalHandler).
		AddRoute(wasmtypes.RouterKey, NewWasmProposalHandler(keeper, wasmtypes.EnableAllProposals))

	govKeeper := gov.NewKeeper(
		cdc, keyGov, paramsKeeper.Subspace(govtypes.DefaultParamspace).WithKeyTable(gov.ParamKeyTable()), supplyKeeper, stakingKeeper, govRouter,
	)

	govKeeper.SetProposalID(ctx, govtypes.DefaultStartingProposalID)
	govKeeper.SetDepositParams(ctx, govtypes.DefaultDepositParams())
	govKeeper.SetVotingParams(ctx, govtypes.DefaultVotingParams())
	govKeeper.SetTallyParams(ctx, govtypes.DefaultTallyParams())

	keepers := TestKeepers{
		AccountKeeper: accountKeeper,
		SupplyKeeper:  supplyKeeper,
		StakingKeeper: stakingKeeper,
		DistKeeper:    distKeeper,
		WasmKeeper:    keeper,
		GovKeeper:     govKeeper,
		BankKeeper:    bankKeeper,
	}
	return ctx, keepers
}

// TestHandler returns a wasm handler for tests (to avoid circular imports)
func TestHandler(k Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		case wasmtypes.MsgInstantiateContract:
			return handleInstantiate(ctx, k, &msg)
		case *wasmtypes.MsgInstantiateContract:
			return handleInstantiate(ctx, k, msg)

		case wasmtypes.MsgExecuteContract:
			return handleExecute(ctx, k, &msg)
		case *wasmtypes.MsgExecuteContract:
			return handleExecute(ctx, k, msg)

		default:
			errMsg := fmt.Sprintf("unrecognized wasm message type: %T", msg)
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, errMsg)
		}
	}
}

func handleInstantiate(ctx sdk.Context, k Keeper, msg *wasmtypes.MsgInstantiateContract) (*sdk.Result, error) {
	contractAddr, err := k.Instantiate(ctx, msg.CodeID, msg.Sender, msg.Admin, msg.InitMsg, msg.Label, msg.InitFunds)
	if err != nil {
		return nil, err
	}

	return &sdk.Result{
		Data:   contractAddr,
		Events: ctx.EventManager().Events(),
	}, nil
}

func handleExecute(ctx sdk.Context, k Keeper, msg *wasmtypes.MsgExecuteContract) (*sdk.Result, error) {
	res, err := k.Execute(ctx, msg.Contract, msg.Sender, msg.Msg, msg.SentFunds)
	if err != nil {
		return nil, err
	}

	res.Events = ctx.EventManager().Events()
	return res, nil
}
