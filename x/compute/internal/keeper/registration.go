package keeper

import (
	"encoding/hex"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/EnigmaBlockchain/x/compute/internal/types"
)

func (k Keeper) getRegistrationInfo(ctx sdk.Context, publicKey types.NodeID) *types.RegistrationNodeInfo {
	store := ctx.KVStore(k.storeKey)
	var nodeInfo types.RegistrationNodeInfo
	fmt.Println("pubkey", hex.EncodeToString(publicKey))
	certBz := store.Get(publicKey)

	if certBz == nil {
		return nil
	}
	k.cdc.MustUnmarshalBinaryBare(certBz, &nodeInfo)

	return &nodeInfo
}

func (k Keeper) setRegistrationInfo(ctx sdk.Context, certificate types.RegistrationNodeInfo, publicKey types.NodeID) {
	store := ctx.KVStore(k.storeKey)
	fmt.Println("pubkey", hex.EncodeToString(publicKey))
	fmt.Println("EncryptedSeed", hex.EncodeToString(certificate.EncryptedSeed))
	store.Set(publicKey, k.cdc.MustMarshalBinaryBare(certificate))
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
