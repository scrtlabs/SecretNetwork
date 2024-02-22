package app

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	authz "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	"cosmossdk.io/x/evidence"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"cosmossdk.io/x/upgrade"
	// upgradeclient "cosmossdk.io/x/upgrade/client"
	ica "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts"
	ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	ibcclient "github.com/cosmos/ibc-go/v8/modules/core/02-client/client"
	ibcswitch "github.com/scrtlabs/SecretNetwork/x/emergencybutton"

	packetforwardrouter "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/router"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	icaauth "github.com/scrtlabs/SecretNetwork/x/mauth"
	"github.com/scrtlabs/SecretNetwork/x/registration"
)

var mbasics = module.NewBasicManager(
	append([]module.AppModuleBasic{
		authz.AppModuleBasic{},
		auth.AppModuleBasic{},
		genutil.AppModuleBasic{},
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distr.AppModuleBasic{},
		gov.NewAppModuleBasic(
			paramsclient.ProposalHandler,
			distrclient.ProposalHandler,
			// upgradeclient.ProposalHandler,
			// upgradeclient.CancelProposalHandler,
			ibcclient.UpdateClientProposalHandler,
			ibcclient.UpgradeProposalHandler,
		),
		params.AppModuleBasic{},
		crisis.AppModuleBasic{},
		slashing.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		transfer.AppModuleBasic{},
		vesting.AppModuleBasic{},
		feegrantmodule.AppModuleBasic{},

		// ibc modules
		ibc.AppModuleBasic{},
		ica.AppModuleBasic{},
		packetforwardrouter.AppModuleBasic{},
		ibcfee.AppModuleBasic{},
	},
		// our stuff
		customModuleBasics()...,
	)...,
)

func customKVStoreKeys() []string {
	return []string{
		compute.StoreKey,
		registration.StoreKey,
		icaauth.StoreKey,
	}
}

func customModuleBasics() []module.AppModuleBasic {
	return []module.AppModuleBasic{
		compute.AppModuleBasic{},
		registration.AppModuleBasic{},
		icaauth.AppModuleBasic{},
		ibcswitch.AppModuleBasic{},
	}
}

// ModuleBasics returns all app modules basics
func ModuleBasics() module.BasicManager {
	return mbasics
}

// EncodingConfig specifies the concrete encoding types to use for a given app.
// This is provided for compatibility between protobuf and amino implementations.
type EncodingConfig struct {
	InterfaceRegistry types.InterfaceRegistry
	Marshaler         codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

func MakeEncodingConfig() EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txCfg := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	std.RegisterInterfaces(interfaceRegistry)
	std.RegisterLegacyAminoCodec(amino)

	ModuleBasics().RegisterLegacyAminoCodec(amino)
	ModuleBasics().RegisterInterfaces(interfaceRegistry)
	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          txCfg,
		Amino:             amino,
	}
}
