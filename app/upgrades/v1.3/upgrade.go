package v1_3

import (
	store "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	icamodule "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts"
	icacontrollertypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/types"
	"github.com/scrtlabs/SecretNetwork/app/keepers"
	"github.com/scrtlabs/SecretNetwork/app/upgrades"
)

const UpgradeName = "v1.3"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades:        store.StoreUpgrades{Added: []string{icahosttypes.StoreKey}},
}

func CreateUpgradeHandler(mm *module.Manager, _ *keepers.SecretAppKeepers, configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx sdk.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		// Assaf: Set version map for all modules because for some
		// reason it's not already set in upgradekeepr.
		// We upgrade from cosmos-sdk v0.44.5 to v0.45.4 and ibc-go v1.1.5 to v3.0.0
		// There were no ConsensusVersion changes between these versions
		// so we should be safe to use the current ConsensusVersion() for each moudle
		for moduleName := range mm.Modules {
			vm[moduleName] = mm.Modules[moduleName].ConsensusVersion()
		}

		vm[icatypes.ModuleName] = mm.Modules[icatypes.ModuleName].ConsensusVersion()

		// create ICS27 Controller submodule params
		controllerParams := icacontrollertypes.Params{
			ControllerEnabled: false,
		}

		// create ICS27 Host submodule params
		hostParams := icahosttypes.Params{
			HostEnabled: true,
			AllowMessages: []string{
				"/cosmos.authz.v1beta1.MsgExec",
				"/cosmos.authz.v1beta1.MsgGrant",
				"/cosmos.authz.v1beta1.MsgRevoke",
				"/cosmos.bank.v1beta1.MsgSend",
				"/cosmos.bank.v1beta1.MsgMultiSend",
				"/cosmos.distribution.v1beta1.MsgSetWithdrawAddress",
				"/cosmos.distribution.v1beta1.MsgWithdrawValidatorCommission",
				"/cosmos.distribution.v1beta1.MsgFundCommunityPool",
				"/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward",
				"/cosmos.feegrant.v1beta1.MsgGrantAllowance",
				"/cosmos.feegrant.v1beta1.MsgRevokeAllowance",
				"/cosmos.gov.v1beta1.MsgVoteWeighted",
				"/cosmos.gov.v1beta1.MsgSubmitProposal",
				"/cosmos.gov.v1beta1.MsgDeposit",
				"/cosmos.gov.v1beta1.MsgVote",
				"/cosmos.staking.v1beta1.MsgDelegate",
				"/cosmos.staking.v1beta1.MsgUndelegate",
				"/cosmos.staking.v1beta1.MsgBeginRedelegate",
				"/ibc.applications.transfer.v1.MsgTransfer",
			},
		}

		ctx.Logger().Info("Starting to init interchainaccount module...")

		// initialize ICS27 module
		// icamodule.InitModule(ctx, controllerParams, hostParams)

		// initialize ICS27 module
		icamoduleInstance, correctTypecast := mm.Modules[icatypes.ModuleName].(icamodule.AppModule)
		if !correctTypecast {
			panic("mm.Modules[icatypes.ModuleName] is not of type ica.AppModule")
		}

		icamoduleInstance.InitModule(ctx, controllerParams, hostParams)

		ctx.Logger().Info("Starting to run module migrations...")

		return mm.RunMigrations(ctx, configurator, vm)
	}
}
