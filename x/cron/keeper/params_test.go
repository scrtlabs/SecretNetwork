package keeper_test

import (
	"testing"

	testkeeper "github.com/scrtlabs/SecretNetwork/testutil/cron/keeper"

	"github.com/stretchr/testify/require"

	"github.com/scrtlabs/SecretNetwork/x/cron/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := testkeeper.CronKeeper(t, nil)
	params := types.Params{
		SecurityAddress: k.GetAuthority(),
		Limit:           5,
	}

	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
}
