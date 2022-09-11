package keeper

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
)

// BeginRedeeming creates a new redeem entry and enqueues it.
func (k Keeper) BeginRedeeming(ctx sdk.Context, accAddr sdk.AccAddress, amount sdk.Coins) (time.Time, error) {
	completionTime := ctx.BlockTime().Add(k.RedeemDur(ctx))

	entry, found := k.GetRedeemEntry(ctx, accAddr)
	if !found {
		entry = types.NewRedeemEntry(accAddr)
	}
	entry.AddOperation(ctx.BlockHeight(), completionTime, amount)

	if uint32(entry.OperationsLeft()) >= k.MaxRedeemEntries(ctx) {
		return time.Time{}, sdkErrors.Wrapf(types.ErrMaxRedeemEntries, "wait for some redeems to complete")
	}

	k.SetRedeemEntry(ctx, entry)
	k.InsertToRedeemQueue(ctx, completionTime, accAddr)

	return completionTime, nil
}

// EndRedeeming dequeues all mature redeem entries and sends collaterals to a requester from the module's Redeeming pool.
func (k Keeper) EndRedeeming(ctx sdk.Context) {
	curTime := ctx.BlockTime()

	matureAccAddrs := k.DequeueAllMatureFromRedeemQueue(ctx, curTime)
	for _, accAddr := range matureAccAddrs {
		entry, found := k.GetRedeemEntry(ctx, accAddr)
		if !found {
			// That could happen if a particular redeeming queue timeSlice has multiple entries for the same address
			continue
		}

		matureOps := entry.RemoveMatureOperations(curTime)
		for _, op := range matureOps {
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.RedeemingPoolName, accAddr, op.CollateralAmount); err != nil {
				panic(fmt.Errorf("sending collateral coins (%s) from module to account (%s): %w", op.CollateralAmount, accAddr, err))
			}

			ctx.EventManager().EmitEvent(
				types.NewRedeemDoneEvent(accAddr, op.CollateralAmount, op.CompletionTime),
			)
		}

		if entry.OperationsLeft() == 0 {
			k.RemoveRedeemEntry(ctx, accAddr)
			continue
		}

		k.SetRedeemEntry(ctx, entry)
	}
}

// SetRedeemEntry sets the types.RedeemEntry object.
func (k Keeper) SetRedeemEntry(ctx sdk.Context, entry types.RedeemEntry) {
	store := ctx.KVStore(k.storeKey)

	accAddr, err := sdk.AccAddressFromBech32(entry.Address)
	if err != nil {
		panic(fmt.Errorf("parsing RedeemEntry.Address (%s): %v", entry.Address, err))
	}
	key := types.GetRedeemEntryKey(accAddr)

	bz := k.cdc.MustMarshal(&entry)
	store.Set(key, bz)
}

// GetRedeemEntry returns the types.RedeemEntry object if found.
func (k Keeper) GetRedeemEntry(ctx sdk.Context, accAddr sdk.AccAddress) (types.RedeemEntry, bool) {
	store := ctx.KVStore(k.storeKey)
	key := types.GetRedeemEntryKey(accAddr)

	bz := store.Get(key)
	if bz == nil {
		return types.RedeemEntry{}, false
	}

	var entry types.RedeemEntry
	k.cdc.MustUnmarshal(bz, &entry)

	return entry, true
}

// RemoveRedeemEntry removes the types.RedeemEntry object.
func (k Keeper) RemoveRedeemEntry(ctx sdk.Context, accAddr sdk.AccAddress) {
	store := ctx.KVStore(k.storeKey)
	key := types.GetRedeemEntryKey(accAddr)

	store.Delete(key)
}

// SetRedeemQueueTimeSlice sets redeeming queue timeSlice at a given timestamp key.
func (k Keeper) SetRedeemQueueTimeSlice(ctx sdk.Context, timestamp time.Time, data types.RedeemingQueueData) {
	store := ctx.KVStore(k.storeKey)
	key := types.GetRedeemingQueueKey(timestamp)

	bz := k.cdc.MustMarshal(&data)
	store.Set(key, bz)
}

// GetRedeemQueueTimeSlice returns redeeming queue timeSlice at a given timestamp key.
func (k Keeper) GetRedeemQueueTimeSlice(ctx sdk.Context, timestamp time.Time) types.RedeemingQueueData {
	store := ctx.KVStore(k.storeKey)
	key := types.GetRedeemingQueueKey(timestamp)

	bz := store.Get(key)
	if bz == nil {
		return types.RedeemingQueueData{}
	}

	var data types.RedeemingQueueData
	k.cdc.MustUnmarshal(bz, &data)

	return data
}

// InsertToRedeemQueue adds redeem entry to the redeeming queue timeSlice.
func (k Keeper) InsertToRedeemQueue(ctx sdk.Context, completionTime time.Time, accAddr sdk.AccAddress) {
	timeSlice := k.GetRedeemQueueTimeSlice(ctx, completionTime)
	timeSlice.Addresses = append(timeSlice.Addresses, accAddr.String())

	k.SetRedeemQueueTimeSlice(ctx, completionTime, timeSlice)
}

// DequeueAllMatureFromRedeemQueue returns all redeeming queue entries whose timestamp key is LTE a given timestamp
// removing them from the queue.
func (k Keeper) DequeueAllMatureFromRedeemQueue(ctx sdk.Context, endTime time.Time) (matureAccAddrs []sdk.AccAddress) {
	store := ctx.KVStore(k.storeKey)

	iterator := store.Iterator(types.RedeemingQueueKey, sdk.InclusiveEndBytes(types.GetRedeemingQueueKey(endTime)))
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var data types.RedeemingQueueData
		k.cdc.MustUnmarshal(iterator.Value(), &data)

		accAddrs := make([]sdk.AccAddress, 0, len(data.Addresses))
		for _, addrRaw := range data.Addresses {
			accAddr, err := sdk.AccAddressFromBech32(addrRaw)
			if err != nil {
				panic(fmt.Errorf("parsing redeeming queue address (%s): %w", addrRaw, err))
			}
			accAddrs = append(accAddrs, accAddr)
		}
		matureAccAddrs = append(matureAccAddrs, accAddrs...)

		store.Delete(iterator.Key())
	}

	return
}

// IterateRedeemEntries iterates over all types.RedeemEntry objects.
func (k Keeper) IterateRedeemEntries(ctx sdk.Context, fn func(entry types.RedeemEntry) (stop bool)) {
	store := ctx.KVStore(k.storeKey)

	iterator := sdk.KVStoreReversePrefixIterator(store, types.RedeemEntryKey)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var entry types.RedeemEntry
		k.cdc.MustUnmarshal(iterator.Value(), &entry)

		if fn(entry) {
			break
		}
	}
}
