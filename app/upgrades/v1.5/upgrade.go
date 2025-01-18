package v1_4

import (
	"context"
	"os"

	"cosmossdk.io/log"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
)

const upgradeName = "v1.5"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
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

		logger.Info("Running module migrations for v1.5...")
		return mm.RunMigrations(ctx, configurator, vm)
	}
}
