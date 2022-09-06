package v2

// example migration

// import (
// 	"github.com/cosmos/cosmos-sdk/codec"
// 	storetypes "github.com/cosmos/cosmos-sdk/store/types"
// 	sdk "github.com/cosmos/cosmos-sdk/types"
// )

// // migrateSomething migrates the supply to be stored by denom key instead in a
// // single blob.
// // ref: https://github.com/cosmos/cosmos-sdk/issues/7092
// func migrateSomething(store sdk.KVStore, cdc codec.BinaryCodec) error {
// 	return nil
// }

// // MigrateStore performs in-place store migrations from v1 to v2. The
// // migration includes:
// func MigrateStore(ctx sdk.Context, storeKey storetypes.StoreKey, cdc codec.BinaryCodec) error {
// 	store := ctx.KVStore(storeKey)

// 	if err := migrateSomething(store, cdc); err != nil {
// 		return err
// 	}

// 	return nil
// }
