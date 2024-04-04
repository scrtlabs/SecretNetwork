package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"

	"github.com/scrtlabs/SecretNetwork/x/mauth/keeper"
	"github.com/scrtlabs/SecretNetwork/x/mauth/types"
)

func (suite *KeeperTestSuite) TestRegisterInterchainAccount() {
	var (
		owner string
		path  *ibctesting.Path
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success", func() {}, true,
		},
		// {
		// 	"port is already bound",
		// 	func() {
		// 		suite.GetICAApp(suite.chainA).GetIBCKeeper().PortKeeper.BindPort(suite.chainA.GetContext(), TestPortID)
		// 	},
		// 	false,
		// },
		// {
		// 	"fails to generate port-id",
		// 	func() {
		// 		owner = ""
		// 	},
		// 	false,
		// },
		// {
		// 	"MsgChanOpenInit fails - channel is already active",
		// 	func() {
		// 		portID, err := icatypes.NewControllerPortID(owner)
		// 		suite.Require().NoError(err)

		// 		suite.GetICAApp(suite.chainA).AppKeepers.ICAControllerKeeper.SetActiveChannelID(suite.chainA.GetContext(), path.EndpointA.ConnectionID, portID, path.EndpointA.ChannelID)
		// 	},
		// 	false,
		// },
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest()

			owner = TestOwnerAddress // must be explicitly changed

			path = NewICAPath(suite.chainA, suite.chainB)
			suite.coordinator.SetupConnections(path)

			tc.malleate() // malleate mutates test data

			msgSrv := keeper.NewMsgServerImpl(*suite.GetICAApp(suite.chainA).AppKeepers.ICAAuthKeeper)
			msg := types.NewMsgRegisterAccount(owner, path.EndpointA.ConnectionID, path.EndpointB.ConnectionID)

			res, err := msgSrv.RegisterAccount(sdk.WrapSDKContext(suite.chainA.GetContext()), msg)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(res)
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(res)
			}
		})
	}
}
