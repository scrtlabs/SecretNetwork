package keeper

import (
	"sync"
	"testing"

	"cosmossdk.io/log"
	metrics2 "cosmossdk.io/store/metrics"
	db2 "github.com/cosmos/cosmos-db"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"cosmossdk.io/store"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/scrtlabs/SecretNetwork/x/cron/keeper"
	"github.com/scrtlabs/SecretNetwork/x/cron/types"
)

var configOnce sync.Once

func CronKeeper(t testing.TB, accountKeeper types.AccountKeeper) (*keeper.Keeper, sdk.Context) {
	configOnce.Do(func() {
		config := sdk.GetConfig()
		config.SetBech32PrefixForAccount("secret", "secretpub")
		config.SetBech32PrefixForValidator("secretvaloper", "secretvaloperpub")
		config.SetBech32PrefixForConsensusNode("secretvalcons", "secretvalconspub")
		config.Seal()
	})

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	memStoreKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)

	db := db2.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics2.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(memStoreKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	k := keeper.NewKeeper(
		cdc,
		storeKey,
		memStoreKey,
		accountKeeper,
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)

	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())

	// Initialize params
	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)

	return k, ctx
}
