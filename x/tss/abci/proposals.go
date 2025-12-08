package abci

import (
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/scrtlabs/SecretNetwork/x/tss/keeper"
)

// TSSDataPrefix identifies TSS aggregated data in proposals
// Uses a unique prefix that won't collide with real transactions
var TSSDataPrefix = []byte("__TSS_VOTE_EXT__")

// ProposalHandler handles block proposals with TSS data
type ProposalHandler struct {
	keeper *keeper.Keeper
	logger log.Logger
}

// NewProposalHandler creates a new proposal handler
func NewProposalHandler(k *keeper.Keeper, logger log.Logger) *ProposalHandler {
	return &ProposalHandler{
		keeper: k,
		logger: logger,
	}
}

// PrepareProposal aggregates TSS data from vote extensions and injects into proposal
// Only the block proposer runs this - they have access to LocalLastCommit
func (h *ProposalHandler) PrepareProposal(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	// Aggregate TSS data from vote extensions
	aggregated := h.aggregateVoteExtensions(req.LocalLastCommit.Votes)

	// Check if there's any TSS data
	hasTSSData := len(aggregated.DKGRound1) > 0 || len(aggregated.DKGRound2) > 0 ||
		len(aggregated.DKGKeySubmissions) > 0 ||
		len(aggregated.SigningCommitments) > 0 || len(aggregated.SignatureShares) > 0

	txs := req.Txs

	if hasTSSData {
		// Serialize aggregated data
		dataBytes, err := json.Marshal(aggregated)
		if err != nil {
			h.logger.Error("Failed to marshal TSS data", "error", err)
		} else {
			// Inject as first "transaction" with prefix
			tssData := append(TSSDataPrefix, dataBytes...)
			txs = append([][]byte{tssData}, txs...)

			h.logger.Debug("PrepareProposal: injected TSS data",
				"height", req.Height,
				"dkg_r1_sessions", len(aggregated.DKGRound1),
				"dkg_r2_sessions", len(aggregated.DKGRound2),
				"signing_commits", len(aggregated.SigningCommitments),
				"sig_shares", len(aggregated.SignatureShares))
		}
	}

	return &abci.ResponsePrepareProposal{Txs: txs}, nil
}

// ProcessProposal extracts TSS data from proposal and stores for BeginBlock
// All validators run this - they extract what the proposer injected
func (h *ProposalHandler) ProcessProposal(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	// Check if first entry is TSS data (has our prefix)
	if len(req.Txs) > 0 && len(req.Txs[0]) > len(TSSDataPrefix) {
		prefix := req.Txs[0][:len(TSSDataPrefix)]
		if string(prefix) == string(TSSDataPrefix) {
			// Extract and parse TSS data
			dataBytes := req.Txs[0][len(TSSDataPrefix):]

			var aggregated keeper.AggregatedTSSData
			if err := json.Unmarshal(dataBytes, &aggregated); err != nil {
				h.logger.Error("Failed to unmarshal TSS data", "error", err)
				// Don't reject block for TSS parsing errors
			} else {
				// Store for BeginBlock to process
				h.keeper.StorePendingTSSData(&aggregated)

				h.logger.Debug("ProcessProposal: stored TSS data",
					"height", req.Height,
					"dkg_r1_sessions", len(aggregated.DKGRound1),
					"dkg_r2_sessions", len(aggregated.DKGRound2))
			}
		}
	}

	return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
}

// aggregateVoteExtensions collects TSS data from all vote extensions
func (h *ProposalHandler) aggregateVoteExtensions(votes []abci.ExtendedVoteInfo) *keeper.AggregatedTSSData {
	aggregated := &keeper.AggregatedTSSData{
		DKGRound1:          make(map[string]map[string][]byte),
		DKGRound2:          make(map[string]map[string][]byte),
		DKGKeySubmissions:  make(map[string]map[string]*keeper.DKGKeySubmission),
		SigningCommitments: make(map[string]map[string][]byte),
		SignatureShares:    make(map[string]map[string][]byte),
	}

	for _, vote := range votes {
		if len(vote.VoteExtension) == 0 {
			continue
		}

		// Get validator address from vote (hex format)
		validatorAddr := fmt.Sprintf("%x", vote.Validator.Address)

		// Decode vote extension
		var ext TSSVoteExtension
		if err := json.Unmarshal(vote.VoteExtension, &ext); err != nil {
			h.logger.Debug("Failed to unmarshal vote extension",
				"validator", validatorAddr,
				"error", err)
			continue
		}

		// Aggregate DKG Round 1 data
		for _, data := range ext.DKGRound1 {
			if aggregated.DKGRound1[data.SessionID] == nil {
				aggregated.DKGRound1[data.SessionID] = make(map[string][]byte)
			}
			aggregated.DKGRound1[data.SessionID][validatorAddr] = data.Commitment
		}

		// Aggregate DKG Round 2 data
		for _, data := range ext.DKGRound2 {
			if aggregated.DKGRound2[data.SessionID] == nil {
				aggregated.DKGRound2[data.SessionID] = make(map[string][]byte)
			}
			aggregated.DKGRound2[data.SessionID][validatorAddr] = data.Share
		}

		// Aggregate DKG Key Submissions (encrypted key shares for on-chain storage)
		for _, data := range ext.DKGKeySubmissions {
			if aggregated.DKGKeySubmissions[data.SessionID] == nil {
				aggregated.DKGKeySubmissions[data.SessionID] = make(map[string]*keeper.DKGKeySubmission)
			}
			aggregated.DKGKeySubmissions[data.SessionID][validatorAddr] = &keeper.DKGKeySubmission{
				EncryptedSecretShare:  data.EncryptedSecretShare,
				EncryptedPublicShares: data.EncryptedPublicShares,
				EphemeralPubKey:       data.EphemeralPubKey,
			}
		}

		// Aggregate signing commitments
		for _, data := range ext.SigningCommitments {
			if aggregated.SigningCommitments[data.RequestID] == nil {
				aggregated.SigningCommitments[data.RequestID] = make(map[string][]byte)
			}
			aggregated.SigningCommitments[data.RequestID][validatorAddr] = data.Commitment
		}

		// Aggregate signature shares
		for _, data := range ext.SignatureShares {
			if aggregated.SignatureShares[data.RequestID] == nil {
				aggregated.SignatureShares[data.RequestID] = make(map[string][]byte)
			}
			aggregated.SignatureShares[data.RequestID][validatorAddr] = data.Share
		}
	}

	return aggregated
}
