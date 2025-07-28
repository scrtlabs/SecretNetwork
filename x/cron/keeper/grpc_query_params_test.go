package keeper_test

import (
	"testing"

	testkeeper "github.com/scrtlabs/SecretNetwork/testutil/cron/keeper"

	"github.com/stretchr/testify/require"

	"github.com/scrtlabs/SecretNetwork/x/cron/types"
)

func TestParamsQuery(t *testing.T) {
	keeper, ctx := testkeeper.CronKeeper(t, nil)
	params := types.DefaultParams()
	err := keeper.SetParams(ctx, params)
	require.NoError(t, err)

	response, err := keeper.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryParamsResponse{Params: params}, response)
}
