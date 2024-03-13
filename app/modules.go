package app

import (
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/ibc-go/modules/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"cosmossdk.io/x/evidence"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"cosmossdk.io/x/upgrade"
	packetforward "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward"
	packetforwardtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	ica "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
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
	appCodec := encodingConfig.Codec

	return []module.AppModule{
		genutil.NewAppModule(app.AppKeepers.AccountKeeper, app.AppKeepers.StakingKeeper, app, encodingConfig.TxConfig),
		auth.NewAppModule(appCodec, *app.AppKeepers.AccountKeeper, authsims.RandomGenesisAccounts, app.AppKeepers.GetSubspace(authtypes.ModuleName)),
		vesting.NewAppModule(*app.AppKeepers.AccountKeeper, app.AppKeepers.BankKeeper),
		bank.NewAppModule(appCodec, *app.AppKeepers.BankKeeper, app.AppKeepers.AccountKeeper, app.AppKeepers.GetSubspace(banktypes.ModuleName)),
		capability.NewAppModule(appCodec, *app.AppKeepers.CapabilityKeeper, false),
		crisis.NewAppModule(app.AppKeepers.CrisisKeeper, skipGenesisInvariants, app.AppKeepers.GetSubspace(crisistypes.ModuleName)),
		feegrantmodule.NewAppModule(appCodec, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, *app.AppKeepers.FeegrantKeeper, app.GetInterfaceRegistry()),
		gov.NewAppModule(appCodec, app.AppKeepers.GovKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, app.AppKeepers.GetSubspace(govtypes.ModuleName)),
		mint.NewAppModule(appCodec, *app.AppKeepers.MintKeeper, app.AppKeepers.AccountKeeper, nil, app.AppKeepers.GetSubspace(minttypes.ModuleName)),
		slashing.NewAppModule(appCodec, *app.AppKeepers.SlashingKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, *app.AppKeepers.StakingKeeper, app.AppKeepers.GetSubspace(slashingtypes.ModuleName), app.GetInterfaceRegistry()),
		distr.NewAppModule(appCodec, *app.AppKeepers.DistrKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, *app.AppKeepers.StakingKeeper, app.AppKeepers.GetSubspace(distrtypes.ModuleName)),
		staking.NewAppModule(appCodec, app.AppKeepers.StakingKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, app.AppKeepers.GetSubspace(stakingtypes.ModuleName)),
		upgrade.NewAppModule(app.AppKeepers.UpgradeKeeper, app.AppKeepers.AccountKeeper.AddressCodec()),
		evidence.NewAppModule(*app.AppKeepers.EvidenceKeeper),
		compute.NewAppModule(*app.AppKeepers.ComputeKeeper),
		params.NewAppModule(*app.AppKeepers.ParamsKeeper),
		authzmodule.NewAppModule(appCodec, *app.AppKeepers.AuthzKeeper, app.AppKeepers.AccountKeeper, *app.AppKeepers.BankKeeper, app.GetInterfaceRegistry()),
		reg.NewAppModule(*app.AppKeepers.RegKeeper),
		ibc.NewAppModule(app.AppKeepers.IbcKeeper),
		transfer.NewAppModule(app.AppKeepers.TransferKeeper),
		ica.NewAppModule(app.AppKeepers.ICAControllerKeeper, app.AppKeepers.ICAHostKeeper),
		packetforward.NewAppModule(app.AppKeepers.PacketForwardKeeper, app.AppKeepers.GetSubspace(packetforwardtypes.ModuleName)),
		ibcfee.NewAppModule(app.AppKeepers.IbcFeeKeeper),
		ibcswitch.NewAppModule(app.AppKeepers.IbcSwitchKeeper),
		icaauth.NewAppModule(appCodec, *app.AppKeepers.ICAAuthKeeper),
	}
}
