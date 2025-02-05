package v1_11

import (
	"context"
	"fmt"
	"os"

	"cosmossdk.io/log"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
	ibchookstypes "github.com/scrtlabs/SecretNetwork/x/ibc-hooks/types"
)

const upgradeName = "v1.11"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added: []string{
			ibchookstypes.StoreKey,
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

		// WASM Hooks doesn't require any initialization code:
		// https://github.com/osmosis-labs/osmosis/blob/8b4c62a26/app/upgrades/v14/upgrades.go#L12-L21

		logger.Info(fmt.Sprintf("Running module migrations for %s...", upgradeName))
		return mm.RunMigrations(ctx, configurator, vm)
	}
}
