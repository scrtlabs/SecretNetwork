package keeper_test

import (
	"encoding/json"
	"os"
	"testing"

	// "github.com/cosmos/cosmos-sdk/simapp"
	compute "github.com/scrtlabs/SecretNetwork/x/compute"

	"github.com/stretchr/testify/suite"
	//"github.com/cometbft/cometbft/crypto"
	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"

	// sdk "github.com/cosmos/cosmos-sdk/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"

	icaapp "github.com/scrtlabs/SecretNetwork/app"
)

var (
	// TestAccAddress defines a resuable bech32 address for testing purposes
	// TODO: update crypto.AddressHash() when sdk uses address.Module()
	// TestAccAddress = icatypes.GenerateAddress(sdk.AccAddress(crypto.AddressHash([]byte(icatypes.ModuleName))), ibctesting.FirstConnectionID, TestPortID)
	// TestOwnerAddress defines a reusable bech32 address for testing purposes
	TestOwnerAddress = "cosmos17dtl0mjt3t77kpuhg2edqzjpszulwhgzuj9ljs"
	// TestPortID defines a resuable port identifier for testing purposes
	TestPortID, _ = icatypes.NewControllerPortID(TestOwnerAddress)
	// TestVersion defines a resuable interchainaccounts version string for testing purposes
	TestVersion = string(icatypes.ModuleCdc.MustMarshalJSON(&icatypes.Metadata{
		Version:                icatypes.Version,
		ControllerConnectionId: ibctesting.FirstConnectionID,
		HostConnectionId:       ibctesting.FirstConnectionID,
	}))
)

func init() {
	ibctesting.DefaultTestingAppInit = SetupICATestingApp
}

func SetupICATestingApp() (ibctesting.TestingApp, map[string]json.RawMessage) {
	db := dbm.NewMemDB()
	// encCdc := icaapp.MakeEncodingConfig()

	tempDir := func() string {
		dir, err := os.MkdirTemp("", "secretd")
		if err != nil {
			dir = icaapp.DefaultNodeHome
		}
		defer os.RemoveAll(dir)

		return dir
	}
	// NewAppOptionsWithFlagHome(tempDir())
	app := icaapp.NewSecretNetworkApp(log.NewNopLogger(), db, nil, true, true, simtestutil.NewAppOptionsWithFlagHome(tempDir()), compute.DefaultWasmConfig())

	// app :=  icaapp.NewSecretNetworkApp(log.NewNopLogger(), db, nil, true, true, icaapp.DefaultNodeHome, 5, false, simapp.EmptyAppOptions{}, compute.DefaultWasmConfig())
	// TODO: figure out if it's ok that w MakeEncodingConfig inside of our Genesis.go. It would be a different instance than the one used in app
	return app, icaapp.NewDefaultGenesisState()
}

// KeeperTestSuite is a testing suite to test keeper functions
type KeeperTestSuite struct {
	suite.Suite

	coordinator *ibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA *ibctesting.TestChain
	chainB *ibctesting.TestChain
}

func (suite *KeeperTestSuite) GetICAApp(chain *ibctesting.TestChain) *icaapp.SecretNetworkApp {
	app, ok := chain.App.(*icaapp.SecretNetworkApp)
	if !ok {
		panic("not ica app")
	}

	return app
}

// TestKeeperTestSuite runs all the tests within this package.
func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

// SetupTest creates a coordinator with 2 test chains.
func (suite *KeeperTestSuite) SetupTest() {
	suite.coordinator = ibctesting.NewCoordinator(suite.T(), 2)
	suite.chainA = suite.coordinator.GetChain(ibctesting.GetChainID(0))
	suite.chainB = suite.coordinator.GetChain(ibctesting.GetChainID(1))
}

func NewICAPath(chainA, chainB *ibctesting.TestChain) *ibctesting.Path {
	path := ibctesting.NewPath(chainA, chainB)
	path.EndpointA.ChannelConfig.PortID = icatypes.HostPortID
	path.EndpointB.ChannelConfig.PortID = icatypes.HostPortID
	path.EndpointA.ChannelConfig.Order = channeltypes.ORDERED
	path.EndpointB.ChannelConfig.Order = channeltypes.ORDERED
	path.EndpointA.ChannelConfig.Version = TestVersion
	path.EndpointB.ChannelConfig.Version = TestVersion

	return path
}
