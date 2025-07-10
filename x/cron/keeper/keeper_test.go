package keeper_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/scrtlabs/SecretNetwork/testutil"
	testutil_keeper "github.com/scrtlabs/SecretNetwork/testutil/cron/keeper"
	"github.com/scrtlabs/SecretNetwork/x/cron/types"
)

// ExecuteReadySchedules:
// - calls msgServer.execute() on ready schedules
// - updates ready schedules lastExecuteHeight
// - does not update lastExecuteHeight of unready schedules
// - does not go over the limit
func TestKeeperExecuteReadySchedules(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := testutil_keeper.CronKeeper(t, nil)
	ctx = ctx.WithBlockHeight(0)

	err := k.SetParams(ctx, types.Params{
		SecurityAddress: testutil.TestOwnerAddress,
		Limit:           2,
	})
	require.NoError(t, err)

	schedules := []types.Schedule{
		{
			Name:   "1_unready1",
			Period: 10,
			Msgs: []types.MsgExecuteContract{
				{
					Contract: "1_neutron",
					Msg:      "1_msg",
				},
			},
			LastExecuteHeight: 4,
		},
		{
			Name:   "2_ready1",
			Period: 4,
			Msgs: []types.MsgExecuteContract{
				{
					Contract: "2_neutron",
					Msg:      "2_msg",
				},
			},
			LastExecuteHeight: 0,
		},
		{
			Name:   "3_ready2",
			Period: 4,
			Msgs: []types.MsgExecuteContract{
				{
					Contract: "3_neutron",
					Msg:      "3_msg",
				},
			},
			LastExecuteHeight: 0,
		},
		{
			Name:              "4_unready2",
			Period:            10,
			Msgs:              []types.MsgExecuteContract{},
			LastExecuteHeight: 4,
		},
		{
			Name:   "5_ready3",
			Period: 3,
			Msgs: []types.MsgExecuteContract{
				{
					Contract: "5_neutron",
					Msg:      "5_msg",
				},
			},
			LastExecuteHeight: 0,
		},
		{
			Name:   "6_ready4",
			Period: 3,
			Msgs: []types.MsgExecuteContract{
				{
					Contract: "6_neutron",
					Msg:      "6_msg",
				},
			},
			LastExecuteHeight: 0,
		},
	}

	for _, item := range schedules {
		ctx = ctx.WithBlockHeight(int64(item.LastExecuteHeight)) //nolint:gosec
		err := k.AddSchedule(ctx, item.Name, item.Period, item.Msgs)
		require.NoError(t, err)
	}

	count := k.GetScheduleCount(ctx)
	require.Equal(t, count, int32(6))

	ctx = ctx.WithBlockHeight(5)

	// Use GetScheduledMsgs instead of ExecuteReadySchedules
	_ = k.GetScheduledMsgs(ctx, types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)

	unready1, _ := k.GetSchedule(ctx, "1_unready1")
	ready1, _ := k.GetSchedule(ctx, "2_ready1")
	ready2, _ := k.GetSchedule(ctx, "3_ready2")
	unready2, _ := k.GetSchedule(ctx, "4_unready2")
	ready3, _ := k.GetSchedule(ctx, "5_ready3")
	ready4, _ := k.GetSchedule(ctx, "6_ready4")

	require.Equal(t, uint64(4), unready1.LastExecuteHeight)
	require.Equal(t, uint64(5), ready1.LastExecuteHeight)
	require.Equal(t, uint64(5), ready2.LastExecuteHeight)
	require.Equal(t, uint64(4), unready2.LastExecuteHeight)
	require.Equal(t, uint64(0), ready3.LastExecuteHeight)
	require.Equal(t, uint64(0), ready4.LastExecuteHeight)

	// let's make another call at the next height
	// Notice that now only one ready schedule left because we got limit of 2 at once
	ctx = ctx.WithBlockHeight(6)

	_ = k.GetScheduledMsgs(ctx, types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)

	unready1, _ = k.GetSchedule(ctx, "1_unready1")
	ready1, _ = k.GetSchedule(ctx, "2_ready1")
	ready2, _ = k.GetSchedule(ctx, "3_ready2")
	unready2, _ = k.GetSchedule(ctx, "4_unready2")
	ready3, _ = k.GetSchedule(ctx, "5_ready3")
	ready4, _ = k.GetSchedule(ctx, "6_ready4")

	require.Equal(t, uint64(4), unready1.LastExecuteHeight)
	require.Equal(t, uint64(5), ready1.LastExecuteHeight)
	require.Equal(t, uint64(5), ready2.LastExecuteHeight)
	require.Equal(t, uint64(4), unready2.LastExecuteHeight)
	require.Equal(t, uint64(6), ready3.LastExecuteHeight)
	require.Equal(t, uint64(6), ready4.LastExecuteHeight)

	ctx = ctx.WithBlockHeight(8)

	_ = k.GetScheduledMsgs(ctx, types.ExecutionStage_EXECUTION_STAGE_END_BLOCKER)

	unready1, _ = k.GetSchedule(ctx, "1_unready1")
	ready1, _ = k.GetSchedule(ctx, "2_ready1")
	ready2, _ = k.GetSchedule(ctx, "3_ready2")
	unready2, _ = k.GetSchedule(ctx, "4_unready2")
	ready3, _ = k.GetSchedule(ctx, "5_ready3")
	ready4, _ = k.GetSchedule(ctx, "6_ready4")

	require.Equal(t, uint64(4), unready1.LastExecuteHeight)
	require.Equal(t, uint64(5), ready1.LastExecuteHeight)
	require.Equal(t, uint64(5), ready2.LastExecuteHeight)
	require.Equal(t, uint64(4), unready2.LastExecuteHeight)
	require.Equal(t, uint64(6), ready3.LastExecuteHeight)
	require.Equal(t, uint64(6), ready4.LastExecuteHeight)

	// reset context to 0 block
	ctx = ctx.WithBlockHeight(0)
	everyTimeSchedule := types.Schedule{
		Name:   "every_block",
		Period: 1,
		Msgs: []types.MsgExecuteContract{
			{
				Contract: "every_block",
				Msg:      "every_block",
			},
		},
		LastExecuteHeight: 0,
	}
	err = k.AddSchedule(ctx, everyTimeSchedule.Name, everyTimeSchedule.Period, everyTimeSchedule.Msgs)

	s, _ := k.GetSchedule(ctx, "every_block")
	require.Equal(t, s.LastExecuteHeight, uint64(0))
	require.NoError(t, err)

	// expect it to not executed right away

	_ = k.GetScheduledMsgs(ctx, types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)
	// LastExecuteHeight should still be at 0
	s, _ = k.GetSchedule(ctx, "every_block")
	require.Equal(t, s.LastExecuteHeight, uint64(0))

	ctx = ctx.WithBlockHeight(1)
	// expect it to be executed again
	_ = k.GetScheduledMsgs(ctx, types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)
	// last execute height changed to 1
	s, _ = k.GetSchedule(ctx, "every_block")
	require.Equal(t, s.LastExecuteHeight, uint64(1))

	k.RemoveSchedule(ctx, "every_block")

	// test schedule with period 2
	ctx = ctx.WithBlockHeight(0)
	onceTwoBlocksSchedule := types.Schedule{
		Name:   "once_in_two",
		Period: 2,
		Msgs: []types.MsgExecuteContract{
			{
				Contract: "once_in_two",
				Msg:      "once_in_two",
			},
		},
		LastExecuteHeight: 0,
	}
	err = k.AddSchedule(ctx, onceTwoBlocksSchedule.Name, onceTwoBlocksSchedule.Period, onceTwoBlocksSchedule.Msgs)

	s, _ = k.GetSchedule(ctx, "once_in_two")
	require.Equal(t, s.LastExecuteHeight, uint64(0))
	require.NoError(t, err)

	// expect it to not executed on 0 and 1 blocks
	_ = k.GetScheduledMsgs(ctx, types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)
	// LastExecuteHeight should still be at 0
	s, _ = k.GetSchedule(ctx, "once_in_two")
	require.Equal(t, s.LastExecuteHeight, uint64(0))

	ctx = ctx.WithBlockHeight(1)
	_ = k.GetScheduledMsgs(ctx, types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)
	s, _ = k.GetSchedule(ctx, "once_in_two")
	require.Equal(t, s.LastExecuteHeight, uint64(0))

	// expect it to be executed on 2 block
	ctx = ctx.WithBlockHeight(2)
	// expect it to be executed again
	_ = k.GetScheduledMsgs(ctx, types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)
	// last execute height changed to 2
	s, _ = k.GetSchedule(ctx, "once_in_two")
	require.Equal(t, s.LastExecuteHeight, uint64(2))
}

func TestAddSchedule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	k, ctx := testutil_keeper.CronKeeper(t, nil)
	ctx = ctx.WithBlockHeight(0)

	err := k.SetParams(ctx, types.Params{
		SecurityAddress: testutil.TestOwnerAddress,
		Limit:           2,
	})
	require.NoError(t, err)

	// normal add schedule
	err = k.AddSchedule(ctx, "a", 7, []types.MsgExecuteContract{
		{
			Contract: "c",
			Msg:      "m",
		},
	})
	require.NoError(t, err)

	err = k.AddSchedule(ctx, "b", 7, []types.MsgExecuteContract{
		{
			Contract: "c",
			Msg:      "m",
		},
	})
	require.NoError(t, err)

	// second time with same name returns error
	err = k.AddSchedule(ctx, "a", 5, []types.MsgExecuteContract{})
	require.Error(t, err)

	scheduleA, found := k.GetSchedule(ctx, "a")
	require.True(t, found)
	require.Equal(t, scheduleA.Name, "a")
	require.Equal(t, scheduleA.Period, uint64(7))
	require.Equal(t, scheduleA.Msgs, []types.MsgExecuteContract{
		{Contract: "c", Msg: "m"},
	})
	// require.Equal(t, scheduleA.ExecutionStage, types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)

	schedules := k.GetAllSchedules(ctx)
	require.Len(t, schedules, 2)
	// require.Equal(t, schedules[0].ExecutionStage, types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)
	// require.Equal(t, schedules[1].ExecutionStage, types.ExecutionStage_EXECUTION_STAGE_END_BLOCKER)

	// remove schedule works
	k.RemoveSchedule(ctx, "a")
	_, found = k.GetSchedule(ctx, "a")
	assert.False(t, found)

	// does not panic even though we don't have it
	k.RemoveSchedule(ctx, "a")
}

func TestGetAllSchedules(t *testing.T) {
	k, ctx := testutil_keeper.CronKeeper(t, nil)

	err := k.SetParams(ctx, types.Params{
		SecurityAddress: testutil.TestOwnerAddress,
		Limit:           2,
	})
	require.NoError(t, err)

	expectedSchedules := make([]types.Schedule, 0, 3)
	for i := range []int{1, 2, 3} {
		s := types.Schedule{
			Name:              strconv.Itoa(i),
			Period:            5,
			Msgs:              nil,
			LastExecuteHeight: uint64(ctx.BlockHeight()), //nolint:gosec
		}
		expectedSchedules = append(expectedSchedules, s)
		err := k.AddSchedule(ctx, s.Name, s.Period, s.Msgs)
		require.NoError(t, err)
	}

	schedules := k.GetAllSchedules(ctx)
	assert.Equal(t, 3, len(schedules))
	assert.ElementsMatch(t, schedules, expectedSchedules)
	assert.Equal(t, int32(3), k.GetScheduleCount(ctx))
}
