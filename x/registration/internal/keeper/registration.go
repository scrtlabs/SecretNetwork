package keeper

import (
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
	ra "github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
)

func (k Keeper) GetMasterKey(ctx sdk.Context, keyType string) *types.MasterKey {
	store := ctx.KVStore(k.storeKey)
	var key types.MasterKey
	certBz := store.Get(types.MasterKeyPrefix(keyType))
	if certBz == nil {
		return nil
	}
	k.cdc.MustUnmarshal(certBz, &key)
	return &key
}

func (k Keeper) setMasterKey(ctx sdk.Context, key types.MasterKey, keyType string) {
	store := ctx.KVStore(k.storeKey)

	store.Set(types.MasterKeyPrefix(keyType), k.cdc.MustMarshal(&key))
}

func (k Keeper) isMasterCertificateDefined(ctx sdk.Context, keyType string) bool {
	regInfo := k.GetMasterKey(ctx, keyType)
	return regInfo != nil
}

func (k Keeper) getRegistrationInfo(ctx sdk.Context, publicKey types.NodeID) *types.RegistrationNodeInfo {
	store := ctx.KVStore(k.storeKey)
	var nodeInfo types.RegistrationNodeInfo
	// fmt.Println("pubkey", hex.EncodeToString(publicKey))
	certBz := store.Get(types.RegistrationKeyPrefix(publicKey))

	if certBz == nil {
		return nil
	}
	k.cdc.MustUnmarshal(certBz, &nodeInfo)

	return &nodeInfo
}

func (k Keeper) ListRegistrationInfo(ctx sdk.Context, cb func([]byte, types.RegistrationNodeInfo) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.RegistrationStorePrefix)
	iter := prefixStore.Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		var regInfo types.RegistrationNodeInfo
		k.cdc.MustUnmarshal(iter.Value(), &regInfo)
		// cb returns true to stop early
		if cb(iter.Key(), regInfo) {
			break
		}
	}
}

func (k Keeper) SetRegistrationInfo(ctx sdk.Context, certificate types.RegistrationNodeInfo) {
	store := ctx.KVStore(k.storeKey)

	publicKey, err := ra.VerifyRaCert(certificate.Certificate)
	if err != nil {
		return
	}

	// fmt.Println("pubkey", hex.EncodeToString(publicKey))
	// fmt.Println("EncryptedSeed", hex.EncodeToString(certificate.EncryptedSeed))
	store.Set(types.RegistrationKeyPrefix(publicKey), k.cdc.MustMarshal(&certificate))
}

func (k Keeper) isNodeAuthenticated(ctx sdk.Context, publicKey types.NodeID) (bool, error) {
	regInfo := k.getRegistrationInfo(ctx, publicKey)
	if regInfo == nil {
		return false, nil
	}

	if regInfo.EncryptedSeed == nil {
		return false, nil
	}
	return true, nil
}
