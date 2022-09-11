package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.QueryServer = (*queryServer)(nil)

// queryServer implements the gRPC querier service.
type queryServer struct {
	Keeper
}

// NewQueryServerImpl returns an implementation of the types.QueryServer interface.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

// Pool implements the types.QueryServer interface.
func (k queryServer) Pool(goCtx context.Context, req *types.QueryPoolRequest) (*types.QueryPoolResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	return &types.QueryPoolResponse{
		ActivePool:    k.Keeper.ActivePool(ctx),
		RedeemingPool: k.Keeper.RedeemingPool(ctx),
	}, nil
}

// Params implements the types.QueryServer interface.
func (k queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	return &types.QueryParamsResponse{
		Params: k.GetParams(ctx),
	}, nil
}

// RedeemEntry implements the types.QueryServer interface.
func (k queryServer) RedeemEntry(goCtx context.Context, req *types.QueryRedeemEntryRequest) (*types.QueryRedeemEntryResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	accAddr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "address parsing: %v", err)
	}

	entry, found := k.GetRedeemEntry(ctx, accAddr)
	if !found {
		return nil, status.Errorf(codes.NotFound, "redeem entry not found for account (%s)", accAddr.String())
	}

	return &types.QueryRedeemEntryResponse{
		Entry: entry,
	}, nil
}
