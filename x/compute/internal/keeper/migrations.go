package keeper

import (
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{keeper: keeper}
}

func v1GetContractKey(ctx sdk.Context, k Keeper, contractAddress sdk.AccAddress) []byte {
	store := ctx.KVStore(k.storeKey)

	contractKey := store.Get(types.GetContractEnclaveKey(contractAddress))

	return contractKey
}

// Migrate1to2 migrates from version 1 to 2. The migration includes converting contractKey from []byte to:
//
//	type ContractKey struct {
//		OgContractKey           []byte
//		CurrentContractKey      []byte
//		CurrentContractKeyProof []byte
//	}
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	iter := prefix.NewStore(ctx.KVStore(m.keeper.storeKey), types.ContractKeyPrefix).Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		var contractAddress sdk.AccAddress = iter.Key()

		var contractInfo types.ContractInfo
		m.keeper.cdc.MustUnmarshal(iter.Value(), &contractInfo)

		if ctx.ChainID() == "secret-4" {
			if hardcodedContractAdmins[contractAddress.String()] != "" {
				contractInfo.Admin = hardcodedContractAdmins[contractAddress.String()]
				// When the contract has a hardcoded admin via gov, adminProof is ignored inside the enclave.
				// Otherwise and if valid, adminProof is a 32 bytes array (output of sha256).
				// For future proofing and avoiding passing null pointers to the enclave, we'll set it to a 32 bytes array of 0.
				contractInfo.AdminProof = make([]byte, 32)
			}
		}

		// get v1 contract key
		v1ContractKey := v1GetContractKey(ctx, m.keeper, contractAddress)

		// convert v1 contract key to v2 contract key
		v2ContractKey := types.ContractKey{
			OgContractKey:           v1ContractKey,
			CurrentContractKey:      v1ContractKey,
			CurrentContractKeyProof: nil,
		}

		// overide v1 contract key with v2 contract key in the store
		m.keeper.SetContractKey(ctx, contractAddress, &v2ContractKey)
	}

	return nil
}
