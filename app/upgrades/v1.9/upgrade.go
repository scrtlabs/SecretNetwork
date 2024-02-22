package v1_9

import (
	"fmt"

	store "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ibcpacketforwardtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v4/router/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v4/modules/apps/29-fee/types"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
	ibcswitchtypes "github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

const upgradeName = "v1.9"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          upgradeName,
	CreateUpgradeHandler: createUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added: []string{
			ibcpacketforwardtypes.StoreKey,
			ibcfeetypes.ModuleName,
			ibcswitchtypes.ModuleName,
		},
	},
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

		vm[ibcfeetypes.ModuleName] = mm.Modules[ibcfeetypes.ModuleName].ConsensusVersion()
		ctx.Logger().Info(fmt.Sprintf("ibcfee module version %s set", fmt.Sprint(vm[ibcfeetypes.ModuleName])))

		keepers.PacketForwardKeeper.SetParams(ctx, ibcpacketforwardtypes.DefaultParams())
		keepers.IbcSwitchKeeper.SetParams(ctx, ibcswitchtypes.DefaultParams())

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
