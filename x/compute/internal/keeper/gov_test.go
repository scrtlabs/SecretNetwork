package keeper

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"

	wasmTypes "github.com/scrtlabs/SecretNetwork/x/compute/internal/types"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/gov/types"
)

var TestProposal = types.NewTextProposal("Test", "description")

type GovInitMsg struct{}

type GovExecMsgVote struct {
	Vote GovInitMsg `json:"vote"`
}

type GovExecMsg struct {
	Proposals GovInitMsg `json:"proposals"`
}

// ProposalEqual checks if two proposals are equal (note: slow, for tests only)
func ProposalEqual(proposalA types.Proposal, proposalB types.Proposal) bool {
	return bytes.Equal(types.ModuleCdc.MustMarshal(&proposalA),
		types.ModuleCdc.MustMarshal(&proposalB))
}

// TestGovQueryProposals tests reading how many proposals are active - first testing 0 proposals, then adding
// an active proposal and checking that there is 1 active
func TestGovQueryProposals(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource wasmTypes.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, _, keeper, govKeeper := keepers.AccountKeeper, keepers.StakingKeeper, keepers.WasmKeeper, keepers.GovKeeper

	govKeeper.SetProposalID(ctx, types.DefaultStartingProposalID)
	govKeeper.SetDepositParams(ctx, types.DefaultDepositParams())
	govKeeper.SetVotingParams(ctx, types.DefaultVotingParams())
	govKeeper.SetTallyParams(ctx, types.DefaultTallyParams())

	deposit := sdk.NewCoins(sdk.NewInt64Coin("stake", 5_000_000_000))
	creator, creatorPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)
	//

	// upload staking derivates code
	govCode, err := os.ReadFile("./testdata/gov.wasm")
	require.NoError(t, err)
	govId, err := keeper.Create(ctx, creator, govCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), govId)

	// register to a valid address
	initMsg := GovInitMsg{}
	initBz, err := json.Marshal(&initMsg)
	require.NoError(t, err)
	initBz, err = testEncrypt(t, keeper, ctx, nil, govId, initBz)
	require.NoError(t, err)

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, creatorPrivKey, initBz, govId, nil)
	govAddr, _, err := keeper.Instantiate(ctx, govId, creator, nil, initBz, "gidi gov", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, govAddr)

	queryReq := GovExecMsg{}
	govQBz, err := json.Marshal(&queryReq)
	require.NoError(t, err)

	_, _, res, _, _, err := execHelper(t, keeper, ctx, govAddr, creator, creatorPrivKey, string(govQBz), false, false, defaultGasForTests, 0)
	require.Empty(t, err)

	require.Equal(t, uint64(0), binary.BigEndian.Uint64(res))

	tp := TestProposal
	// check that gov is working
	proposal, err := govKeeper.SubmitProposal(ctx, tp, false)
	require.NoError(t, err)
	proposalID := proposal.ProposalId
	gotProposal, ok := govKeeper.GetProposal(ctx, proposalID)
	require.True(t, ok)
	require.True(t, ProposalEqual(proposal, gotProposal))

	votingStarted, err := govKeeper.AddDeposit(ctx, proposalID, creator, deposit)
	require.NoError(t, err)
	require.True(t, votingStarted)

	_, _, res, _, _, err = execHelper(t, keeper, ctx, govAddr, creator, creatorPrivKey, string(govQBz), false, false, defaultGasForTests, 0)
	require.Empty(t, err)
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(res))
}

// TestGovQueryProposals tests reading how many proposals are active - first testing 0 proposals, then adding
// an active proposal and checking that there is 1 active
func TestGovVote(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	transferPortSource := MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, _, keeper, govKeeper := keepers.AccountKeeper, keepers.StakingKeeper, keepers.WasmKeeper, keepers.GovKeeper

	govKeeper.SetProposalID(ctx, types.DefaultStartingProposalID)
	govKeeper.SetDepositParams(ctx, types.DefaultDepositParams())
	govKeeper.SetVotingParams(ctx, types.DefaultVotingParams())
	govKeeper.SetTallyParams(ctx, types.DefaultTallyParams())

	deposit2 := sdk.NewCoins(sdk.NewInt64Coin("stake", 5_000_000_000))
	deposit := sdk.NewCoins(sdk.NewInt64Coin("stake", 5_000_000_000))
	initFunds := sdk.NewCoins(sdk.NewInt64Coin("stake", 10_000_000_000))
	creator, creatorPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, initFunds)
	//

	// upload staking derivates code
	govCode, err := os.ReadFile("./testdata/gov.wasm")
	require.NoError(t, err)
	govId, err := keeper.Create(ctx, creator, govCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), govId)

	// register to a valid address
	initMsg := GovInitMsg{}
	initBz, err := json.Marshal(&initMsg)
	require.NoError(t, err)
	initBz, err = testEncrypt(t, keeper, ctx, nil, govId, initBz)
	require.NoError(t, err)

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, creatorPrivKey, initBz, govId, deposit2)
	govAddr, _, err := keeper.Instantiate(ctx, govId, creator, nil, initBz, "gidi gov", deposit2, nil)
	require.NoError(t, err)
	require.NotEmpty(t, govAddr)

	queryReq := GovExecMsgVote{}
	govQBz, err := json.Marshal(&queryReq)
	require.NoError(t, err)

	// check that gov is working
	proposal, err := govKeeper.SubmitProposal(ctx, TestProposal, false)
	require.NoError(t, err)
	proposalID := proposal.ProposalId
	gotProposal, ok := govKeeper.GetProposal(ctx, proposalID)
	require.True(t, ok)
	require.True(t, ProposalEqual(proposal, gotProposal))

	_, _, _, _, _, err = execHelper(t, keeper, ctx, govAddr, creator, creatorPrivKey, string(govQBz), false, false, defaultGasForTests, 0)
	require.NotEmpty(t, err)
	require.Equal(t, "encrypted: dispatch: submessages: 1: inactive proposal", err.Error())

	votingStarted, err := govKeeper.AddDeposit(ctx, proposalID, creator, deposit)
	require.NoError(t, err)
	require.True(t, votingStarted)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, govAddr, creator, creatorPrivKey, string(govQBz), false, false, defaultGasForTests, 0)
	require.Empty(t, err)

	votes := govKeeper.GetAllVotes(ctx)
	require.Equal(t, uint64(0x1), votes[0].ProposalId)
	require.Equal(t, govAddr.String(), votes[0].Voter)
	require.Equal(t, types.OptionYes, votes[0].Option)
}
