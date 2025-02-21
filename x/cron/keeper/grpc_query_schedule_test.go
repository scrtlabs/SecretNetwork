package keeper_test

import (
	"strconv"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/neutron-org/neutron/v5/testutil/common/nullify"
	testutil_keeper "github.com/neutron-org/neutron/v5/testutil/cron/keeper"
	cronkeeper "github.com/neutron-org/neutron/v5/x/cron/keeper"
	"github.com/neutron-org/neutron/v5/x/cron/types"
)

// Prevent strconv unused error
var _ = strconv.IntSize

func TestScheduleQuerySingle(t *testing.T) {
	k, ctx := testutil_keeper.CronKeeper(t, nil, nil)
	schedules := createNSchedule(t, ctx, k, 2)

	for _, tc := range []struct {
		desc     string
		request  *types.QueryGetScheduleRequest
		response *types.QueryGetScheduleResponse
		err      error
	}{
		{
			desc: "First",
			request: &types.QueryGetScheduleRequest{
				Name: schedules[0].Name,
			},
			response: &types.QueryGetScheduleResponse{Schedule: schedules[0]},
		},
		{
			desc: "Second",
			request: &types.QueryGetScheduleRequest{
				Name: schedules[1].Name,
			},
			response: &types.QueryGetScheduleResponse{Schedule: schedules[1]},
		},
		{
			desc: "KeyIsAbsent",
			request: &types.QueryGetScheduleRequest{
				Name: "absent_key",
			},
			err: status.Error(codes.NotFound, "schedule not found"),
		},
		{
			desc: "InvalidRequest",
			err:  status.Error(codes.InvalidArgument, "invalid request"),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			response, err := k.Schedule(ctx, tc.request)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t,
					nullify.Fill(tc.response),
					nullify.Fill(response),
				)
			}
		})
	}
}

func TestScheduleQueryPaginated(t *testing.T) {
	k, ctx := testutil_keeper.CronKeeper(t, nil, nil)
	schedules := createNSchedule(t, ctx, k, 5)

	request := func(next []byte, offset, limit uint64, total bool) *types.QuerySchedulesRequest {
		return &types.QuerySchedulesRequest{
			Pagination: &query.PageRequest{
				Key:        next,
				Offset:     offset,
				Limit:      limit,
				CountTotal: total,
			},
		}
	}
	t.Run("ByOffset", func(t *testing.T) {
		step := 2
		for i := 0; i < len(schedules); i += step {
			resp, err := k.Schedules(ctx, request(nil, uint64(i), uint64(step), false)) //nolint:gosec
			require.NoError(t, err)
			require.LessOrEqual(t, len(resp.Schedules), step)
			require.Subset(t,
				nullify.Fill(schedules),
				nullify.Fill(resp.Schedules),
			)
		}
	})
	t.Run("ByKey", func(t *testing.T) {
		step := 2
		var next []byte
		for i := 0; i < len(schedules); i += step {
			resp, err := k.Schedules(ctx, request(next, 0, uint64(step), false)) //nolint:gosec
			require.NoError(t, err)
			require.LessOrEqual(t, len(resp.Schedules), step)
			require.Subset(t,
				nullify.Fill(schedules),
				nullify.Fill(resp.Schedules),
			)
			next = resp.Pagination.NextKey
		}
	})
	t.Run("Total", func(t *testing.T) {
		resp, err := k.Schedules(ctx, request(nil, 0, 0, true))
		require.NoError(t, err)
		require.Equal(t, len(schedules), int(resp.Pagination.Total)) //nolint:gosec
		require.ElementsMatch(t,
			nullify.Fill(schedules),
			nullify.Fill(resp.Schedules),
		)
	})
	t.Run("InvalidRequest", func(t *testing.T) {
		_, err := k.Schedules(ctx, nil)
		require.ErrorIs(t, err, status.Error(codes.InvalidArgument, "invalid request"))
	})
}

func createNSchedule(t *testing.T, ctx sdk.Context, k *cronkeeper.Keeper, n int32) []types.Schedule {
	res := make([]types.Schedule, n)

	for idx, item := range res {
		item.Name = strconv.Itoa(idx)
		item.Period = 1000
		item.Msgs = nil
		item.LastExecuteHeight = uint64(ctx.BlockHeight()) //nolint:gosec
		item.ExecutionStage = types.ExecutionStage_EXECUTION_STAGE_END_BLOCKER

		err := k.AddSchedule(ctx, item.Name, item.Period, item.Msgs, item.ExecutionStage)
		require.NoError(t, err)

		res[idx] = item
	}

	return res
}
