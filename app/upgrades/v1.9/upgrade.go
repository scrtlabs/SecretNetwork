package v1_9

import (
	"fmt"
	store "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v4/modules/apps/29-fee/types"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
	"github.com/scrtlabs/SecretNetwork/x/mauth"
	ibcpacketforwardtypes "github.com/strangelove-ventures/packet-forward-middleware/v4/router/types"
)

const upgradeName = "v1.9"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	// todo: mauth
	StoreUpgrades: store.StoreUpgrades{Added: []string{ibcpacketforwardtypes.StoreKey, ibcfeetypes.ModuleName, mauth.ModuleName}},
}

func createUpgradeHandler(mm *module.Manager, keepers *keepers.SecretAppKeepers, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		ctx.Logger().Info(` _    _ _____   _____ _____            _____  ______ `)
		ctx.Logger().Info(`| |  | |  __ \ / ____|  __ \     /\   |  __ \|  ____|`)
		ctx.Logger().Info(`| |  | | |__) | |  __| |__) |   /  \  | |  | | |__   `)
		ctx.Logger().Info(`| |  | |  ___/| | |_ |  _  /   / /\ \ | |  | |  __|  `)
		ctx.Logger().Info(`| |__| | |    | |__| | | \ \  / ____ \| |__| | |____ `)
		ctx.Logger().Info(` \____/|_|     \_____|_|  \_\/_/    \_\_____/|______|`)

		ctx.Logger().Info(fmt.Sprintf("Running module migrations for %s...", upgradeName))

		keepers.IbcRouterKeeper.SetParams(ctx, ibcpacketforwardtypes.DefaultParams())

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
