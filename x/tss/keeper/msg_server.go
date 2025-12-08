package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// ========================
// DKG Messages (from x/mpc)
// ========================

// CreateKeySet creates a new KeySet and initiates a DKG ceremony for it
func (ms msgServer) CreateKeySet(ctx context.Context, msg *types.MsgCreateKeySet) (*types.MsgCreateKeySetResponse, error) {
	// Validate parameters
	if msg.Threshold == 0 || msg.MaxSigners == 0 {
		return nil, types.ErrInvalidThreshold
	}
	if msg.Threshold > msg.MaxSigners {
		return nil, types.ErrInvalidThreshold
	}

	// Create the KeySet using the keeper method
	keySetID, err := ms.Keeper.CreateKeySet(ctx, msg.Creator, msg.Threshold, msg.MaxSigners, msg.Description)
	if err != nil {
		return nil, err
	}

	// Initiate DKG ceremony for this KeySet
	dkgSessionID, err := ms.Keeper.InitiateDKGForKeySet(ctx, keySetID, msg.Threshold, msg.MaxSigners, msg.TimeoutBlocks)
	if err != nil {
		return nil, err
	}

	// TODO: Emit event for KeySet creation

	return &types.MsgCreateKeySetResponse{
		KeySetId:     keySetID,
		DkgSessionId: dkgSessionID,
	}, nil
}

// InitiateDKG initiates a new DKG ceremony
func (ms msgServer) InitiateDKG(ctx context.Context, msg *types.MsgInitiateDKG) (*types.MsgInitiateDKGResponse, error) {
	// TODO: Implement DKG initiation logic
	return &types.MsgInitiateDKGResponse{
		SessionId: "dkg-session-1",
	}, nil
}

// SubmitDKGRound1 submits a validator's round 1 commitment
func (ms msgServer) SubmitDKGRound1(ctx context.Context, msg *types.MsgSubmitDKGRound1) (*types.MsgSubmitDKGRound1Response, error) {
	// SECURITY: Verify the transaction has a valid signer
	// This prevents unsigned or malicious transactions
	signers := msg.GetSigners()
	if len(signers) == 0 {
		return nil, fmt.Errorf("no signers found in transaction")
	}

	// TODO: Add proper validator address verification
	// Need to map between account addresses (transaction signers) and consensus addresses (msg.Validator)
	// For now, we verify that a valid signature exists
	// In production, should verify: staking.GetValidatorByConsAddr(msg.Validator).OperatorAddress == signers[0]
	signerAddr := signers[0].String()
	_ = signerAddr // Use the signer address for logging/debugging

	sdk.UnwrapSDKContext(ctx).Logger().Info("DKG Round 1 submission",
		"validator", msg.Validator,
		"signer", signerAddr,
		"session", msg.SessionId)

	// Process the Round 1 commitment
	err := ms.Keeper.ProcessDKGRound1(ctx, msg.SessionId, msg.Validator, msg.Commitment)
	if err != nil {
		return nil, err
	}

	// TODO: Emit event for Round 1 submission

	return &types.MsgSubmitDKGRound1Response{}, nil
}

// SubmitDKGRound2 submits a validator's round 2 share
func (ms msgServer) SubmitDKGRound2(ctx context.Context, msg *types.MsgSubmitDKGRound2) (*types.MsgSubmitDKGRound2Response, error) {
	// SECURITY: Verify the transaction has a valid signer
	// This prevents unsigned or malicious transactions
	signers := msg.GetSigners()
	if len(signers) == 0 {
		return nil, fmt.Errorf("no signers found in transaction")
	}

	// TODO: Add proper validator address verification (see SubmitDKGRound1 comments)
	signerAddr := signers[0].String()
	_ = signerAddr

	sdk.UnwrapSDKContext(ctx).Logger().Info("DKG Round 2 submission",
		"validator", msg.Validator,
		"signer", signerAddr,
		"session", msg.SessionId)

	// Process the Round 2 share
	err := ms.Keeper.ProcessDKGRound2(ctx, msg.SessionId, msg.Validator, msg.Share)
	if err != nil {
		return nil, err
	}

	// TODO: Emit event for Round 2 submission

	return &types.MsgSubmitDKGRound2Response{}, nil
}

// ========================
// Signing Messages (from x/signing)
// ========================

// RequestSignature creates a new signing request
func (ms msgServer) RequestSignature(ctx context.Context, msg *types.MsgRequestSignature) (*types.MsgRequestSignatureResponse, error) {
	// Validate that the KeySet exists
	keySet, err := ms.Keeper.GetKeySet(ctx, msg.KeySetId)
	if err != nil {
		return nil, types.ErrKeySetNotFound
	}

	// Verify the requester is the KeySet owner
	if keySet.Owner != msg.Requester {
		return nil, types.ErrUnauthorizedKeySet
	}

	// Create the signing request
	requestID, err := ms.Keeper.CreateSigningRequest(ctx, msg.KeySetId, msg.Requester, msg.MessageHash, msg.Callback)
	if err != nil {
		return nil, err
	}

	// TODO: Emit event for signing request creation

	return &types.MsgRequestSignatureResponse{
		RequestId: requestID,
	}, nil
}

// SubmitCommitment submits a signing commitment (Round 1)
func (ms msgServer) SubmitCommitment(ctx context.Context, msg *types.MsgSubmitCommitment) (*types.MsgSubmitCommitmentResponse, error) {
	// SECURITY: Verify the transaction has a valid signer
	// This prevents unsigned or malicious transactions
	signers := msg.GetSigners()
	if len(signers) == 0 {
		return nil, fmt.Errorf("no signers found in transaction")
	}

	// TODO: Add proper validator address verification (see SubmitDKGRound1 comments)
	signerAddr := signers[0].String()
	_ = signerAddr

	sdk.UnwrapSDKContext(ctx).Logger().Info("Signing commitment submission",
		"validator", msg.Validator,
		"signer", signerAddr,
		"request", msg.RequestId)

	// Process the commitment
	err := ms.Keeper.ProcessSigningCommitment(ctx, msg.RequestId, msg.Validator, msg.Commitment)
	if err != nil {
		return nil, err
	}

	// TODO: Emit event for commitment submission

	return &types.MsgSubmitCommitmentResponse{}, nil
}

// SubmitSignatureShare submits a signature share (Round 2)
func (ms msgServer) SubmitSignatureShare(ctx context.Context, msg *types.MsgSubmitSignatureShare) (*types.MsgSubmitSignatureShareResponse, error) {
	// SECURITY: Verify the transaction has a valid signer
	// This prevents unsigned or malicious transactions
	signers := msg.GetSigners()
	if len(signers) == 0 {
		return nil, fmt.Errorf("no signers found in transaction")
	}

	// TODO: Add proper validator address verification (see SubmitDKGRound1 comments)
	signerAddr := signers[0].String()
	_ = signerAddr

	sdk.UnwrapSDKContext(ctx).Logger().Info("Signature share submission",
		"validator", msg.Validator,
		"signer", signerAddr,
		"request", msg.RequestId)

	// Process the signature share
	err := ms.Keeper.ProcessSignatureShare(ctx, msg.RequestId, msg.Validator, msg.Share)
	if err != nil {
		return nil, err
	}

	// TODO: Emit event for signature share submission

	return &types.MsgSubmitSignatureShareResponse{}, nil
}
