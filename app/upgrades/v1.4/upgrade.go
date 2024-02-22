package v1_4

import (
	"fmt"

	store "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
)

const UpgradeName = "v1.4"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

func CreateUpgradeHandler(mm *module.Manager, keepers *keepers.SecretAppKeepers, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		// We're not upgrading cosmos-sdk, Tendermint or ibc-go, so no ConsensusVersion changes
		// Therefore mm.RunMigrations() should not find any module to upgrade

		ctx.Logger().Info("Running revert of tombstoning")
		err := RevertCosTombstoning(
			ctx,
			keepers.SlashingKeeper,
			keepers.MintKeeper,
			keepers.BankKeeper,
			keepers.StakingKeeper,
		)
		if err != nil {
			panic(fmt.Sprintf("failed to revert tombstoning: %s", err))
		}

		ctx.Logger().Info("Running module migrations for v1.4...")

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
