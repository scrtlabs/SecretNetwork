package app

import (
	"cosmossdk.io/x/circuit"
	"cosmossdk.io/x/evidence"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/upgrade"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
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
	packetforward "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward"
	packetforwardtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	"github.com/cosmos/ibc-go/modules/capability"
	ica "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	ibctm "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	ibcswitch "github.com/scrtlabs/SecretNetwork/x/emergencybutton"
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
	appCodec codec.Codec,
) []module.AppModule {
	return []module.AppModule{
		genutil.NewAppModule(app.AppKeepers.AccountKeeper, app.AppKeepers.StakingKeeper, app, app.TxConfig()),
		auth.NewAppModule(appCodec, *app.AppKeepers.AccountKeeper, authsims.RandomGenesisAccounts, app.AppKeepers.GetSubspace(authtypes.ModuleName)),
		vesting.NewAppModule(*app.AppKeepers.AccountKeeper, app.AppKeepers.BankKeeper),
		bank.NewAppModule(appCodec, *app.AppKeepers.BankKeeper, app.AppKeepers.AccountKeeper, app.AppKeepers.GetSubspace(banktypes.ModuleName)),
		capability.NewAppModule(appCodec, *app.AppKeepers.CapabilityKeeper, false),
		consensus.NewAppModule(appCodec, app.AppKeepers.ConsensusParamsKeeper),
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
		circuit.NewAppModule(appCodec, *app.AppKeepers.CircuitKeeper),
		reg.NewAppModule(*app.AppKeepers.RegKeeper),
		ibc.NewAppModule(app.AppKeepers.IbcKeeper),
		ibctm.NewAppModule(),
		transfer.NewAppModule(app.AppKeepers.TransferKeeper),
		ica.NewAppModule(app.AppKeepers.ICAControllerKeeper, app.AppKeepers.ICAHostKeeper),
		packetforward.NewAppModule(app.AppKeepers.PacketForwardKeeper, app.AppKeepers.GetSubspace(packetforwardtypes.ModuleName)),
		ibcfee.NewAppModule(app.AppKeepers.IbcFeeKeeper),
		ibcswitch.NewAppModule(app.AppKeepers.IbcSwitchKeeper, app.AppKeepers.GetSubspace(ibcswitch.ModuleName)),
	}
}
