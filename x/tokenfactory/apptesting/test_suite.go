package apptesting

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/baseapp"
	authzcodec "github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/scrtlabs/SecretNetwork/x/compute"

	"github.com/scrtlabs/SecretNetwork/app"
)

type KeeperTestHelper struct {
	suite.Suite

	App         *app.SecretNetworkApp
	Ctx         sdk.Context
	QueryHelper *baseapp.QueryServiceTestHelper
	TestAccs    []sdk.AccAddress
}

var (
	SecondaryDenom  = "uion"
	SecondaryAmount = sdk.NewInt(100000000)
)

// Setup initializes a new SecretNetworkApp.
func Setup2(isCheckTx bool) *app.SecretNetworkApp {
	db := dbm.NewMemDB()
	// app :=     NewSecretNetworkApp(log.NewNopLogger(), db, nil, true, map[int64]bool{}, app.DefaultNodeHome, 5, false, simapp.EmptyAppOptions{}, GetWasmEnabledProposals())
	app := app.NewSecretNetworkApp(log.NewNopLogger(), db, nil, true, map[int64]bool{}, app.DefaultNodeHome, 5, false, simapp.EmptyAppOptions{}, compute.DefaultWasmConfig())

	if !isCheckTx {
		stateBytes := getDefaultGenesisStateBytes()

		app.InitChain(
			abci.RequestInitChain{
				Validators:      []abci.ValidatorUpdate{},
				ConsensusParams: simapp.DefaultConsensusParams,
				AppStateBytes:   stateBytes,
			},
		)
	}

	return app
}

// Setup sets up basic environment for suite (App, Ctx, and test accounts)
func (s *KeeperTestHelper) Setup() {
	// s.App = app.Setup(false)
	s.App = Setup2(false)
	s.Ctx = s.App.BaseApp.NewContext(false, tmtypes.Header{Height: 1, ChainID: "osmosis-1", Time: time.Now().UTC()})
	s.QueryHelper = &baseapp.QueryServiceTestHelper{
		GRPCQueryRouter: s.App.GRPCQueryRouter(),
		Ctx:             s.Ctx,
	}

	// s.SetEpochStartTime()
	s.TestAccs = CreateRandomAccounts(3)
}

func (s *KeeperTestHelper) SetupTestForInitGenesis() {
	// Setting to True, leads to init genesis not running
	s.App = Setup2(true)
	s.Ctx = s.App.BaseApp.NewContext(true, tmtypes.Header{})
}

// CreateTestContext creates a test context.
func (s *KeeperTestHelper) Commit() {
	oldHeight := s.Ctx.BlockHeight()
	oldHeader := s.Ctx.BlockHeader()
	s.App.Commit()
	newHeader := tmtypes.Header{Height: oldHeight + 1, ChainID: oldHeader.ChainID, Time: oldHeader.Time.Add(time.Second)}
	s.App.BeginBlock(abci.RequestBeginBlock{Header: newHeader})
	s.Ctx = s.App.GetBaseApp().NewContext(false, newHeader)
}

// FundAcc funds target address with specified amount.
func (s *KeeperTestHelper) FundAcc(acc sdk.AccAddress, amounts sdk.Coins) {
	err := simapp.FundAccount(s.App.AppKeepers.BankKeeper, s.Ctx, acc, amounts)
	s.Require().NoError(err)
}

// StateNotAltered validates that app state is not altered. Fails if it is.
func (s *KeeperTestHelper) StateNotAltered() {
	oldState := s.App.ExportState(s.Ctx)
	s.Commit()
	newState := s.App.ExportState(s.Ctx)
	s.Require().Equal(oldState, newState)
}

// // CreateRandomAccounts is a function return a list of randomly generated AccAddresses
func CreateRandomAccounts(numAccts int) []sdk.AccAddress {
	testAddrs := make([]sdk.AccAddress, numAccts)
	for i := 0; i < numAccts; i++ {
		pk := ed25519.GenPrivKey().PubKey()
		testAddrs[i] = sdk.AccAddress(pk.Address())
	}

	return testAddrs
}

func TestMessageAuthzSerialization(t *testing.T, msg sdk.Msg) {
	someDate := time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)
	const (
		mockGranter string = "cosmos1abc"
		mockGrantee string = "cosmos1xyz"
	)

	var (
		mockMsgGrant  authz.MsgGrant
		mockMsgRevoke authz.MsgRevoke
		mockMsgExec   authz.MsgExec
	)
	amino := authzcodec.NewLegacyAmino()

	// ModuleCdc references the global x/wasm module codec.
	ModuleCdc := authzcodec.NewAminoCodec(amino)

	// Authz: Grant Msg
	typeURL := sdk.MsgTypeURL(msg)
	// grant, err := authz.NewGrant(someDate, authz.NewGenericAuthorization(typeURL), someDate.Add(time.Hour))
	grant, err := authz.NewGrant(authz.NewGenericAuthorization(typeURL), someDate.Add(time.Hour))
	require.NoError(t, err)

	msgGrant := authz.MsgGrant{Granter: mockGranter, Grantee: mockGrantee, Grant: grant}
	msgGrantBytes := json.RawMessage(sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msgGrant)))
	err = ModuleCdc.UnmarshalJSON(msgGrantBytes, &mockMsgGrant)
	require.NoError(t, err)

	// Authz: Revoke Msg
	msgRevoke := authz.MsgRevoke{Granter: mockGranter, Grantee: mockGrantee, MsgTypeUrl: typeURL}
	msgRevokeByte := json.RawMessage(sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msgRevoke)))
	err = ModuleCdc.UnmarshalJSON(msgRevokeByte, &mockMsgRevoke)
	require.NoError(t, err)

	// Authz: Exec Msg
	msgAny, err := cdctypes.NewAnyWithValue(msg)
	require.NoError(t, err)
	msgExec := authz.MsgExec{Grantee: mockGrantee, Msgs: []*cdctypes.Any{msgAny}}
	execMsgByte := json.RawMessage(sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msgExec)))
	err = ModuleCdc.UnmarshalJSON(execMsgByte, &mockMsgExec)
	require.NoError(t, err)
	require.Equal(t, msgExec.Msgs[0].Value, mockMsgExec.Msgs[0].Value)
}
