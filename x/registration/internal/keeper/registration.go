package keeper

import (
	"encoding/hex"
	"fmt"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/EnigmaBlockchain/x/registration/internal/types"
	ra "github.com/enigmampc/EnigmaBlockchain/x/registration/remote_attestation"
)

func (k Keeper) GetMasterPublicKey(ctx sdk.Context) *types.PublicKey {
	store := ctx.KVStore(k.storeKey)

	var pkIO types.PublicKey
	certBz := store.Get([]byte(types.MasterPublicKeyId))
	if certBz == nil {
		return nil
	}
	k.cdc.MustUnmarshalBinaryBare(certBz, &pkIO)

	return &pkIO
}

func (k Keeper) setMasterPublicKey(ctx sdk.Context, publicKey types.PublicKey) {
	store := ctx.KVStore(k.storeKey)

	store.Set(types.GetMasterPublicKey([]byte(types.MasterPublicKeyId)), k.cdc.MustMarshalBinaryBare(publicKey))
}

func (k Keeper) isMasterKeyDefined(ctx sdk.Context) bool {
	regInfo := k.GetMasterPublicKey(ctx)
	if regInfo == nil {
		return false
	}
	return true
}

func (k Keeper) getRegistrationInfo(ctx sdk.Context, publicKey types.NodeID) *types.RegistrationNodeInfo {
	store := ctx.KVStore(k.storeKey)
	var nodeInfo types.RegistrationNodeInfo
	fmt.Println("pubkey", hex.EncodeToString(publicKey))
	certBz := store.Get(types.GetRegistrationKey(publicKey))

	if certBz == nil {
		return nil
	}
	k.cdc.MustUnmarshalBinaryBare(certBz, &nodeInfo)

	return &nodeInfo
}

func (k Keeper) ListRegistrationInfo(ctx sdk.Context, cb func([]byte, types.RegistrationNodeInfo) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.RegistrationStorePrefix)
	iter := prefixStore.Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		var regInfo types.RegistrationNodeInfo
		k.cdc.MustUnmarshalBinaryBare(iter.Value(), &regInfo)
		// cb returns true to stop early
		if cb(iter.Key(), regInfo) {
			break
		}
	}
}

func (k Keeper) setRegistrationInfo(ctx sdk.Context, certificate types.RegistrationNodeInfo) {
	store := ctx.KVStore(k.storeKey)

	publicKey, err := ra.VerifyRaCert(certificate.Certificate)
	if err != nil {
		return
	}

	fmt.Println("pubkey", hex.EncodeToString(publicKey))
	fmt.Println("EncryptedSeed", hex.EncodeToString(certificate.EncryptedSeed))
	store.Set(types.GetRegistrationKey(publicKey), k.cdc.MustMarshalBinaryBare(certificate))
}

func (k Keeper) isNodeAuthenticated(ctx sdk.Context, publicKey types.NodeID) (bool, error) {
	regInfo := k.getRegistrationInfo(ctx, publicKey)
	if regInfo == nil {
		return false, nil
	}

	if regInfo.EncryptedSeed == nil {
		return false, nil
	} else {
		return true, nil
	}
}
