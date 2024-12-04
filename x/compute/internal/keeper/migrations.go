package keeper

import (
	"fmt"
	"time"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
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
	store := k.storeService.OpenKVStore(ctx)

	contractKey, _ := store.Get(types.GetContractEnclaveKey(contractAddress))

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
	iter := prefix.NewStore(runtime.KVStoreAdapter(m.keeper.storeService.OpenKVStore(ctx)), types.ContractKeyPrefix).Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		var contractAddress sdk.AccAddress = iter.Key()

		var contractInfo types.ContractInfo
		m.keeper.cdc.MustUnmarshal(iter.Value(), &contractInfo)

		if hardcodedContractAdmins[contractAddress.String()] != "" {
			contractInfo.Admin = hardcodedContractAdmins[contractAddress.String()]
			// When the contract has a hardcoded admin via gov, adminProof is ignored inside the enclave.
			// Otherwise and if valid, adminProof is a 32 bytes array (output of sha256).
			// For future proofing and avoiding passing null pointers to the enclave, we'll set it to a 32 bytes array of 0.
			contractInfo.AdminProof = make([]byte, 32)
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

func (m Migrator) Migrate2to3(_ sdk.Context) error {
	// Empty migration.
	// Because of a testnet bug, we had to do a bunch of migrations in the testnet
	// which let the ConsensusVersion there to be 4.
	// This migration is here to match the ConsensusVersion on mainnet to that of the testnet.
	return nil
}

func (m Migrator) Migrate3to4(_ sdk.Context) error {
	// Empty migration.
	// Because of a testnet bug, we had to do a bunch of migrations in the testnet
	// which let the ConsensusVersion there to be 4.
	// This migration is here to match the ConsensusVersion on mainnet to that of the testnet.
	return nil
}

func (m Migrator) Migrate4to5(ctx sdk.Context) error {
	store := prefix.NewStore(runtime.KVStoreAdapter(m.keeper.storeService.OpenKVStore(ctx)), types.ContractKeyPrefix)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	formatter := message.NewPrinter(language.English)
	migratedContracts := uint64(0)
	totalContracts := m.keeper.peekAutoIncrementID(ctx, types.KeyLastInstanceID) - 1
	previousTime := time.Now().UnixNano()

	for ; iter.Valid(); iter.Next() {
		var contractAddress sdk.AccAddress = iter.Key()

		var contractInfo types.ContractInfo
		m.keeper.cdc.MustUnmarshal(iter.Value(), &contractInfo)

		// Pre v1.11 contracts don't have a history, so we'll add an initial history entry for them.
		// This is required for the hardcoded admins feature to work.
		// This will also prevent an inconsistent state between pre v1.11 and post v1.11 contracts.
		contractHistory := m.keeper.GetContractHistory(ctx, contractAddress)
		if len(contractHistory) == 0 {
			historyEntry := contractInfo.InitialHistory(nil)

			// Persist the history entry changes.
			m.keeper.addToContractCodeSecondaryIndex(ctx, contractAddress, historyEntry)
			m.keeper.appendToContractHistory(ctx, contractAddress, historyEntry)
		}

		if hardcodedContractAdmins[contractAddress.String()] != "" {
			// This is the same code as in Migrate1to2() but with store.Set() to persist the changes.

			contractInfo.Admin = hardcodedContractAdmins[contractAddress.String()]
			// When the contract has a hardcoded admin via gov, adminProof is ignored inside the enclave.
			// Otherwise and if valid, adminProof is a 32 bytes array (output of sha256).
			// For future proofing and avoiding passing null pointers to the enclave, we'll set it to a 32 bytes array of 0.
			contractInfo.AdminProof = make([]byte, 32)

			// Persist the contractInfo changes.
			newContractInfoBz := m.keeper.cdc.MustMarshal(&contractInfo)
			store.Set(iter.Key(), newContractInfoBz)
		}

		migratedContracts++
		logMigrationProgress(ctx, formatter, migratedContracts, totalContracts, previousTime)
		previousTime = time.Now().UnixNano()
	}
	return nil
}

func (m Migrator) Migrate5to6(ctx sdk.Context) error {
	store := m.keeper.storeService.OpenKVStore(ctx)
	defaultParams := types.DefaultParams()
	bz, err := m.keeper.cdc.Marshal(&defaultParams)
	if err != nil {
		return err
	}
	store.Set(types.ParamsKey, bz)

	return nil
}

const progressPartSize = 1000

func logMigrationProgress(ctx sdk.Context, formatter *message.Printer, migratedContracts uint64, totalContracts uint64, previousTime int64) {
	if migratedContracts%progressPartSize == 0 || migratedContracts == totalContracts {
		if totalContracts > 0 {
			timePerPartNs := time.Now().UnixNano() - previousTime
			partsLeft := float64(totalContracts-migratedContracts) / float64(progressPartSize)
			timeLeftNs := uint64(partsLeft * float64(timePerPartNs))
			timeLeftSec := timeLeftNs / 1e9
			etaMinutes := uint(timeLeftSec / 60)
			etaSeconds := uint(timeLeftSec % 60)

			ctx.Logger().Info(
				formatter.Sprintf("Migrated contracts: %d/%d (%f%%), ETA: %s\n",
					migratedContracts,
					totalContracts,
					(float64(migratedContracts)/float64(totalContracts))*100,
					fmt.Sprintf(
						"%02dm:%02ds",
						etaMinutes,
						etaSeconds,
					),
				),
			)
		} else {
			ctx.Logger().Info(fmt.Sprintf("Migrated contracts: %d\n", migratedContracts))
		}
	}
}
