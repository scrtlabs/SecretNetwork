package v1_4

import (
	store "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/enigmampc/SecretNetwork/app/upgrades"
)

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

const UpgradeName = "v1.4"

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		// Assaf: Set version map for all modules because for some
		// reason it's not already set in upgradekeepr.
		// We upgrade from cosmos-sdk v0.44.5 to v0.45.4 and ibc-go v1.1.5 to v3.0.0
		// There were no ConsensusVersion changes between these versions
		// so we should be safe to use the current ConsensusVersion() for each moudle
		for moduleName := range mm.Modules {
			vm[moduleName] = mm.Modules[moduleName].ConsensusVersion()
		}

		ctx.Logger().Info("Starting to run module migrations...")

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
