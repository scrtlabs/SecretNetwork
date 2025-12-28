package keeper

import (
	"context"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/scrtlabs/SecretNetwork/x/cron/types"
)

type msgServer struct {
	keeper Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// AddSchedule adds new schedule
func (k msgServer) AddSchedule(goCtx context.Context, req *types.MsgAddSchedule) (*types.MsgAddScheduleResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to validate MsgAddSchedule")
	}

	authority := k.keeper.GetAuthority()
	if authority != req.Authority {
		return nil, errors.Wrapf(sdkerrors.ErrUnauthorized, "invalid authority; expected %s, got %s", authority, req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := k.keeper.AddSchedule(ctx, req.Name, req.Period, req.Msgs); err != nil {
		return nil, errors.Wrap(err, "failed to add schedule")
	}

	return &types.MsgAddScheduleResponse{}, nil
}

// RemoveSchedule removes schedule
func (k msgServer) RemoveSchedule(goCtx context.Context, req *types.MsgRemoveSchedule) (*types.MsgRemoveScheduleResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to validate MsgRemoveSchedule")
	}

	authority := k.keeper.GetAuthority()
	if authority != req.Authority {
		return nil, errors.Wrapf(sdkerrors.ErrUnauthorized, "invalid authority; expected %s, got %s", authority, req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	k.keeper.RemoveSchedule(ctx, req.Name)

	return &types.MsgRemoveScheduleResponse{}, nil
}

// UpdateParams updates the module parameters
func (k msgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to validate MsgUpdateParams")
	}

	authority := k.keeper.GetAuthority()
	if authority != req.Authority {
		return nil, errors.Wrapf(sdkerrors.ErrUnauthorized, "invalid authority; expected %s, got %s", authority, req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := k.keeper.SetParams(ctx, req.Params); err != nil {
		return nil, errors.Wrap(err, "failed to set params")
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
