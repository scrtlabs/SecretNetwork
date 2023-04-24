package grpc

// todo maybe this should be auto-generated somehow

import (
	"context"

	"github.com/scrtlabs/SecretNetwork/x/ibc-switch/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/ibc-switch/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Querier struct {
	Q client.Querier
}

var _ types.QueryServer = Querier{}

func (q Querier) Params(grpcCtx context.Context,
	req *types.ParamsRequest,
) (*types.ParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(grpcCtx)
	return q.Q.Params(ctx, *req)
}
