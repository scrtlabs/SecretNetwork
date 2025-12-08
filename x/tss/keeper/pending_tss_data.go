package keeper

import (
	"context"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AggregatedTSSData contains all TSS data from vote extensions
type AggregatedTSSData struct {
	DKGRound1          map[string]map[string][]byte             `json:"dkg_round1"`
	DKGRound2          map[string]map[string][]byte             `json:"dkg_round2"`
	DKGKeySubmissions  map[string]map[string]*DKGKeySubmission  `json:"dkg_key_submissions"`
	SigningCommitments map[string]map[string][]byte             `json:"signing_commitments"`
	SignatureShares    map[string]map[string][]byte             `json:"signature_shares"`
}

// DKGKeySubmission contains encrypted key share data for aggregation
type DKGKeySubmission struct {
	EncryptedSecretShare  []byte `json:"encrypted_secret_share"`
	EncryptedPublicShares []byte `json:"encrypted_public_shares"`
	EphemeralPubKey       []byte `json:"ephemeral_pubkey"`
}

// In-memory storage for TSS data between ProcessProposal and BeginBlock
// Safe because ProcessProposal and BeginBlock run sequentially on the same node
var (
	pendingTSSData *AggregatedTSSData
	tssDataMutex   sync.RWMutex
)

// StorePendingTSSData stores aggregated TSS data for BeginBlock to process
// Called by ProposalHandler during PrepareProposal/ProcessProposal
func (k Keeper) StorePendingTSSData(data *AggregatedTSSData) {
	tssDataMutex.Lock()
	defer tssDataMutex.Unlock()
	pendingTSSData = data
}

// ProcessPendingTSSData retrieves and processes TSS data stored during proposal handling
// Called during BeginBlock
func (k Keeper) ProcessPendingTSSData(ctx context.Context) error {
	tssDataMutex.Lock()
	defer tssDataMutex.Unlock()

	// No pending TSS data - normal for blocks without TSS activity
	if pendingTSSData == nil {
		return nil
	}

	// Get data and clear it
	data := pendingTSSData
	pendingTSSData = nil

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	logger := sdkCtx.Logger().With("module", "tss", "phase", "begin_block")

	// Track counts for logging
	var dkgR1Count, dkgR2Count, dkgKeySubCount, sigCommitCount, sigShareCount int

	// Process DKG Round 1 data
	for sessionID, validators := range data.DKGRound1 {
		for validatorAddr, commitment := range validators {
			if err := k.ProcessDKGRound1(ctx, sessionID, validatorAddr, commitment); err != nil {
				logger.Debug("Failed to process DKG Round 1",
					"session", sessionID,
					"validator", validatorAddr,
					"error", err)
			} else {
				dkgR1Count++
			}
		}
	}

	// Process DKG Round 2 data
	for sessionID, validators := range data.DKGRound2 {
		for validatorAddr, share := range validators {
			if err := k.ProcessDKGRound2(ctx, sessionID, validatorAddr, share); err != nil {
				logger.Debug("Failed to process DKG Round 2",
					"session", sessionID,
					"validator", validatorAddr,
					"error", err)
			} else {
				dkgR2Count++
			}
		}
	}

	// Process DKG Key Submissions (encrypted key shares for on-chain storage)
	for sessionID, validators := range data.DKGKeySubmissions {
		for validatorAddr, submission := range validators {
			if err := k.ProcessDKGKeySubmission(ctx, sessionID, validatorAddr,
				submission.EncryptedSecretShare, submission.EncryptedPublicShares, submission.EphemeralPubKey); err != nil {
				logger.Debug("Failed to process DKG key submission",
					"session", sessionID,
					"validator", validatorAddr,
					"error", err)
			} else {
				dkgKeySubCount++
				logger.Info("Stored encrypted key submission on-chain",
					"session", sessionID,
					"validator", validatorAddr)
			}
		}
	}

	// Process signing commitments
	for requestID, validators := range data.SigningCommitments {
		for validatorAddr, commitment := range validators {
			if err := k.ProcessSigningCommitment(ctx, requestID, validatorAddr, commitment); err != nil {
				logger.Debug("Failed to process signing commitment",
					"request", requestID,
					"validator", validatorAddr,
					"error", err)
			} else {
				sigCommitCount++
			}
		}
	}

	// Process signature shares
	for requestID, validators := range data.SignatureShares {
		for validatorAddr, share := range validators {
			if err := k.ProcessSignatureShare(ctx, requestID, validatorAddr, share); err != nil {
				logger.Debug("Failed to process signature share",
					"request", requestID,
					"validator", validatorAddr,
					"error", err)
			} else {
				sigShareCount++
			}
		}
	}

	// Log summary if there was any TSS activity
	if dkgR1Count > 0 || dkgR2Count > 0 || dkgKeySubCount > 0 || sigCommitCount > 0 || sigShareCount > 0 {
		logger.Info("Processed TSS data from vote extensions",
			"height", sdkCtx.BlockHeight(),
			"dkg_r1", dkgR1Count,
			"dkg_r2", dkgR2Count,
			"dkg_key_submissions", dkgKeySubCount,
			"signing_commitments", sigCommitCount,
			"signature_shares", sigShareCount)
	}

	return nil
}
