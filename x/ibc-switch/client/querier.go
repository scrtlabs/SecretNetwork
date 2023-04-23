package client

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	ibcswitch "github.com/scrtlabs/SecretNetwork/x/ibc-switch"
	"github.com/scrtlabs/SecretNetwork/x/ibc-switch/types"
)

// This file should evolve to being code gen'd, off of `proto/twap/v1beta/query.yml`

type Querier struct {
	K ibcswitch.ICS4Wrapper
}

func (q Querier) Params(ctx sdk.Context,
	req types.ParamsRequest,
) (*types.ParamsResponse, error) {
	params := q.K.GetParams(ctx)
	return &types.ParamsResponse{Params: params}, nil
}
