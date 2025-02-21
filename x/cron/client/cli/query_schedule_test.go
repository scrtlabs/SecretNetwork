package cli_test

import (
	"fmt"
	"strconv"
	"testing"

	tmcli "github.com/cometbft/cometbft/libs/cli"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/neutron-org/neutron/v5/testutil/common/nullify"
	"github.com/neutron-org/neutron/v5/testutil/cron/network"
	"github.com/neutron-org/neutron/v5/x/cron/client/cli"
	"github.com/neutron-org/neutron/v5/x/cron/types"
)

func networkWithScheduleObjects(t *testing.T, n int) (*network.Network, []types.Schedule) {
	t.Helper()
	cfg := network.DefaultConfig()
	state := types.GenesisState{}
	require.NoError(t, cfg.Codec.UnmarshalJSON(cfg.GenesisState[types.ModuleName], &state))

	for i := 0; i < n; i++ {
		schedule := types.Schedule{
			Name:              strconv.Itoa(i),
			Period:            1000,
			Msgs:              []types.MsgExecuteContract{},
			LastExecuteHeight: uint64(0),
		}
		state.ScheduleList = append(state.ScheduleList, schedule)
	}
	state.Params = types.DefaultParams()
	buf, err := cfg.Codec.MarshalJSON(&state)
	require.NoError(t, err)
	cfg.GenesisState[types.ModuleName] = buf
	return network.New(t, cfg), state.ScheduleList
}

func TestShowSchedule(t *testing.T) {
	net, objs := networkWithScheduleObjects(t, 2)

	ctx := net.Validators[0].ClientCtx
	common := []string{
		fmt.Sprintf("--%s=json", tmcli.OutputFlag),
	}
	for _, tc := range []struct {
		desc string
		name string

		args []string
		err  error
		obj  types.Schedule
	}{
		{
			desc: "found",
			name: objs[0].Name,

			args: common,
			obj:  objs[0],
			err:  nil,
		},
		{
			desc: "not found",
			name: strconv.Itoa(100000),

			args: common,
			err:  status.Error(codes.NotFound, "not found"),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			args := []string{
				tc.name,
			}
			args = append(args, tc.args...)
			out, err := clitestutil.ExecTestCLICmd(ctx, cli.CmdShowSchedule(), args)
			if tc.err != nil {
				stat, ok := status.FromError(tc.err)
				require.True(t, ok)
				require.ErrorIs(t, stat.Err(), tc.err)
			} else {
				require.NoError(t, err)
				var resp types.QueryGetScheduleResponse
				require.NoError(t, net.Config.Codec.UnmarshalJSON(out.Bytes(), &resp))
				require.NotNil(t, resp.Schedule)
				require.Equal(t,
					nullify.Fill(&tc.obj),
					nullify.Fill(&resp.Schedule),
				)
			}
		})
	}
}

func TestListSchedule(t *testing.T) {
	net, objs := networkWithScheduleObjects(t, 5)

	ctx := net.Validators[0].ClientCtx
	request := func(next []byte, offset, limit uint64, total bool) []string {
		args := []string{
			fmt.Sprintf("--%s=json", tmcli.OutputFlag),
		}
		if next == nil {
			args = append(args, fmt.Sprintf("--%s=%d", flags.FlagOffset, offset))
		} else {
			args = append(args, fmt.Sprintf("--%s=%s", flags.FlagPageKey, next))
		}
		args = append(args, fmt.Sprintf("--%s=%d", flags.FlagLimit, limit))
		if total {
			args = append(args, fmt.Sprintf("--%s", flags.FlagCountTotal))
		}
		return args
	}
	t.Run("ByOffset", func(t *testing.T) {
		step := 2
		for i := 0; i < len(objs); i += step {
			args := request(nil, uint64(i), uint64(step), false) //nolint:gosec
			out, err := clitestutil.ExecTestCLICmd(ctx, cli.CmdListSchedule(), args)
			require.NoError(t, err)
			var resp types.QuerySchedulesResponse
			require.NoError(t, net.Config.Codec.UnmarshalJSON(out.Bytes(), &resp))
			require.LessOrEqual(t, len(resp.Schedules), step)
			require.Subset(t,
				nullify.Fill(objs),
				nullify.Fill(resp.Schedules),
			)
		}
	})
	t.Run("ByKey", func(t *testing.T) {
		step := 2
		var next []byte
		for i := 0; i < len(objs); i += step {
			args := request(next, 0, uint64(step), false) //nolint:gosec
			out, err := clitestutil.ExecTestCLICmd(ctx, cli.CmdListSchedule(), args)
			require.NoError(t, err)
			var resp types.QuerySchedulesResponse
			require.NoError(t, net.Config.Codec.UnmarshalJSON(out.Bytes(), &resp))
			require.LessOrEqual(t, len(resp.Schedules), step)
			require.Subset(t,
				nullify.Fill(objs),
				nullify.Fill(resp.Schedules),
			)
			next = resp.Pagination.NextKey
		}
	})
	t.Run("Total", func(t *testing.T) {
		args := request(nil, 0, uint64(len(objs)), true)
		out, err := clitestutil.ExecTestCLICmd(ctx, cli.CmdListSchedule(), args)
		require.NoError(t, err)
		var resp types.QuerySchedulesResponse
		require.NoError(t, net.Config.Codec.UnmarshalJSON(out.Bytes(), &resp))
		require.NoError(t, err)
		require.Equal(t, len(objs), int(resp.Pagination.Total)) //nolint:gosec
		require.ElementsMatch(t,
			nullify.Fill(objs),
			nullify.Fill(resp.Schedules),
		)
	})
}
