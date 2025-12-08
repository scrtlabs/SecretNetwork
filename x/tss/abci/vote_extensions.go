package abci

import (
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/scrtlabs/SecretNetwork/x/tss/keeper"
	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// VoteExtensionHandler handles TSS vote extensions
// This allows validators to include DKG/signing data in their consensus votes
type VoteExtensionHandler struct {
	keeper *keeper.Keeper
	logger log.Logger
}

// NewVoteExtensionHandler creates a new vote extension handler
func NewVoteExtensionHandler(k *keeper.Keeper, logger log.Logger) *VoteExtensionHandler {
	return &VoteExtensionHandler{
		keeper: k,
		logger: logger,
	}
}

// TSSVoteExtension contains all TSS data a validator wants to submit
type TSSVoteExtension struct {
	// DKG Round 1 data for active sessions
	DKGRound1 []DKGRound1Data `json:"dkg_round1,omitempty"`

	// DKG Round 2 data for active sessions
	DKGRound2 []DKGRound2Data `json:"dkg_round2,omitempty"`

	// DKG Key Submissions - encrypted key shares for on-chain storage
	DKGKeySubmissions []DKGKeySubmissionData `json:"dkg_key_submissions,omitempty"`

	// Signing commitments for active signing requests
	SigningCommitments []SigningCommitmentData `json:"signing_commitments,omitempty"`

	// Signature shares for active signing requests
	SignatureShares []SignatureShareData `json:"signature_shares,omitempty"`
}

// DKGRound1Data represents a validator's DKG Round 1 submission
type DKGRound1Data struct {
	SessionID  string `json:"session_id"`
	Commitment []byte `json:"commitment"`
}

// DKGRound2Data represents a validator's DKG Round 2 submission
type DKGRound2Data struct {
	SessionID string `json:"session_id"`
	Share     []byte `json:"share"`
}

// DKGKeySubmissionData represents a validator's encrypted key share submission
type DKGKeySubmissionData struct {
	SessionID             string `json:"session_id"`
	EncryptedSecretShare  []byte `json:"encrypted_secret_share"`
	EncryptedPublicShares []byte `json:"encrypted_public_shares"`
	EphemeralPubKey       []byte `json:"ephemeral_pubkey"`
}

// SigningCommitmentData represents a validator's signing commitment
type SigningCommitmentData struct {
	RequestID  string `json:"request_id"`
	Commitment []byte `json:"commitment"`
}

// SignatureShareData represents a validator's signature share
type SignatureShareData struct {
	RequestID string `json:"request_id"`
	Share     []byte `json:"share"`
}

// ExtendVote allows a validator to include TSS data in their vote
// This is called before the validator signs their vote
func (h *VoteExtensionHandler) ExtendVote(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
	fmt.Printf("TSS ExtendVote: height=%d\n", req.Height)

	// Get validator's consensus address
	validatorAddr, err := h.keeper.GetValidatorAddress(ctx)
	fmt.Printf("TSS ExtendVote: validatorAddr=%q, err=%v\n", validatorAddr, err)
	if err != nil || validatorAddr == "" {
		// Not a validator or address not available yet
		fmt.Printf("TSS ExtendVote: no validator address, returning empty\n")
		return &abci.ResponseExtendVote{VoteExtension: []byte{}}, nil
	}

	// Collect all TSS data this validator needs to submit
	ext := TSSVoteExtension{}

	// Debug: only log active sessions (skip completed/failed)
	h.keeper.DKGSessionStore.Walk(ctx, nil, func(sessionID string, session types.DKGSession) (bool, error) {
		if session.State != types.DKGState_DKG_STATE_COMPLETE && session.State != types.DKGState_DKG_STATE_FAILED {
			fmt.Printf("TSS ExtendVote: active DKG session=%s state=%v\n", sessionID, session.State)
		}
		return false, nil
	})
	h.keeper.SigningRequestStore.Walk(ctx, nil, func(requestID string, request types.SigningRequest) (bool, error) {
		if request.Status != types.SigningRequestStatus_SIGNING_REQUEST_STATUS_COMPLETE &&
			request.Status != types.SigningRequestStatus_SIGNING_REQUEST_STATUS_FAILED {
			fmt.Printf("TSS ExtendVote: active signing request=%s status=%v\n", requestID, request.Status)
		}
		return false, nil
	})

	// Check for DKG Round 1 data to submit
	if err := h.keeper.DKGSessionStore.Walk(ctx, nil, func(sessionID string, session types.DKGSession) (bool, error) {
		if session.State == types.DKGState_DKG_STATE_ROUND1 && h.isParticipant(validatorAddr, session.Participants) {
			// Check if already submitted
			key := fmt.Sprintf("%s:%s", sessionID, validatorAddr)
			has, _ := h.keeper.DKGRound1DataStore.Has(ctx, key)
			if !has {
				// Generate DKG Round 1 data
				commitment := h.keeper.GenerateDKGRound1Data(ctx, sessionID, validatorAddr)
				ext.DKGRound1 = append(ext.DKGRound1, DKGRound1Data{
					SessionID:  sessionID,
					Commitment: commitment,
				})
			}
		}
		return false, nil
	}); err != nil {
		h.logger.Error("Error collecting DKG Round 1 data", "error", err)
	}

	// Check for DKG Round 2 data to submit
	if err := h.keeper.DKGSessionStore.Walk(ctx, nil, func(sessionID string, session types.DKGSession) (bool, error) {
		if session.State == types.DKGState_DKG_STATE_ROUND2 && h.isParticipant(validatorAddr, session.Participants) {
			key := fmt.Sprintf("%s:%s", sessionID, validatorAddr)
			has, _ := h.keeper.DKGRound2DataStore.Has(ctx, key)
			if !has {
				share := h.keeper.GenerateDKGRound2Data(ctx, sessionID, validatorAddr)
				ext.DKGRound2 = append(ext.DKGRound2, DKGRound2Data{
					SessionID: sessionID,
					Share:     share,
				})
			}
		}
		return false, nil
	}); err != nil {
		h.logger.Error("Error collecting DKG Round 2 data", "error", err)
	}

	// Check for DKG Key Submission data (encrypted key shares for on-chain storage)
	if err := h.keeper.DKGSessionStore.Walk(ctx, nil, func(sessionID string, session types.DKGSession) (bool, error) {
		if session.State == types.DKGState_DKG_STATE_KEY_SUBMISSION && h.isParticipant(validatorAddr, session.Participants) {
			key := fmt.Sprintf("%s:%s", sessionID, validatorAddr)
			has, _ := h.keeper.DKGKeySubmissionStore.Has(ctx, key)
			if !has {
				// Generate encrypted key share submission
				encSecretShare, encPublicShares, ephemeralPubKey, err := h.keeper.GenerateEncryptedKeySubmission(ctx, sessionID, validatorAddr)
				if err != nil {
					h.logger.Error("Failed to generate encrypted key submission",
						"session_id", sessionID, "error", err)
					return false, nil
				}
				ext.DKGKeySubmissions = append(ext.DKGKeySubmissions, DKGKeySubmissionData{
					SessionID:             sessionID,
					EncryptedSecretShare:  encSecretShare,
					EncryptedPublicShares: encPublicShares,
					EphemeralPubKey:       ephemeralPubKey,
				})
				h.logger.Info("Generated encrypted key submission for on-chain storage",
					"session_id", sessionID)
			}
		}
		return false, nil
	}); err != nil {
		h.logger.Error("Error collecting DKG key submissions", "error", err)
	}

	// Check for signing commitments to submit
	if err := h.keeper.SigningRequestStore.Walk(ctx, nil, func(requestID string, request types.SigningRequest) (bool, error) {
		if request.Status == types.SigningRequestStatus_SIGNING_REQUEST_STATUS_ROUND1 {
			// Get session to check if validator is a participant
			session, err := h.keeper.SigningSessionStore.Get(ctx, requestID)
			if err != nil {
				return false, nil
			}

			if h.isParticipant(validatorAddr, session.Participants) {
				key := fmt.Sprintf("%s:%s", requestID, validatorAddr)
				has, _ := h.keeper.SigningCommitmentStore.Has(ctx, key)
				if !has {
					commitment := h.keeper.GenerateSigningCommitment(ctx, requestID, validatorAddr)
					ext.SigningCommitments = append(ext.SigningCommitments, SigningCommitmentData{
						RequestID:  requestID,
						Commitment: commitment,
					})
				}
			}
		}
		return false, nil
	}); err != nil {
		h.logger.Error("Error collecting signing commitments", "error", err)
	}

	// Check for signature shares to submit
	if err := h.keeper.SigningRequestStore.Walk(ctx, nil, func(requestID string, request types.SigningRequest) (bool, error) {
		if request.Status == types.SigningRequestStatus_SIGNING_REQUEST_STATUS_ROUND2 {
			// Get session to check if validator is a participant
			session, err := h.keeper.SigningSessionStore.Get(ctx, requestID)
			if err != nil {
				return false, nil
			}

			if h.isParticipant(validatorAddr, session.Participants) {
				key := fmt.Sprintf("%s:%s", requestID, validatorAddr)
				has, _ := h.keeper.SignatureShareStore.Has(ctx, key)
				if !has {
					share := h.keeper.GenerateSignatureShare(ctx, requestID, validatorAddr)
					ext.SignatureShares = append(ext.SignatureShares, SignatureShareData{
						RequestID: requestID,
						Share:     share,
					})
				}
			}
		}
		return false, nil
	}); err != nil {
		h.logger.Error("Error collecting signature shares", "error", err)
	}

	// Encode the extension
	extBytes, err := json.Marshal(ext)
	if err != nil {
		h.logger.Error("Failed to marshal vote extension", "error", err)
		return &abci.ResponseExtendVote{VoteExtension: []byte{}}, nil
	}

	h.logger.Info("Extended vote with TSS data",
		"dkg_r1", len(ext.DKGRound1),
		"dkg_r2", len(ext.DKGRound2),
		"dkg_key_submissions", len(ext.DKGKeySubmissions),
		"commitments", len(ext.SigningCommitments),
		"shares", len(ext.SignatureShares))

	return &abci.ResponseExtendVote{VoteExtension: extBytes}, nil
}

// VerifyVoteExtension verifies TSS data from another validator's vote extension
// This is called when a validator receives a vote from another validator
func (h *VoteExtensionHandler) VerifyVoteExtension(ctx sdk.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	// If extension is empty, accept it (validator had nothing to submit)
	if len(req.VoteExtension) == 0 {
		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}

	// Decode the extension
	var ext TSSVoteExtension
	if err := json.Unmarshal(req.VoteExtension, &ext); err != nil {
		h.logger.Error("Failed to unmarshal vote extension", "error", err)
		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, nil
	}

	// TODO: Add validation logic:
	// 1. Verify validator is actually a participant in the sessions they're submitting for
	// 2. Verify cryptographic validity of commitments/shares
	// 3. Verify no duplicate submissions
	//
	// For now, we accept all extensions to get the basic flow working

	h.logger.Debug("Verified vote extension",
		"height", req.Height,
		"dkg_r1", len(ext.DKGRound1),
		"dkg_r2", len(ext.DKGRound2))

	return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
}

// isParticipant checks if a validator is in the participant list
func (h *VoteExtensionHandler) isParticipant(validatorAddr string, participants []string) bool {
	for _, p := range participants {
		if p == validatorAddr {
			return true
		}
	}
	return false
}
