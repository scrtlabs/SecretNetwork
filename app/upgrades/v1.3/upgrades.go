package v1_3

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	icamodule "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts"
	icacontrollertypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/types"
)

const UpgradeName = "v1.3"

func CreateUpgradeHandler(mm *module.Manager, icamodule *icamodule.AppModule, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {

		vm[icatypes.ModuleName] = icamodule.ConsensusVersion()

		// create ICS27 Controller submodule params
		controllerParams := icacontrollertypes.Params{
			ControllerEnabled: true,
		}

		// create ICS27 Host submodule params
		hostParams := icahosttypes.Params{
			HostEnabled: true,
			AllowMessages: []string{
				"/cosmos.bank.v1beta1.MsgSend",
				"/cosmos.staking.v1beta1.MsgDelegate",
				"/cosmos.staking.v1beta1.MsgUndelegate",
				"/cosmos.gov.v1beta1.Vote"},
		}

		// initialize ICS27 module
		icamodule.InitModule(ctx, controllerParams, hostParams)

		// otherwise we run this, which will run wasm.InitGenesis(wasm.DefaultGenesis())
		// and then override it after
		newVM, err := mm.RunMigrations(ctx, configurator, vm)
		if err != nil {
			return newVM, err
		}

		// override here
		return newVM, err
	}
}
