package keeper

import (
	"fmt"
	"strconv"

	"cosmossdk.io/log"
	"github.com/hashicorp/go-metrics"

	"github.com/cosmos/cosmos-sdk/telemetry"

	"cosmossdk.io/store/prefix"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/cron/types"
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
		regKeeper     types.RegKeeper
		// WasmMsgServer types.WasmMsgServer
		authority string
		txConfig  client.TxConfig
	}
)

func (k *Keeper) GetTxConfig() client.TxConfig {
	return k.txConfig
}

// GetScheduledMsgs implements types.CronKeeper.
func (k *Keeper) GetScheduledMsgs(ctx sdk.Context) []types.MsgExecuteContract {
	schedules := k.getSchedulesReadyForExecution(ctx)
	var msgExecuteContractList []types.MsgExecuteContract
	for _, schedule := range schedules {
		msgs, err := k.getCronsMsgs(ctx, schedule)
		if err != nil {
			ctx.Logger().Error("Failed to get crons msgs", "error", err)
			continue
		}

		msgExecuteContractList = append(msgExecuteContractList, msgs...)
		recordExecutedSchedule(nil, schedule)
	}
	return msgExecuteContractList
}

// executeSchedule executes all msgs in a given schedule and changes LastExecuteHeight
// if at least one msg execution fails, rollback all messages
func (k *Keeper) getCronsMsgs(ctx sdk.Context, schedule types.Schedule) ([]types.MsgExecuteContract, error) {
	// Even if contract execution returned an error, we still increase the height
	// and execute it after this interval
	schedule.LastExecuteHeight = uint64(ctx.BlockHeight() + 1) //nolint:gosec
	k.storeSchedule(ctx, schedule)

	var cronMsgs []types.MsgExecuteContract

	for _, msg := range schedule.Msgs {
		contractAddr, err := sdk.AccAddressFromBech32(msg.Contract)
		if err != nil {
			ctx.Logger().Info("getCronsMsgs: failed to extract contract address", "err", err)
			return nil, err
		}
		executeMsg := types.MsgExecuteContract{
			Contract: contractAddr.String(),
			Msg:      msg.Msg,
		}
		cronMsgs = append(cronMsgs, executeMsg)
	}
	return cronMsgs, nil
}

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
		regKeeper:     nil,
		authority:     authority,
		txConfig:      nil,
	}
}

// SetTxConfig injects the transaction configuration into the keeper.
func (k *Keeper) SetTxConfig(txConfig client.TxConfig) {
	k.txConfig = txConfig
}

func (k *Keeper) SetRegKeeper(regKeeper types.RegKeeper) {
	k.regKeeper = regKeeper
}

func (k Keeper) GetAuthority() string {
	return k.authority
}

func (k *Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// AddSchedule adds a new schedule to be executed every certain number of blocks, specified in the `period`.
// First schedule execution is supposed to be on `now + period` block.
func (k *Keeper) AddSchedule(
	ctx sdk.Context,
	name string,
	period uint64,
	msgs []types.MsgExecuteContract,
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

func (k *Keeper) getSchedulesReadyForExecution(ctx sdk.Context) []types.Schedule {
	params := k.GetParams(ctx)
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)
	count := uint64(0)

	res := make([]types.Schedule, 0)

	iterator := storetypes.KVStorePrefixIterator(store, []byte{})
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var schedule types.Schedule
		k.cdc.MustUnmarshal(iterator.Value(), &schedule)
		if k.intervalPassed(ctx, schedule) {
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
	// EndBlock runs at the end of block N and prepares messages for block N+1.
	// So we check if the schedule should execute in block N+1:
	// (currentHeight + 1) >= (lastExecuteHeight + period)
	return uint64(ctx.BlockHeight())+1 >= (schedule.LastExecuteHeight + schedule.Period) //nolint:gosec
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
