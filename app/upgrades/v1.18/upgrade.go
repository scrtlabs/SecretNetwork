package v1_18

import (
	"context"
	"fmt"
	"os"

	"cosmossdk.io/log"
	store "cosmossdk.io/store/types"
	circuittypes "cosmossdk.io/x/circuit/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
)

const upgradeName = "v1.18"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added: []string{
			circuittypes.StoreKey,
		},
		Deleted: []string{
			crisistypes.StoreKey,
		},
	},
}

func createUpgradeHandler(mm *module.Manager, _ *keepers.SecretAppKeepers, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		logger := log.NewLogger(os.Stderr)
		logger.Info(` _    _ _____   _____ _____            _____  ______ `)
		logger.Info(`| |  | |  __ \ / ____|  __ \     /\   |  __ \|  ____|`)
		logger.Info(`| |  | | |__) | |  __| |__) |   /  \  | |  | | |__   `)
		logger.Info(`| |  | |  ___/| | |_ |  _  /   / /\ \ | |  | |  __|  `)
		logger.Info(`| |__| | |    | |__| | | \ \  / ____ \| |__| | |____ `)
		logger.Info(` \____/|_|     \_____|_|  \_\/_/    \_\_____/|______|`)

		logger.Info(fmt.Sprintf("Running module migrations for %s...", upgradeName))

		// Migration from legacy must have already be done BEFORE running this upgrade

		// _, err := api.MigrationOp(4)
		// if err != nil {
		// 	return nil, err
		// }

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
