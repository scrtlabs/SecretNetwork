package keeper_test

import (
	"testing"

	"github.com/neutron-org/neutron/v5/app/config"

	"github.com/neutron-org/neutron/v5/testutil"

	testkeeper "github.com/neutron-org/neutron/v5/testutil/cron/keeper"

	"github.com/stretchr/testify/require"

	"github.com/neutron-org/neutron/v5/x/cron/types"
)

func TestGetParams(t *testing.T) {
	_ = config.GetDefaultConfig()

	k, ctx := testkeeper.CronKeeper(t, nil, nil)
	params := types.Params{
		SecurityAddress: testutil.TestOwnerAddress,
		Limit:           5,
	}

	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
}
