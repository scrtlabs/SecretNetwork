package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	testkeeper "github.com/scrtlabs/SecretNetwork/testutil/cron/keeper"
	cronkeeper "github.com/scrtlabs/SecretNetwork/x/cron/keeper"
	"github.com/scrtlabs/SecretNetwork/x/cron/types"
)

// const k.GetAuthority() = "secret10d07y265gmmuvt4z0w9aw880jnsr700jc88vt0"

func TestMsgAddScheduleValidate(t *testing.T) {
	k, ctx := testkeeper.CronKeeper(t, nil)
	msgServer := cronkeeper.NewMsgServerImpl(*k)

	tests := []struct {
		name        string
		msg         types.MsgAddSchedule
		expectedErr string
	}{
		{
			"empty authority",
			types.MsgAddSchedule{
				Authority: "",
				Name:      "name",
				Period:    3,
				Msgs: []types.MsgExecuteContract{
					{
						Contract: "contract",
						Msg:      "msg",
					},
				},
				ExecutionStage: types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER,
			},
			"authority is invalid",
		},
		{
			"invalid authority",
			types.MsgAddSchedule{
				Authority: "invalid authority",
				Name:      "name",
				Period:    3,
				Msgs: []types.MsgExecuteContract{
					{
						Contract: "contract",
						Msg:      "msg",
					},
				},
				ExecutionStage: types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER,
			},
			"authority is invalid",
		},
		{
			"invalid name",
			types.MsgAddSchedule{
				Authority: k.GetAuthority(),
				Name:      "",
				Period:    3,
				Msgs: []types.MsgExecuteContract{
					{
						Contract: "contract",
						Msg:      "msg",
					},
				},
				ExecutionStage: types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER,
			},
			"name is invalid",
		},
		{
			"invalid period",
			types.MsgAddSchedule{
				Authority: k.GetAuthority(),
				Name:      "name",
				Period:    0,
				Msgs: []types.MsgExecuteContract{
					{
						Contract: "contract",
						Msg:      "msg",
					},
				},
				ExecutionStage: types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER,
			},
			"period is invalid",
		},
		{
			"empty msgs",
			types.MsgAddSchedule{
				Authority:      k.GetAuthority(),
				Name:           "name",
				Period:         3,
				Msgs:           []types.MsgExecuteContract{},
				ExecutionStage: types.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER,
			},
			"msgs should not be empty",
		},
		{
			"invalid execution stage",
			types.MsgAddSchedule{
				Authority: k.GetAuthority(),
				Name:      "name",
				Period:    3,
				Msgs: []types.MsgExecuteContract{
					{
						Contract: "contract",
						Msg:      "msg",
					},
				},
				ExecutionStage: 7,
			},
			"execution stage is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := msgServer.AddSchedule(ctx, &tt.msg)
			require.ErrorContains(t, err, tt.expectedErr)
			require.Nil(t, resp)
		})
	}
}

func TestMsgRemoveScheduleValidate(t *testing.T) {
	k, ctx := testkeeper.CronKeeper(t, nil)
	msgServer := cronkeeper.NewMsgServerImpl(*k)

	tests := []struct {
		name        string
		msg         types.MsgRemoveSchedule
		expectedErr string
	}{
		{
			"empty authority",
			types.MsgRemoveSchedule{
				Authority: "",
				Name:      "name",
			},
			"authority is invalid",
		},
		{
			"invalid authority",
			types.MsgRemoveSchedule{
				Authority: "invalid authority",
				Name:      "name",
			},
			"authority is invalid",
		},
		{
			"invalid name",
			types.MsgRemoveSchedule{
				Authority: k.GetAuthority(),
				Name:      "",
			},
			"name is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := msgServer.RemoveSchedule(ctx, &tt.msg)
			require.ErrorContains(t, err, tt.expectedErr)
			require.Nil(t, resp)
		})
	}
}

func TestMsgUpdateParamsValidate(t *testing.T) {
	k, ctx := testkeeper.CronKeeper(t, nil)
	msgServer := cronkeeper.NewMsgServerImpl(*k)

	tests := []struct {
		name        string
		msg         types.MsgUpdateParams
		expectedErr string
	}{
		{
			"empty authority",
			types.MsgUpdateParams{
				Authority: "",
			},
			"authority is invalid",
		},
		{
			"invalid authority",
			types.MsgUpdateParams{
				Authority: "invalid authority",
			},
			"authority is invalid",
		},
		{
			"empty security_address",
			types.MsgUpdateParams{
				Authority: k.GetAuthority(),
				Params: types.Params{
					SecurityAddress: "",
				},
			},
			"security_address is invalid",
		},
		{
			"invalid security_address",
			types.MsgUpdateParams{
				Authority: k.GetAuthority(),
				Params: types.Params{
					SecurityAddress: "invalid security_address",
				},
			},
			"security_address is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := msgServer.UpdateParams(ctx, &tt.msg)
			require.ErrorContains(t, err, tt.expectedErr)
			require.Nil(t, resp)
		})
	}
}
