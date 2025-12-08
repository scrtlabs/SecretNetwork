package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// KeySet returns a specific KeySet by ID
func (qs queryServer) KeySet(ctx context.Context, req *types.QueryKeySetRequest) (*types.QueryKeySetResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "key_set_id cannot be empty")
	}

	keySet, err := qs.k.GetKeySet(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("KeySet query result",
		"id", keySet.Id,
		"owner", keySet.Owner,
		"threshold", keySet.Threshold,
		"max_signers", keySet.MaxSigners,
		"status", keySet.Status,
		"participants_count", len(keySet.Participants))

	return &types.QueryKeySetResponse{
		KeySet: keySet,
	}, nil
}

// AllKeySets returns all KeySets
func (qs queryServer) AllKeySets(ctx context.Context, req *types.QueryAllKeySetsRequest) (*types.QueryAllKeySetsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	keySets, err := qs.k.GetAllKeySets(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("AllKeySets query", "count", len(keySets))
	for i, ks := range keySets {
		sdkCtx.Logger().Info(fmt.Sprintf("KeySet[%d]", i),
			"id", ks.Id,
			"owner", ks.Owner,
			"threshold", ks.Threshold,
			"max_signers", ks.MaxSigners,
			"status", ks.Status)
	}

	response := &types.QueryAllKeySetsResponse{
		KeySets: keySets,
		// TODO: Add pagination support
	}

	sdkCtx.Logger().Info("AllKeySets RESPONSE",
		"key_sets_len", len(response.KeySets),
		"first_id", func() string {
			if len(response.KeySets) > 0 {
				return response.KeySets[0].Id
			}
			return "none"
		}())

	return response, nil
}
