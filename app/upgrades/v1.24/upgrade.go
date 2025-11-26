package v1_24

import (
	"context"
	"fmt"
	"os"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
	minttypes "github.com/scrtlabs/SecretNetwork/x/mint/types"
)

const upgradeName = "v1.24"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

func createUpgradeHandler(mm *module.Manager, keepers *keepers.SecretAppKeepers, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		logger := log.NewLogger(os.Stderr)
		logger.Info(` _    _ _____   _____ _____            _____  ______ `)
		logger.Info(`| |  | |  __ \ / ____|  __ \     /\   |  __ \|  ____|`)
		logger.Info(`| |  | | |__) | |  __| |__) |   /  \  | |  | | |__   `)
		logger.Info(`| |  | |  ___/| | |_ |  _  /   / /\ \ | |  | |  __|  `)
		logger.Info(`| |__| | |    | |__| | | \ \  / ____ \| |__| | |____ `)
		logger.Info(` \____/|_|     \_____|_|  \_\/_/    \_\_____/|______|`)
		logger.Info("")
		logger.Info("🔥 SWITCHING TO FIXED BLOCK REWARDS 🔥")
		logger.Info("")

		logger.Info(fmt.Sprintf("Running module migrations for %s...", upgradeName))

		// Migrate mint module parameters from percentage-based to fixed block rewards
		logger.Info("Migrating mint module to fixed block rewards...")

		// Set new mint parameters with fixed block reward of 4 SCRT (4,000,000 uscrt)
		newParams := minttypes.NewParams(
			"uscrt",                      // MintDenom
			math.NewInt(4_000_000),       // FixedBlockReward: 4 SCRT = 4,000,000 uscrt
			uint64(60*60*24*365/6.3),     // BlocksPerYear: ~5,005,714 blocks/year
		)

		if err := keepers.MintKeeper.SetParams(ctx, newParams); err != nil {
			return nil, fmt.Errorf("failed to set mint params: %w", err)
		}

		// Initialize minter with annual provisions calculated from fixed block reward
		// AnnualProvisions = FixedBlockReward * BlocksPerYear
		annualProvisions := math.LegacyNewDecFromInt(newParams.FixedBlockReward).
			Mul(math.LegacyNewDec(int64(newParams.BlocksPerYear)))

		minter := minttypes.NewMinter(annualProvisions)
		if err := keepers.MintKeeper.SetMinter(ctx, minter); err != nil {
			return nil, fmt.Errorf("failed to set minter: %w", err)
		}

		logger.Info(fmt.Sprintf("✅ Mint module migrated successfully:"))
		logger.Info(fmt.Sprintf("   - Fixed block reward: %s uscrt (4 SCRT)", newParams.FixedBlockReward.String()))
		logger.Info(fmt.Sprintf("   - Blocks per year: %d", newParams.BlocksPerYear))
		logger.Info(fmt.Sprintf("   - Annual provisions: %s uscrt (~21M SCRT)", annualProvisions.TruncateInt().String()))
		logger.Info(fmt.Sprintf("   - Current inflation will decrease from ~9%% to ~6.2%%"))
		logger.Info(fmt.Sprintf("   - Inflation will continue to decrease over time as supply grows"))
		logger.Info("")

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
