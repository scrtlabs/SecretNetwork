package cron_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neutron-org/neutron/v5/testutil/common/nullify"
	"github.com/neutron-org/neutron/v5/testutil/cron/keeper"
	"github.com/neutron-org/neutron/v5/x/cron"
	"github.com/neutron-org/neutron/v5/x/cron/types"
)

func TestGenesis(t *testing.T) {
	k, ctx := keeper.CronKeeper(t, nil, nil)

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		ScheduleList: []types.Schedule{
			{
				Name:              "a",
				Period:            5,
				Msgs:              nil,
				LastExecuteHeight: uint64(ctx.BlockHeight()), //nolint:gosec
			},
		},
	}

	cron.InitGenesis(ctx, *k, genesisState)
	got := cron.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)

	require.Equal(t, genesisState.Params, got.Params)
	require.ElementsMatch(t, genesisState.ScheduleList, got.ScheduleList)
}
