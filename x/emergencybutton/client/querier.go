package client

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	ibcswitch "github.com/scrtlabs/SecretNetwork/x/emergencybutton/keeper"
	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

// This file should evolve to being code gen'd, off of `proto/twap/v1beta/query.yml`

type Querier struct {
	K ibcswitch.Keeper
}

func (q Querier) Params(ctx sdk.Context,
	_ types.ParamsRequest,
) (*types.ParamsResponse, error) {
	params := q.K.GetParams(ctx)
	return &types.ParamsResponse{Params: params}, nil
}
