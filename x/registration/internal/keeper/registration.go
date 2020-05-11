package keeper

import (
	"encoding/hex"
	"fmt"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/EnigmaBlockchain/x/registration/internal/types"
)

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

func (k Keeper) setRegistrationInfo(ctx sdk.Context, certificate types.RegistrationNodeInfo, publicKey types.NodeID) {
	store := ctx.KVStore(k.storeKey)
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
