package keeper

import (
	"context"
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// SetKeyShare stores a validator's key share for a specific KeySet (legacy, metadata only)
func (k Keeper) SetKeyShare(ctx context.Context, keySetID, validatorAddr string, shareData, pubkey []byte) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	keyShare := types.KeyShare{
		KeySetId:         keySetID,
		ValidatorAddress: validatorAddr,
		ShareData:        shareData,
		GroupPubkey:      pubkey,
		CreatedHeight:    sdkCtx.BlockHeight(),
	}

	key := collections.Join(keySetID, validatorAddr)
	return k.KeyShareStore.Set(ctx, key, keyShare)
}

// SetEncryptedKeyShare stores a validator's encrypted key share on-chain
// The secret share and public shares are encrypted with the validator's Ed25519 public key
func (k Keeper) SetEncryptedKeyShare(ctx context.Context, keySetID, validatorAddr string, groupPubkey []byte,
	encryptedSecretShare, encryptedPublicShares, ephemeralPubKey []byte) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	keyShare := types.KeyShare{
		KeySetId:              keySetID,
		ValidatorAddress:      validatorAddr,
		GroupPubkey:           groupPubkey,
		CreatedHeight:         sdkCtx.BlockHeight(),
		EncryptedSecretShare:  encryptedSecretShare,
		EncryptedPublicShares: encryptedPublicShares,
		EphemeralPubkey:       ephemeralPubKey,
	}

	key := collections.Join(keySetID, validatorAddr)
	return k.KeyShareStore.Set(ctx, key, keyShare)
}

// GetKeyShare retrieves a validator's key share for a specific KeySet
func (k Keeper) GetKeyShare(ctx context.Context, keySetID, validatorAddr string) (types.KeyShare, error) {
	key := collections.Join(keySetID, validatorAddr)
	keyShare, err := k.KeyShareStore.Get(ctx, key)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.KeyShare{}, fmt.Errorf("key share not found for keyset %s, validator %s", keySetID, validatorAddr)
		}
		return types.KeyShare{}, err
	}
	return keyShare, nil
}

// GetKeySharesForKeySet retrieves all key shares for a specific KeySet
func (k Keeper) GetKeySharesForKeySet(ctx context.Context, keySetID string) ([]types.KeyShare, error) {
	var keyShares []types.KeyShare

	// Walk through all key shares and filter by keySetID
	err := k.KeyShareStore.Walk(ctx, nil, func(key collections.Pair[string, string], value types.KeyShare) (bool, error) {
		if value.KeySetId == keySetID {
			keyShares = append(keyShares, value)
		}
		return false, nil
	})

	return keyShares, err
}

// HasKeyShare checks if a validator has a key share for a specific KeySet
func (k Keeper) HasKeyShare(ctx context.Context, keySetID, validatorAddr string) (bool, error) {
	key := collections.Join(keySetID, validatorAddr)
	return k.KeyShareStore.Has(ctx, key)
}

// DeleteKeyShare removes a validator's key share for a specific KeySet
func (k Keeper) DeleteKeyShare(ctx context.Context, keySetID, validatorAddr string) error {
	key := collections.Join(keySetID, validatorAddr)
	return k.KeyShareStore.Remove(ctx, key)
}
