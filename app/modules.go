package app

import (
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	feegrantmodule "github.com/cosmos/cosmos-sdk/x/feegrant/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	packetforward "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v4/router"
	ica "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts"
	icatypes "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/types"
	ibcfee "github.com/cosmos/ibc-go/v4/modules/apps/29-fee"
	ibcfeetypes "github.com/cosmos/ibc-go/v4/modules/apps/29-fee/types"
	"github.com/cosmos/ibc-go/v4/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v4/modules/core"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	ibcswitch "github.com/scrtlabs/SecretNetwork/x/emergencybutton"
	icaauth "github.com/scrtlabs/SecretNetwork/x/mauth"
	reg "github.com/scrtlabs/SecretNetwork/x/registration"
)

var ModuleAccountPermissions = map[string][]string{
	authtypes.FeeCollectorName:     nil,
	distrtypes.ModuleName:          nil,
	minttypes.ModuleName:           {authtypes.Minter},
	stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
	stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	govtypes.ModuleName:            {authtypes.Burner},
	ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
	icatypes.ModuleName:            nil,
	ibcfeetypes.ModuleName:         nil,
	ibcswitch.ModuleName:           nil,
	compute.ModuleName:             {authtypes.Burner},
}

func Modules(
	app *SecretNetworkApp,
	encodingConfig EncodingConfig,
	skipGenesisInvariants bool,
) []module.AppModule {
	appCodec := encodingConfig.Marshaler

	return []module.AppModule{
		genutil.NewAppModule(app.AppKeepers.AccountKeeper, app.AppKeepers.StakingKeeper, app.BaseApp.DeliverTx, encodingConfig.TxConfig),
		auth.NewAppModule(appCodec, *app.AppKeepers.AccountKeeper, authsims.RandomGenesisAccounts),
		vesting.NewAppModule(*app.AppKeepers.AccountKeeper, app.AppKeepers.BankKeeper),
		bank.NewAppModule(appCodec, *app.AppKeepers.BankKeeper, app.AppKeepers.AccountKeeper),
		capability.NewAppModule(appCodec, *app.AppKeepers.CapabilityKeeper),
		crisis.NewAppModule(app.AppKeepers.CrisisKeeper, skipGenesisInvariants),
		feegrantmodule.NewAppModule(appCodec, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, *app.AppKeepers.FeegrantKeeper, app.GetInterfaceRegistry()), gov.NewAppModule(app.GetCodec(), *app.AppKeepers.GovKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper),
		mint.NewAppModule(appCodec, *app.AppKeepers.MintKeeper, app.AppKeepers.AccountKeeper),
		slashing.NewAppModule(appCodec, *app.AppKeepers.SlashingKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, *app.AppKeepers.StakingKeeper),
		distr.NewAppModule(appCodec, *app.AppKeepers.DistrKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, *app.AppKeepers.StakingKeeper),
		staking.NewAppModule(appCodec, *app.AppKeepers.StakingKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper),
		upgrade.NewAppModule(*app.AppKeepers.UpgradeKeeper),
		evidence.NewAppModule(*app.AppKeepers.EvidenceKeeper),
		compute.NewAppModule(*app.AppKeepers.ComputeKeeper),
		params.NewAppModule(*app.AppKeepers.ParamsKeeper),
		authzmodule.NewAppModule(appCodec, *app.AppKeepers.AuthzKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, app.GetInterfaceRegistry()),
		reg.NewAppModule(*app.AppKeepers.RegKeeper),
		ibc.NewAppModule(app.AppKeepers.IbcKeeper),
		transfer.NewAppModule(app.AppKeepers.TransferKeeper),
		ica.NewAppModule(app.AppKeepers.ICAControllerKeeper, app.AppKeepers.ICAHostKeeper),
		packetforward.NewAppModule(app.AppKeepers.PacketForwardKeeper),
		ibcfee.NewAppModule(app.AppKeepers.IbcFeeKeeper),
		ibcswitch.NewAppModule(app.AppKeepers.IbcSwitchKeeper),
		icaauth.NewAppModule(appCodec, *app.AppKeepers.ICAAuthKeeper),
	}
}
