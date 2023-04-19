package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
	icatypes "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/types"

	"github.com/scrtlabs/SecretNetwork/x/mauth/types"
)

// InterchainAccountFromAddress implements the Query/InterchainAccountFromAddress gRPC method
func (k Keeper) InterchainAccountFromAddress(goCtx context.Context, req *types.QueryInterchainAccountFromAddressRequest) (*types.QueryInterchainAccountFromAddressResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	portID, err := icatypes.NewControllerPortID(req.Owner)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not find account: %s", err)
	}

	addr, found := k.icaControllerKeeper.GetInterchainAccountAddress(ctx, req.ConnectionId, portID)
	if !found {
		return nil, status.Errorf(codes.NotFound, "no account found for connectionId %s portID %s",
			req.ConnectionId, portID)
	}

	return types.NewQueryInterchainAccountResponse(addr), nil
}
