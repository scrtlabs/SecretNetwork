package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/neutron-org/neutron/v5/x/cron/types"
)

func (k Keeper) Schedules(c context.Context, req *types.QuerySchedulesRequest) (*types.QuerySchedulesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	var schedules []types.Schedule
	ctx := sdk.UnwrapSDKContext(c)

	scheduleStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.ScheduleKey)

	pageRes, err := query.Paginate(scheduleStore, req.Pagination, func(_, value []byte) error {
		var schedule types.Schedule
		k.cdc.MustUnmarshal(value, &schedule)

		schedules = append(schedules, schedule)
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QuerySchedulesResponse{Schedules: schedules, Pagination: pageRes}, nil
}

func (k Keeper) Schedule(c context.Context, req *types.QueryGetScheduleRequest) (*types.QueryGetScheduleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(c)

	val, found := k.GetSchedule(
		ctx,
		req.Name,
	)
	if !found {
		return nil, status.Error(codes.NotFound, "schedule not found")
	}

	return &types.QueryGetScheduleResponse{Schedule: *val}, nil
}
