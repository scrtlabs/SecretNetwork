package v1_11_testnet_shade_hardcoded_admins_fix

import (
	"fmt"

	store "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
)

const upgradeName = "v1.11-testnet-shade-hardcoded-admins-fix"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

func setHardCodedAdmins(ctx sdk.Context) error {
	iter := prefix.NewStore(ctx.KVStore(m.keeper.storeKey), types.ContractKeyPrefix).Iterator(nil, nil)
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

func createUpgradeHandler(mm *module.Manager, _ *keepers.SecretAppKeepers, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info(` _    _ _____   _____ _____            _____  ______ `)
		ctx.Logger().Info(`| |  | |  __ \ / ____|  __ \     /\   |  __ \|  ____|`)
		ctx.Logger().Info(`| |  | | |__) | |  __| |__) |   /  \  | |  | | |__   `)
		ctx.Logger().Info(`| |  | |  ___/| | |_ |  _  /   / /\ \ | |  | |  __|  `)
		ctx.Logger().Info(`| |__| | |    | |__| | | \ \  / ____ \| |__| | |____ `)
		ctx.Logger().Info(` \____/|_|     \_____|_|  \_\/_/    \_\_____/|______|`)

		setHardCodedAdmins(ctx)

		ctx.Logger().Info(fmt.Sprintf("Running module migrations for %s...", upgradeName))
		return mm.RunMigrations(ctx, configurator, vm)
	}
}
