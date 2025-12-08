package keeper

import (
	"context"
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// InitiateDKGForKeySet creates a new DKG session for a specific KeySet
func (k Keeper) InitiateDKGForKeySet(ctx context.Context, keySetID string, threshold, maxSigners uint32, timeoutBlocks int64) (string, error) {
	// Get the KeySet to verify it exists and is in PENDING_DKG state
	keySet, err := k.GetKeySet(ctx, keySetID)
	if err != nil {
		return "", err
	}

	if keySet.Status != types.KeySetStatus_KEY_SET_STATUS_PENDING_DKG {
		return "", fmt.Errorf("keyset is not in PENDING_DKG state")
	}

	// Generate unique session ID
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sessionID := fmt.Sprintf("dkg-%s-%d", keySetID, sdkCtx.BlockHeight())

	// Get current block height for timeout
	currentHeight := sdkCtx.BlockHeight()
	// Default to 100 blocks if not specified
	if timeoutBlocks == 0 {
		timeoutBlocks = 100
	}
	timeoutHeight := currentHeight + timeoutBlocks

	// Get all active validators by their consensus addresses
	// For now, use all validators from the current validator set
	participants, err := k.GetActiveValidatorAddresses(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get active validators: %w", err)
	}

	sdkCtx.Logger().Info("InitiateDKGForKeySet", "participants_count", len(participants), "participants", participants)

	// DEBUG: Fail explicitly if no participants found so we can see it in tx result
	if len(participants) == 0 {
		return "", fmt.Errorf("no active validators found for DKG - check that validators are bonded and not jailed")
	}

	// Create DKG session
	session := types.DKGSession{
		Id:            sessionID,
		KeySetId:      keySetID,
		State:         types.DKGState_DKG_STATE_ROUND1,
		Threshold:     threshold,
		MaxSigners:    maxSigners,
		Participants:  participants, // Use active validator consensus addresses
		StartHeight:   currentHeight,
		TimeoutHeight: timeoutHeight,
	}

	// Store the session
	if err := k.DKGSessionStore.Set(ctx, sessionID, session); err != nil {
		return "", err
	}

	sdkCtx.Logger().Info("DKG Session Created",
		"session_id", sessionID,
		"key_set_id", keySetID,
		"participants_count", len(session.Participants),
		"participants", session.Participants,
		"threshold", threshold,
		"timeout_height", timeoutHeight)

	return sessionID, nil
}

// GetDKGSession retrieves a DKG session by ID
func (k Keeper) GetDKGSession(ctx context.Context, sessionID string) (types.DKGSession, error) {
	session, err := k.DKGSessionStore.Get(ctx, sessionID)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.DKGSession{}, fmt.Errorf("DKG session not found: %s", sessionID)
		}
		return types.DKGSession{}, err
	}
	return session, nil
}

// SetDKGSession updates a DKG session
func (k Keeper) SetDKGSession(ctx context.Context, session types.DKGSession) error {
	return k.DKGSessionStore.Set(ctx, session.Id, session)
}

// GetAllDKGSessions retrieves all DKG sessions
func (k Keeper) GetAllDKGSessions(ctx context.Context) ([]types.DKGSession, error) {
	var sessions []types.DKGSession
	err := k.DKGSessionStore.Walk(ctx, nil, func(key string, value types.DKGSession) (bool, error) {
		sessions = append(sessions, value)
		return false, nil
	})
	return sessions, err
}

// ProcessDKGRound1 stores a validator's Round 1 commitment
func (k Keeper) ProcessDKGRound1(ctx context.Context, sessionID, validatorAddr string, commitment []byte) error {
	// Get the session
	session, err := k.GetDKGSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Verify session is in ROUND1 state
	if session.State != types.DKGState_DKG_STATE_ROUND1 {
		return fmt.Errorf("DKG session is not in ROUND1 state")
	}

	// Verify validator is a participant
	if !contains(session.Participants, validatorAddr) {
		return fmt.Errorf("validator %s is not a participant in this DKG session", validatorAddr)
	}

	// Check if validator already submitted
	existingKey := fmt.Sprintf("%s:%s", sessionID, validatorAddr)
	has, err := k.DKGRound1DataStore.Has(ctx, existingKey)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("validator %s already submitted round 1 data", validatorAddr)
	}

	// Store Round 1 data
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	round1Data := types.DKGRound1Data{
		ValidatorAddress: validatorAddr,
		Commitment:       commitment,
		SubmittedHeight:  sdkCtx.BlockHeight(),
	}

	return k.DKGRound1DataStore.Set(ctx, existingKey, round1Data)
}

// ProcessDKGRound2 stores a validator's Round 2 share
func (k Keeper) ProcessDKGRound2(ctx context.Context, sessionID, validatorAddr string, share []byte) error {
	// Get the session
	session, err := k.GetDKGSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Verify session is in ROUND2 state
	if session.State != types.DKGState_DKG_STATE_ROUND2 {
		return fmt.Errorf("DKG session is not in ROUND2 state")
	}

	// Verify validator is a participant
	if !contains(session.Participants, validatorAddr) {
		return fmt.Errorf("validator %s is not a participant in this DKG session", validatorAddr)
	}

	// Check if validator already submitted
	existingKey := fmt.Sprintf("%s:%s", sessionID, validatorAddr)
	has, err := k.DKGRound2DataStore.Has(ctx, existingKey)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("validator %s already submitted round 2 data", validatorAddr)
	}

	// Store Round 2 data
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	round2Data := types.DKGRound2Data{
		ValidatorAddress: validatorAddr,
		Share:            share,
		SubmittedHeight:  sdkCtx.BlockHeight(),
	}

	return k.DKGRound2DataStore.Set(ctx, existingKey, round2Data)
}

// GetDKGRound1Count returns the number of Round 1 submissions for a session
func (k Keeper) GetDKGRound1Count(ctx context.Context, sessionID string) (int, error) {
	count := 0
	prefix := sessionID + ":"

	err := k.DKGRound1DataStore.Walk(ctx, nil, func(key string, value types.DKGRound1Data) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			count++
		}
		return false, nil
	})

	return count, err
}

// GetDKGRound2Count returns the number of Round 2 submissions for a session
func (k Keeper) GetDKGRound2Count(ctx context.Context, sessionID string) (int, error) {
	count := 0
	prefix := sessionID + ":"

	err := k.DKGRound2DataStore.Walk(ctx, nil, func(key string, value types.DKGRound2Data) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			count++
		}
		return false, nil
	})

	return count, err
}

// ProcessDKGKeySubmission stores a validator's encrypted key share submission
func (k Keeper) ProcessDKGKeySubmission(ctx context.Context, sessionID, validatorAddr string,
	encryptedSecretShare, encryptedPublicShares, ephemeralPubKey []byte) error {
	// Get the session
	session, err := k.GetDKGSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Verify session is in KEY_SUBMISSION state
	if session.State != types.DKGState_DKG_STATE_KEY_SUBMISSION {
		return fmt.Errorf("DKG session is not in KEY_SUBMISSION state")
	}

	// Verify validator is a participant
	if !contains(session.Participants, validatorAddr) {
		return fmt.Errorf("validator %s is not a participant in this DKG session", validatorAddr)
	}

	// Check if validator already submitted
	existingKey := fmt.Sprintf("%s:%s", sessionID, validatorAddr)
	has, err := k.DKGKeySubmissionStore.Has(ctx, existingKey)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("validator %s already submitted encrypted key share", validatorAddr)
	}

	// Store key submission
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	submission := types.DKGKeySubmission{
		ValidatorAddress:      validatorAddr,
		EncryptedSecretShare:  encryptedSecretShare,
		EncryptedPublicShares: encryptedPublicShares,
		EphemeralPubkey:       ephemeralPubKey,
		SubmittedHeight:       sdkCtx.BlockHeight(),
	}

	return k.DKGKeySubmissionStore.Set(ctx, existingKey, submission)
}

// GetDKGKeySubmissionCount returns the number of encrypted key submissions for a session
func (k Keeper) GetDKGKeySubmissionCount(ctx context.Context, sessionID string) (int, error) {
	count := 0
	prefix := sessionID + ":"

	err := k.DKGKeySubmissionStore.Walk(ctx, nil, func(key string, value types.DKGKeySubmission) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			count++
		}
		return false, nil
	})

	return count, err
}

// GetDKGKeySubmissions returns all encrypted key submissions for a session
func (k Keeper) GetDKGKeySubmissions(ctx context.Context, sessionID string) (map[string]types.DKGKeySubmission, error) {
	submissions := make(map[string]types.DKGKeySubmission)
	prefix := sessionID + ":"

	err := k.DKGKeySubmissionStore.Walk(ctx, nil, func(key string, value types.DKGKeySubmission) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			submissions[value.ValidatorAddress] = value
		}
		return false, nil
	})

	return submissions, err
}

// CompleteDKG finalizes a DKG ceremony and activates the KeySet
// This is called after KEY_SUBMISSION phase when all encrypted shares have been collected
func (k Keeper) CompleteDKG(ctx context.Context, sessionID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get the session
	session, err := k.GetDKGSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Get the group public key from local FROST state (computed during Round 2)
	groupPubkey, err := k.GetDKGGroupPubKey(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get group public key: %w", err)
	}

	// Get all encrypted key submissions
	submissions, err := k.GetDKGKeySubmissions(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get key submissions: %w", err)
	}

	// Update KeySet status to ACTIVE with the group public key and participants
	if err := k.ActivateKeySet(ctx, session.KeySetId, groupPubkey, session.Participants); err != nil {
		return err
	}

	// Store encrypted key shares for each validator who submitted
	for validatorAddr, submission := range submissions {
		if err := k.SetEncryptedKeyShare(ctx, session.KeySetId, validatorAddr, groupPubkey,
			submission.EncryptedSecretShare, submission.EncryptedPublicShares, submission.EphemeralPubkey); err != nil {
			return fmt.Errorf("failed to store encrypted key share for %s: %w", validatorAddr, err)
		}
		sdkCtx.Logger().Info("Stored encrypted key share on-chain",
			"keyset_id", session.KeySetId, "validator", validatorAddr)
	}

	// Delete the completed DKG session - no longer needed
	if err := k.DKGSessionStore.Remove(ctx, sessionID); err != nil {
		return err
	}

	// Clean up all round data including key submissions
	k.cleanupDKGRoundData(ctx, sessionID)
	k.cleanupDKGKeySubmissions(ctx, sessionID)

	// Clean up local FROST state (we no longer keep keys in memory)
	k.CleanupDKGState(sessionID)

	sdkCtx.Logger().Info("DKG completed - encrypted key shares stored on-chain", "session_id", sessionID)

	return nil
}

// FailDKG marks a DKG session and its KeySet as failed
func (k Keeper) FailDKG(ctx context.Context, sessionID string) error {
	// Get the session
	session, err := k.GetDKGSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Update KeySet status to FAILED
	if err := k.FailKeySet(ctx, session.KeySetId); err != nil {
		return err
	}

	// Delete the failed DKG session
	if err := k.DKGSessionStore.Remove(ctx, sessionID); err != nil {
		return err
	}

	// Clean up round data
	k.cleanupDKGRoundData(ctx, sessionID)

	return nil
}

// cleanupDKGRoundData removes round 1 and round 2 data for a session
func (k Keeper) cleanupDKGRoundData(ctx context.Context, sessionID string) {
	prefix := sessionID + ":"

	// Clean up Round 1 data
	var round1Keys []string
	k.DKGRound1DataStore.Walk(ctx, nil, func(key string, _ types.DKGRound1Data) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			round1Keys = append(round1Keys, key)
		}
		return false, nil
	})
	for _, key := range round1Keys {
		k.DKGRound1DataStore.Remove(ctx, key)
	}

	// Clean up Round 2 data
	var round2Keys []string
	k.DKGRound2DataStore.Walk(ctx, nil, func(key string, _ types.DKGRound2Data) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			round2Keys = append(round2Keys, key)
		}
		return false, nil
	})
	for _, key := range round2Keys {
		k.DKGRound2DataStore.Remove(ctx, key)
	}
}

// cleanupDKGKeySubmissions removes key submission data for a session
func (k Keeper) cleanupDKGKeySubmissions(ctx context.Context, sessionID string) {
	prefix := sessionID + ":"

	var keys []string
	k.DKGKeySubmissionStore.Walk(ctx, nil, func(key string, _ types.DKGKeySubmission) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
		return false, nil
	})
	for _, key := range keys {
		k.DKGKeySubmissionStore.Remove(ctx, key)
	}
}

// ProcessDKGEndBlock handles DKG state transitions at the end of each block
func (k Keeper) ProcessDKGEndBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	// Iterate through all DKG sessions
	err := k.DKGSessionStore.Walk(ctx, nil, func(sessionID string, session types.DKGSession) (bool, error) {
		// Skip completed or failed sessions
		if session.State == types.DKGState_DKG_STATE_COMPLETE || session.State == types.DKGState_DKG_STATE_FAILED {
			return false, nil
		}

		// Check for timeout
		if currentHeight >= session.TimeoutHeight {
			if err := k.FailDKG(ctx, sessionID); err != nil {
				return true, err
			}
			return false, nil
		}

		// Handle state transitions based on current state
		switch session.State {
		case types.DKGState_DKG_STATE_ROUND1:
			// Check if enough Round 1 submissions
			count, err := k.GetDKGRound1Count(ctx, sessionID)
			if err != nil {
				return true, err
			}

			// If threshold met, advance to Round 2
			if uint32(count) >= session.Threshold {
				session.State = types.DKGState_DKG_STATE_ROUND2
				if err := k.SetDKGSession(ctx, session); err != nil {
					return true, err
				}
				sdkCtx.Logger().Info("DKG transitioning to ROUND2", "session_id", sessionID)
			}

		case types.DKGState_DKG_STATE_ROUND2:
			// Check if enough Round 2 submissions
			count, err := k.GetDKGRound2Count(ctx, sessionID)
			if err != nil {
				return true, err
			}

			// If threshold met, advance to KEY_SUBMISSION state
			// (validators need to submit their encrypted key shares)
			if uint32(count) >= session.Threshold {
				session.State = types.DKGState_DKG_STATE_KEY_SUBMISSION
				if err := k.SetDKGSession(ctx, session); err != nil {
					return true, err
				}
				sdkCtx.Logger().Info("DKG transitioning to KEY_SUBMISSION", "session_id", sessionID)
			}

		case types.DKGState_DKG_STATE_KEY_SUBMISSION:
			// Check if enough encrypted key submissions
			count, err := k.GetDKGKeySubmissionCount(ctx, sessionID)
			if err != nil {
				return true, err
			}

			// If threshold met, complete DKG and store encrypted shares on-chain
			if uint32(count) >= session.Threshold {
				if err := k.CompleteDKG(ctx, sessionID); err != nil {
					return true, err
				}
				sdkCtx.Logger().Info("DKG completed with encrypted key shares stored on-chain", "session_id", sessionID)
			}
		}

		return false, nil
	})

	return err
}

// Helper function to check if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
