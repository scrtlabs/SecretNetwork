package cron_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scrtlabs/SecretNetwork/testutil/common/nullify"
	"github.com/scrtlabs/SecretNetwork/testutil/cron/keeper"
	"github.com/scrtlabs/SecretNetwork/x/cron"
	"github.com/scrtlabs/SecretNetwork/x/cron/types"
)

func TestGenesis(t *testing.T) {
	k, ctx := keeper.CronKeeper(t, nil)

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
