package keeper

import (
	"context"

	"cosmossdk.io/collections"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// SigningRequest queries a signing request by ID
func (s queryServer) SigningRequest(ctx context.Context, req *types.QuerySigningRequestRequest) (*types.QuerySigningRequestResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	request, err := s.k.SigningRequestStore.Get(ctx, req.RequestId)
	if err != nil {
		if err == collections.ErrNotFound {
			return nil, status.Error(codes.NotFound, "signing request not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QuerySigningRequestResponse{Request: request}, nil
}

// AllSigningRequests queries all signing requests
func (s queryServer) AllSigningRequests(ctx context.Context, req *types.QueryAllSigningRequestsRequest) (*types.QueryAllSigningRequestsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	var requests []types.SigningRequest
	err := s.k.SigningRequestStore.Walk(ctx, nil, func(key string, value types.SigningRequest) (bool, error) {
		requests = append(requests, value)
		return false, nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryAllSigningRequestsResponse{Requests: requests}, nil
}
