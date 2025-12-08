package keeper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// CreateSigningRequest creates a new signing request and initializes a signing session
func (k Keeper) CreateSigningRequest(ctx context.Context, keySetID, requester string, messageHash []byte, callback string) (string, error) {
	// Get the KeySet to verify it exists and is active
	keySet, err := k.GetKeySet(ctx, keySetID)
	if err != nil {
		return "", err
	}

	if keySet.Status != 2 { // ACTIVE status (from KeySetStatus enum)
		return "", fmt.Errorf("keyset is not active")
	}

	// Generate unique request ID
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	requestID := fmt.Sprintf("sig-%s-%d", keySetID, sdkCtx.BlockHeight())

	// Get current block height
	currentHeight := sdkCtx.BlockHeight()

	// Create signing request
	request := types.SigningRequest{
		Id:              requestID,
		KeySetId:        keySetID,
		Requester:       requester,
		MessageHash:     messageHash,
		Status:          types.SigningRequestStatus_SIGNING_REQUEST_STATUS_PENDING,
		Signature:       nil,
		Callback:        callback,
		CreatedHeight:   currentHeight,
	}

	// Store the request
	if err := k.SigningRequestStore.Set(ctx, requestID, request); err != nil {
		return "", err
	}

	// Create signing session
	session := types.SigningSession{
		Participants: keySet.Participants,
		Threshold:    keySet.Threshold,
	}

	// Store the session
	if err := k.SigningSessionStore.Set(ctx, requestID, session); err != nil {
		return "", err
	}

	return requestID, nil
}

// GetSigningRequest retrieves a signing request by ID
func (k Keeper) GetSigningRequest(ctx context.Context, requestID string) (types.SigningRequest, error) {
	request, err := k.SigningRequestStore.Get(ctx, requestID)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.SigningRequest{}, fmt.Errorf("signing request not found: %s", requestID)
		}
		return types.SigningRequest{}, err
	}
	return request, nil
}

// SetSigningRequest updates a signing request
func (k Keeper) SetSigningRequest(ctx context.Context, request types.SigningRequest) error {
	return k.SigningRequestStore.Set(ctx, request.Id, request)
}

// ProcessSigningCommitment stores a validator's Round 1 commitment
func (k Keeper) ProcessSigningCommitment(ctx context.Context, requestID, validatorAddr string, commitment []byte) error {
	// Get the request
	request, err := k.GetSigningRequest(ctx, requestID)
	if err != nil {
		return err
	}

	// Verify request is in ROUND1 state
	if request.Status != types.SigningRequestStatus_SIGNING_REQUEST_STATUS_ROUND1 {
		return fmt.Errorf("signing request is not in ROUND1 state")
	}

	// Get the session
	session, err := k.SigningSessionStore.Get(ctx, requestID)
	if err != nil {
		return err
	}

	// Verify validator is a participant
	if !contains(session.Participants, validatorAddr) {
		return fmt.Errorf("validator %s is not a participant in this signing session", validatorAddr)
	}

	// Check if validator already submitted
	existingKey := fmt.Sprintf("%s:%s", requestID, validatorAddr)
	has, err := k.SigningCommitmentStore.Has(ctx, existingKey)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("validator %s already submitted commitment", validatorAddr)
	}

	// Store commitment
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	commitmentData := types.SigningCommitment{
		ValidatorAddress: validatorAddr,
		Commitment:       commitment,
		SubmittedHeight:  sdkCtx.BlockHeight(),
	}

	return k.SigningCommitmentStore.Set(ctx, existingKey, commitmentData)
}

// ProcessSignatureShare stores a validator's Round 2 share
func (k Keeper) ProcessSignatureShare(ctx context.Context, requestID, validatorAddr string, share []byte) error {
	// Get the request
	request, err := k.GetSigningRequest(ctx, requestID)
	if err != nil {
		return err
	}

	// Verify request is in ROUND2 state
	if request.Status != types.SigningRequestStatus_SIGNING_REQUEST_STATUS_ROUND2 {
		return fmt.Errorf("signing request is not in ROUND2 state")
	}

	// Get the session
	session, err := k.SigningSessionStore.Get(ctx, requestID)
	if err != nil {
		return err
	}

	// Verify validator is a participant
	if !contains(session.Participants, validatorAddr) {
		return fmt.Errorf("validator %s is not a participant in this signing session", validatorAddr)
	}

	// Check if validator already submitted
	existingKey := fmt.Sprintf("%s:%s", requestID, validatorAddr)
	has, err := k.SignatureShareStore.Has(ctx, existingKey)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("validator %s already submitted signature share", validatorAddr)
	}

	// Store share
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	shareData := types.SignatureShare{
		ValidatorAddress: validatorAddr,
		Share:            share,
		SubmittedHeight:  sdkCtx.BlockHeight(),
	}

	return k.SignatureShareStore.Set(ctx, existingKey, shareData)
}

// GetSigningCommitmentCount returns the number of Round 1 commitments for a request
func (k Keeper) GetSigningCommitmentCount(ctx context.Context, requestID string) (int, error) {
	count := 0
	prefix := requestID + ":"

	err := k.SigningCommitmentStore.Walk(ctx, nil, func(key string, value types.SigningCommitment) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			count++
		}
		return false, nil
	})

	return count, err
}

// GetSignatureShareCount returns the number of Round 2 shares for a request
func (k Keeper) GetSignatureShareCount(ctx context.Context, requestID string) (int, error) {
	count := 0
	prefix := requestID + ":"

	err := k.SignatureShareStore.Walk(ctx, nil, func(key string, value types.SignatureShare) (bool, error) {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			count++
		}
		return false, nil
	})

	return count, err
}

// CompleteSignature finalizes a signing request and stores the aggregated signature
func (k *Keeper) CompleteSignature(ctx context.Context, requestID string) error {
	// Get the request
	request, err := k.GetSigningRequest(ctx, requestID)
	if err != nil {
		return err
	}

	// Get the session
	session, err := k.SigningSessionStore.Get(ctx, requestID)
	if err != nil {
		return fmt.Errorf("failed to get signing session: %w", err)
	}

	// Aggregate signature using FROST
	aggregatedSignature, err := k.AggregateSignature(ctx, request, session)
	if err != nil {
		return fmt.Errorf("failed to aggregate signature: %w", err)
	}

	// Update request status
	request.Status = types.SigningRequestStatus_SIGNING_REQUEST_STATUS_COMPLETE
	request.Signature = aggregatedSignature

	if err := k.SetSigningRequest(ctx, request); err != nil {
		return err
	}

	// Log the completed signature
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("TSS Signature completed",
		"request_id", requestID,
		"keyset_id", request.KeySetId,
		"signature_length", len(aggregatedSignature))
	fmt.Printf("TSS Signature completed: request_id=%s signature_hex=%x\n", requestID, aggregatedSignature)

	// If callback is set, invoke callback contract via sudo
	if request.Callback != "" && k.wasmKeeper != nil {
		if err := k.invokeSignatureCallback(ctx, request.Callback, requestID, aggregatedSignature); err != nil {
			sdkCtx.Logger().Error("Failed to invoke signature callback",
				"callback", request.Callback,
				"request_id", requestID,
				"error", err)
			// Don't fail the whole operation if callback fails
			// The signature is still valid and stored
		}
	}

	return nil
}

// SignatureCompleteMsg is the sudo message sent to contracts when signature is ready
type SignatureCompleteMsg struct {
	SignatureComplete SignatureCompleteData `json:"signature_complete"`
}

// SignatureCompleteData contains the signature completion data
type SignatureCompleteData struct {
	RequestID string `json:"request_id"`
	Signature []byte `json:"signature"`
}

// invokeSignatureCallback calls a contract's sudo entry point with the signature
func (k Keeper) invokeSignatureCallback(ctx context.Context, callbackAddr string, requestID string, signature []byte) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Parse the callback contract address
	contractAddr, err := sdk.AccAddressFromBech32(callbackAddr)
	if err != nil {
		return fmt.Errorf("invalid callback address %s: %w", callbackAddr, err)
	}

	// Create the sudo message
	sudoMsg := SignatureCompleteMsg{
		SignatureComplete: SignatureCompleteData{
			RequestID: requestID,
			Signature: signature,
		},
	}

	msgBytes, err := json.Marshal(sudoMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal sudo message: %w", err)
	}

	sdkCtx.Logger().Info("Invoking signature callback",
		"contract", callbackAddr,
		"request_id", requestID)

	// Call the contract via sudo
	_, err = k.wasmKeeper.Sudo(ctx, contractAddr, msgBytes)
	if err != nil {
		return fmt.Errorf("sudo call failed: %w", err)
	}

	sdkCtx.Logger().Info("Signature callback successful",
		"contract", callbackAddr,
		"request_id", requestID)

	return nil
}

// FailSigningRequest marks a signing request as failed
func (k Keeper) FailSigningRequest(ctx context.Context, requestID string) error {
	// Get the request
	request, err := k.GetSigningRequest(ctx, requestID)
	if err != nil {
		return err
	}

	// Update request status to FAILED
	request.Status = types.SigningRequestStatus_SIGNING_REQUEST_STATUS_FAILED
	return k.SetSigningRequest(ctx, request)
}

// ProcessSigningEndBlock handles signing state transitions at the end of each block
func (k *Keeper) ProcessSigningEndBlock(ctx context.Context) error {
	// Iterate through all signing requests
	err := k.SigningRequestStore.Walk(ctx, nil, func(requestID string, request types.SigningRequest) (bool, error) {
		// Skip completed or failed requests
		if request.Status == types.SigningRequestStatus_SIGNING_REQUEST_STATUS_COMPLETE ||
			request.Status == types.SigningRequestStatus_SIGNING_REQUEST_STATUS_FAILED {
			return false, nil
		}

		// Get the session
		session, err := k.SigningSessionStore.Get(ctx, requestID)
		if err != nil {
			return true, err
		}

		// Handle state transitions based on current status
		switch request.Status {
		case types.SigningRequestStatus_SIGNING_REQUEST_STATUS_PENDING:
			// Transition to ROUND1
			request.Status = types.SigningRequestStatus_SIGNING_REQUEST_STATUS_ROUND1
			if err := k.SetSigningRequest(ctx, request); err != nil {
				return true, err
			}

		case types.SigningRequestStatus_SIGNING_REQUEST_STATUS_ROUND1:
			// Check if enough Round 1 commitments
			count, err := k.GetSigningCommitmentCount(ctx, requestID)
			if err != nil {
				return true, err
			}

			// If threshold met, advance to Round 2
			if uint32(count) >= session.Threshold {
				request.Status = types.SigningRequestStatus_SIGNING_REQUEST_STATUS_ROUND2
				if err := k.SetSigningRequest(ctx, request); err != nil {
					return true, err
				}
			}

		case types.SigningRequestStatus_SIGNING_REQUEST_STATUS_ROUND2:
			// Check if enough Round 2 shares
			count, err := k.GetSignatureShareCount(ctx, requestID)
			if err != nil {
				return true, err
			}

			// If threshold met, complete signing
			if uint32(count) >= session.Threshold {
				if err := k.CompleteSignature(ctx, requestID); err != nil {
					return true, err
				}
			}
		}

		return false, nil
	})

	return err
}
