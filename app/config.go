package app

import (
	"cosmossdk.io/x/evidence"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/upgrade"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	authz "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/ibc-go/modules/capability"
	ica "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts"
	ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	ibctm "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	ibcswitch "github.com/scrtlabs/SecretNetwork/x/emergencybutton"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	packetforwardrouter "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward"
	scrt "github.com/scrtlabs/SecretNetwork/types"
	"github.com/scrtlabs/SecretNetwork/x/compute"
	"github.com/scrtlabs/SecretNetwork/x/registration"
)

var mbasics = module.NewBasicManager(
	append([]module.AppModuleBasic{
		authz.AppModuleBasic{},
		auth.AppModuleBasic{},
		genutil.AppModuleBasic{
			GenTxValidator: genutiltypes.DefaultMessageValidator,
		},
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		consensus.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distr.AppModuleBasic{},
		gov.NewAppModuleBasic(
			[]govclient.ProposalHandler{
				paramsclient.ProposalHandler,
			},
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
		ibctm.AppModuleBasic{},
		ica.AppModuleBasic{},
		packetforwardrouter.AppModuleBasic{},
		ibcfee.AppModuleBasic{},
	},
		// our stuff
		customModuleBasics()...,
	)...,
)

func customModuleBasics() []module.AppModuleBasic {
	return []module.AppModuleBasic{
		compute.AppModuleBasic{},
		registration.AppModuleBasic{},
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
	InterfaceRegistry codectypes.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

type (
	CodecOptions struct {
		AccAddressPrefix string
		ValAddressPrefix string
	}
)

func (o CodecOptions) NewInterfaceRegistry() codectypes.InterfaceRegistry {
	accAddressPrefix := o.AccAddressPrefix
	if accAddressPrefix == "" {
		accAddressPrefix = scrt.Bech32PrefixAccAddr // sdk.GetConfig().GetBech32AccountAddrPrefix()
	}

	valAddressPrefix := o.ValAddressPrefix
	if valAddressPrefix == "" {
		valAddressPrefix = scrt.Bech32PrefixValAddr // sdk.GetConfig().GetBech32ValidatorAddrPrefix()
	}

	ir, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          address.NewBech32Codec(accAddressPrefix),
			ValidatorAddressCodec: address.NewBech32Codec(valAddressPrefix),
		},
	})
	if err != nil {
		panic(err)
	}
	return ir
}

func MakeEncodingConfig() EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := CodecOptions{
		AccAddressPrefix: scrt.Bech32PrefixAccAddr,
		ValAddressPrefix: scrt.Bech32PrefixValAddr,
	}.NewInterfaceRegistry()

	appCodec := codec.NewProtoCodec(interfaceRegistry)
	txCfg := tx.NewTxConfig(appCodec, tx.DefaultSignModes)

	sdk.RegisterLegacyAminoCodec(amino)
	sdk.RegisterInterfaces(interfaceRegistry)
	txtypes.RegisterInterfaces(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)

	ModuleBasics().RegisterLegacyAminoCodec(amino)
	ModuleBasics().RegisterInterfaces(interfaceRegistry)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             appCodec,
		TxConfig:          txCfg,
		Amino:             amino,
	}
}
