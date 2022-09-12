package v1_4_fix

import (
	store "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/enigmampc/SecretNetwork/app/upgrades"
)

const UpgradeName = "v1.4-fix"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		// We're not upgrading cosmos-sdk, Tendermint or ibc-go, so no ConsensusVersion changes
		// Therefore mm.RunMigrations() should not find any module to upgrade

		ctx.Logger().Info("Running module migrations for v1.4-fix...")

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
