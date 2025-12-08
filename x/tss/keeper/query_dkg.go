package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// DKGSession returns a specific DKG session by ID
func (qs queryServer) DKGSession(ctx context.Context, req *types.QueryDKGSessionRequest) (*types.QueryDKGSessionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id cannot be empty")
	}

	session, err := qs.k.GetDKGSession(ctx, req.SessionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("DKGSession query result",
		"id", session.Id,
		"key_set_id", session.KeySetId,
		"state", session.State,
		"participants_count", len(session.Participants),
		"participants", session.Participants)

	return &types.QueryDKGSessionResponse{
		Session: session,
	}, nil
}

// AllDKGSessions returns all DKG sessions
func (qs queryServer) AllDKGSessions(ctx context.Context, req *types.QueryAllDKGSessionsRequest) (*types.QueryAllDKGSessionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sessions, err := qs.k.GetAllDKGSessions(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("AllDKGSessions query", "count", len(sessions))
	for i, s := range sessions {
		sdkCtx.Logger().Info(fmt.Sprintf("DKGSession[%d]", i),
			"id", s.Id,
			"key_set_id", s.KeySetId,
			"state", s.State,
			"participants_count", len(s.Participants),
			"participants", s.Participants)
	}

	response := &types.QueryAllDKGSessionsResponse{
		Sessions: sessions,
		// TODO: Add pagination support
	}

	return response, nil
}
