package v1_3

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

const UpgradeName = "v1.3"

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {

		// Assaf: Set version map for all modules because for some
		// reason it's not already set in upgradekeepr.
		for moduleName := range mm.Modules {
			vm[moduleName] = mm.Modules[moduleName].ConsensusVersion()
		}

		ctx.Logger().Info("Starting to run module migrations...")

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
