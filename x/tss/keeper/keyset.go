package keeper

import (
	"context"
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// CreateKeySet creates a new KeySet and returns its ID
func (k Keeper) CreateKeySet(ctx context.Context, owner string, threshold, maxSigners uint32, description string) (string, error) {
	// Generate unique key_set_id (using block height + owner for uniqueness)
	// In production, consider using a counter or UUID
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	keySetID := fmt.Sprintf("keyset_%s_%d", owner, sdkCtx.BlockHeight())

	// Get active validators from x/threshold module
	// For now, we'll populate participants later when DKG starts
	participants := []string{}

	keySet := types.KeySet{
		Id:            keySetID,
		Owner:         owner,
		Description:   description,
		Threshold:     threshold,
		MaxSigners:    maxSigners,
		Participants:  participants,
		GroupPubkey:   nil, // Will be set after DKG completes
		Status:        types.KeySetStatus_KEY_SET_STATUS_PENDING_DKG,
		CreatedHeight: 0, // TODO: Get from context
	}

	if err := k.KeySetStore.Set(ctx, keySetID, keySet); err != nil {
		return "", err
	}

	sdkCtx = sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("CreateKeySet STORED",
		"id", keySet.Id,
		"owner", keySet.Owner,
		"threshold", keySet.Threshold,
		"max_signers", keySet.MaxSigners,
		"status", keySet.Status,
		"description", keySet.Description)

	// Verify it was stored by reading it back
	stored, err := k.KeySetStore.Get(ctx, keySetID)
	if err != nil {
		sdkCtx.Logger().Error("Failed to read back stored KeySet", "error", err)
	} else {
		sdkCtx.Logger().Info("CreateKeySet READ BACK",
			"id", stored.Id,
			"owner", stored.Owner,
			"threshold", stored.Threshold,
			"max_signers", stored.MaxSigners,
			"status", stored.Status,
			"description", stored.Description)
	}

	return keySetID, nil
}

// GetKeySet retrieves a KeySet by ID
func (k Keeper) GetKeySet(ctx context.Context, keySetID string) (types.KeySet, error) {
	keySet, err := k.KeySetStore.Get(ctx, keySetID)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.KeySet{}, fmt.Errorf("keyset not found: %s", keySetID)
		}
		return types.KeySet{}, err
	}
	return keySet, nil
}

// SetKeySet updates a KeySet
func (k Keeper) SetKeySet(ctx context.Context, keySet types.KeySet) error {
	return k.KeySetStore.Set(ctx, keySet.Id, keySet)
}

// GetAllKeySets retrieves all KeySets
func (k Keeper) GetAllKeySets(ctx context.Context) ([]types.KeySet, error) {
	var keySets []types.KeySet
	err := k.KeySetStore.Walk(ctx, nil, func(key string, value types.KeySet) (bool, error) {
		keySets = append(keySets, value)
		return false, nil
	})
	return keySets, err
}

// GetKeySetsByOwner retrieves all KeySets owned by a specific address
func (k Keeper) GetKeySetsByOwner(ctx context.Context, owner string) ([]types.KeySet, error) {
	var keySets []types.KeySet
	err := k.KeySetStore.Walk(ctx, nil, func(key string, value types.KeySet) (bool, error) {
		if value.Owner == owner {
			keySets = append(keySets, value)
		}
		return false, nil
	})
	return keySets, err
}

// ActivateKeySet marks a KeySet as active after DKG completes
func (k Keeper) ActivateKeySet(ctx context.Context, keySetID string, aggregatedPubkey []byte, participants []string) error {
	keySet, err := k.GetKeySet(ctx, keySetID)
	if err != nil {
		return err
	}

	keySet.Status = types.KeySetStatus_KEY_SET_STATUS_ACTIVE
	keySet.GroupPubkey = aggregatedPubkey
	keySet.Participants = participants
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	keySet.CreatedHeight = sdkCtx.BlockHeight()

	return k.SetKeySet(ctx, keySet)
}

// FailKeySet marks a KeySet as failed
func (k Keeper) FailKeySet(ctx context.Context, keySetID string) error {
	keySet, err := k.GetKeySet(ctx, keySetID)
	if err != nil {
		return err
	}

	keySet.Status = types.KeySetStatus_KEY_SET_STATUS_FAILED
	return k.SetKeySet(ctx, keySet)
}

// DeactivateKeySet marks a KeySet as inactive
func (k Keeper) DeactivateKeySet(ctx context.Context, keySetID string) error {
	keySet, err := k.GetKeySet(ctx, keySetID)
	if err != nil {
		return err
	}

	keySet.Status = types.KeySetStatus_KEY_SET_STATUS_FAILED
	return k.SetKeySet(ctx, keySet)
}
