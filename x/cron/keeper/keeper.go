package keeper

import (
	"fmt"
	"strconv"
	"time"

	"cosmossdk.io/log"
	"github.com/hashicorp/go-metrics"

	"github.com/cosmos/cosmos-sdk/telemetry"

	"cosmossdk.io/store/prefix"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/neutron-org/neutron/v5/x/cron/types"
)

var (
	LabelExecuteReadySchedules   = "execute_ready_schedules"
	LabelScheduleCount           = "schedule_count"
	LabelScheduleExecutionsCount = "schedule_executions_count"

	MetricLabelSuccess      = "success"
	MetricLabelScheduleName = "schedule_name"
)

type (
	Keeper struct {
		cdc           codec.BinaryCodec
		storeKey      storetypes.StoreKey
		memKey        storetypes.StoreKey
		accountKeeper types.AccountKeeper
		WasmMsgServer types.WasmMsgServer
		authority     string
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey,
	memKey storetypes.StoreKey,
	accountKeeper types.AccountKeeper,
	authority string,
) *Keeper {
	return &Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		memKey:        memKey,
		accountKeeper: accountKeeper,
		authority:     authority,
	}
}

func (k Keeper) GetAuthority() string {
	return k.authority
}

func (k *Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// ExecuteReadySchedules gets all schedules that are due for execution (with limit that is equal to Params.Limit)
// and executes messages in each one
func (k *Keeper) ExecuteReadySchedules(ctx sdk.Context, executionStage types.ExecutionStage) {
	telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), LabelExecuteReadySchedules)
	schedules := k.getSchedulesReadyForExecution(ctx, executionStage)

	for _, schedule := range schedules {
		err := k.executeSchedule(ctx, schedule)
		recordExecutedSchedule(err, schedule)
	}
}

// AddSchedule adds a new schedule to be executed every certain number of blocks, specified in the `period`.
// First schedule execution is supposed to be on `now + period` block.
func (k *Keeper) AddSchedule(
	ctx sdk.Context,
	name string,
	period uint64,
	msgs []types.MsgExecuteContract,
	executionStage types.ExecutionStage,
) error {
	if k.scheduleExists(ctx, name) {
		return fmt.Errorf("schedule already exists with name=%v", name)
	}

	schedule := types.Schedule{
		Name:   name,
		Period: period,
		Msgs:   msgs,
		// let's execute newly added schedule on `now + period` block
		LastExecuteHeight: uint64(ctx.BlockHeight()), //nolint:gosec
		ExecutionStage:    executionStage,
	}

	k.storeSchedule(ctx, schedule)
	k.changeTotalCount(ctx, 1)

	return nil
}

// RemoveSchedule removes schedule with a given `name`
func (k *Keeper) RemoveSchedule(ctx sdk.Context, name string) {
	if !k.scheduleExists(ctx, name) {
		return
	}

	k.changeTotalCount(ctx, -1)
	k.removeSchedule(ctx, name)
}

// GetSchedule returns schedule with a given `name`
func (k *Keeper) GetSchedule(ctx sdk.Context, name string) (*types.Schedule, bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)
	bzSchedule := store.Get(types.GetScheduleKey(name))
	if bzSchedule == nil {
		return nil, false
	}

	var schedule types.Schedule
	k.cdc.MustUnmarshal(bzSchedule, &schedule)
	return &schedule, true
}

// GetAllSchedules returns all schedules
func (k *Keeper) GetAllSchedules(ctx sdk.Context) []types.Schedule {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)

	res := make([]types.Schedule, 0)

	iterator := storetypes.KVStorePrefixIterator(store, []byte{})
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var schedule types.Schedule
		k.cdc.MustUnmarshal(iterator.Value(), &schedule)
		res = append(res, schedule)
	}

	return res
}

func (k *Keeper) GetScheduleCount(ctx sdk.Context) int32 {
	return k.getScheduleCount(ctx)
}

func (k *Keeper) getSchedulesReadyForExecution(ctx sdk.Context, executionStage types.ExecutionStage) []types.Schedule {
	params := k.GetParams(ctx)
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)
	count := uint64(0)

	res := make([]types.Schedule, 0)

	iterator := storetypes.KVStorePrefixIterator(store, []byte{})
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var schedule types.Schedule
		k.cdc.MustUnmarshal(iterator.Value(), &schedule)

		if k.intervalPassed(ctx, schedule) && schedule.ExecutionStage == executionStage {
			res = append(res, schedule)
			count++

			if count >= params.Limit {
				k.Logger(ctx).Info("limit of schedule executions per block reached")
				return res
			}
		}
	}

	return res
}

// executeSchedule executes all msgs in a given schedule and changes LastExecuteHeight
// if at least one msg execution fails, rollback all messages
func (k *Keeper) executeSchedule(ctx sdk.Context, schedule types.Schedule) error {
	// Even if contract execution returned an error, we still increase the height
	// and execute it after this interval
	schedule.LastExecuteHeight = uint64(ctx.BlockHeight()) //nolint:gosec
	k.storeSchedule(ctx, schedule)

	cacheCtx, writeFn := ctx.CacheContext()

	for idx, msg := range schedule.Msgs {
		executeMsg := wasmtypes.MsgExecuteContract{
			Sender:   k.accountKeeper.GetModuleAddress(types.ModuleName).String(),
			Contract: msg.Contract,
			Msg:      []byte(msg.Msg),
			Funds:    sdk.NewCoins(),
		}
		_, err := k.WasmMsgServer.ExecuteContract(cacheCtx, &executeMsg)
		if err != nil {
			ctx.Logger().Info("executeSchedule: failed to execute contract msg",
				"schedule_name", schedule.Name,
				"msg_idx", idx,
				"msg_contract", msg.Contract,
				"msg", msg.Msg,
				"error", err,
			)
			return err
		}
	}

	// only save state if all the messages in a schedule were executed successfully
	writeFn()
	return nil
}

func (k *Keeper) storeSchedule(ctx sdk.Context, schedule types.Schedule) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)

	bzSchedule := k.cdc.MustMarshal(&schedule)
	store.Set(types.GetScheduleKey(schedule.Name), bzSchedule)
}

func (k *Keeper) removeSchedule(ctx sdk.Context, name string) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)

	store.Delete(types.GetScheduleKey(name))
}

func (k *Keeper) scheduleExists(ctx sdk.Context, name string) bool {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)
	return store.Has(types.GetScheduleKey(name))
}

func (k *Keeper) intervalPassed(ctx sdk.Context, schedule types.Schedule) bool {
	return uint64(ctx.BlockHeight()) > (schedule.LastExecuteHeight + schedule.Period) //nolint:gosec
}

func (k *Keeper) changeTotalCount(ctx sdk.Context, incrementAmount int32) {
	store := ctx.KVStore(k.storeKey)
	count := k.getScheduleCount(ctx)
	newCount := types.ScheduleCount{Count: count + incrementAmount}
	bzCount := k.cdc.MustMarshal(&newCount)
	store.Set(types.ScheduleCountKey, bzCount)

	telemetry.ModuleSetGauge(types.ModuleName, float32(newCount.Count), LabelScheduleCount)
}

func (k *Keeper) getScheduleCount(ctx sdk.Context) int32 {
	store := ctx.KVStore(k.storeKey)
	bzCount := store.Get(types.ScheduleCountKey)
	if bzCount == nil {
		return 0
	}

	var count types.ScheduleCount
	k.cdc.MustUnmarshal(bzCount, &count)
	return count.Count
}

func recordExecutedSchedule(err error, schedule types.Schedule) {
	telemetry.IncrCounterWithLabels([]string{LabelScheduleExecutionsCount}, 1, []metrics.Label{
		telemetry.NewLabel(telemetry.MetricLabelNameModule, types.ModuleName),
		telemetry.NewLabel(MetricLabelSuccess, strconv.FormatBool(err == nil)),
		telemetry.NewLabel(MetricLabelScheduleName, schedule.Name),
	})
}
