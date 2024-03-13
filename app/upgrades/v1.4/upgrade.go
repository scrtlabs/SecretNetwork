package v1_4

import (
	"context"
	"fmt"
	"os"

	"cosmossdk.io/log"
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
	return func(ctx context.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		logger := log.NewLogger(os.Stderr)
		// We're not upgrading cosmos-sdk, Tendermint or ibc-go, so no ConsensusVersion changes
		// Therefore mm.RunMigrations() should not find any module to upgrade

		logger.Info("Running revert of tombstoning")
		err := RevertCosTombstoning(
			sdk.UnwrapSDKContext(ctx),
			keepers.SlashingKeeper,
			keepers.MintKeeper,
			keepers.BankKeeper,
			keepers.StakingKeeper,
		)
		if err != nil {
			panic(fmt.Sprintf("failed to revert tombstoning: %s", err))
		}

		logger.Info("Running module migrations for v1.4...")

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
