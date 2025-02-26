package v2

import (
	"cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/neutron-org/neutron/v5/x/cron/types"
)

// MigrateStore performs in-place store migrations.
// The migration adds execution stage for schedules.
func MigrateStore(ctx sdk.Context, cdc codec.BinaryCodec, storeKey storetypes.StoreKey) error {
	return migrateSchedules(ctx, cdc, storeKey)
}

type migrationUpdate struct {
	key []byte
	val []byte
}

func migrateSchedules(ctx sdk.Context, cdc codec.BinaryCodec, storeKey storetypes.StoreKey) error {
	ctx.Logger().Info("Migrating cron Schedules...")

	store := prefix.NewStore(ctx.KVStore(storeKey), types.ScheduleKey)
	iterator := storetypes.KVStorePrefixIterator(store, []byte{})
	schedulesToUpdate := make([]migrationUpdate, 0)

	for ; iterator.Valid(); iterator.Next() {
		var schedule types.Schedule
		cdc.MustUnmarshal(iterator.Value(), &schedule)
		// Set execution in EndBlocker
		schedule.ExecutionStage = types.ExecutionStage_EXECUTION_STAGE_END_BLOCKER

		schedulesToUpdate = append(schedulesToUpdate, migrationUpdate{
			key: iterator.Key(),
			val: cdc.MustMarshal(&schedule),
		})
	}

	err := iterator.Close()
	if err != nil {
		return errors.Wrap(err, "iterator failed to close during migration")
	}

	// Store the updated Schedules
	for _, v := range schedulesToUpdate {
		store.Set(v.key, v.val)
	}

	ctx.Logger().Info("Finished migrating cron Schedules...")

	return nil
}
