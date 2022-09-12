package v1_4

import (
	store "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/enigmampc/SecretNetwork/app/upgrades"
	"github.com/enigmampc/SecretNetwork/x/usc"
	usctypes "github.com/enigmampc/SecretNetwork/x/usc/types"
)

const UpgradeName = "v1.4"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// We're not upgrading cosmos-sdk, Tendermint or ibc-go, so no ConsensusVersion changes
		// Therefore mm.RunMigrations() should not find any module to upgrade

		ctx.Logger().Info("Running module migrations for v1.4...")

		fromVM[usctypes.ModuleName] = mm.Modules[usctypes.ModuleName].ConsensusVersion()

		// create USC module params
		uscParams := usctypes.DefaultParams()

		ctx.Logger().Info("Initializing USC module...")

		uscmoduleInstance, correctTypecast := mm.Modules[usctypes.ModuleName].(usc.AppModule)
		if !correctTypecast {
			panic("mm.Modules[usctypes.ModuleName] is not of type usc.AppModule")
		}

		uscmoduleInstance.InitModule(ctx, uscParams)

		ctx.Logger().Info("Starting to run module migrations...")

		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
