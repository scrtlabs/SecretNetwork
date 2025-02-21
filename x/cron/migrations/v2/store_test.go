package v2_test

import (
	"testing"

	"cosmossdk.io/store/prefix"
	"github.com/stretchr/testify/suite"

	"github.com/neutron-org/neutron/v5/testutil"
	v2 "github.com/neutron-org/neutron/v5/x/cron/migrations/v2"
	"github.com/neutron-org/neutron/v5/x/cron/types"
	v1types "github.com/neutron-org/neutron/v5/x/cron/types/v1"
)

type V2CronMigrationTestSuite struct {
	testutil.IBCConnectionTestSuite
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(V2CronMigrationTestSuite))
}

func (suite *V2CronMigrationTestSuite) TestScheduleUpgrade() {
	var (
		app      = suite.GetNeutronZoneApp(suite.ChainA)
		storeKey = app.GetKey(types.StoreKey)
		ctx      = suite.ChainA.GetContext()
		cdc      = app.AppCodec()
	)

	schedule := v1types.Schedule{
		Name:   "name",
		Period: 3,
		Msgs: []v1types.MsgExecuteContract{
			{
				Contract: "contract",
				Msg:      "msg",
			},
		},
		LastExecuteHeight: 1,
	}

	store := prefix.NewStore(ctx.KVStore(storeKey), types.ScheduleKey)
	bz := cdc.MustMarshal(&schedule)
	store.Set(types.GetScheduleKey(schedule.Name), bz)

	// Run migration
	suite.NoError(v2.MigrateStore(ctx, cdc, storeKey))

	// Check Schedule has correct ExecutionStage
	newSchedule, _ := app.CronKeeper.GetSchedule(ctx, schedule.Name)
	suite.Equal(newSchedule.Name, schedule.Name)
	suite.Equal(newSchedule.Period, schedule.Period)
	for i, msg := range newSchedule.Msgs {
		suite.Equal(msg.Contract, schedule.Msgs[i].Contract)
		suite.Equal(msg.Msg, schedule.Msgs[i].Msg)
	}
	suite.Equal(newSchedule.LastExecuteHeight, schedule.LastExecuteHeight)
	suite.Equal(newSchedule.ExecutionStage, types.ExecutionStage_EXECUTION_STAGE_END_BLOCKER)
}
