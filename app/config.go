package app

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	authz "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/capability"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	feegrantmodule "github.com/cosmos/cosmos-sdk/x/feegrant/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	transfer "github.com/cosmos/ibc-go/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/modules/core"
	ibchost "github.com/cosmos/ibc-go/modules/core/24-host"

	//transfer "github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer"
	//ibctransfertypes "github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer/types"
	//ibc "github.com/cosmos/cosmos-sdk/x/ibc/core"
	//ibchost "github.com/cosmos/cosmos-sdk/x/ibc/core/24-host"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/enigmampc/SecretNetwork/x/compute"
	"github.com/enigmampc/SecretNetwork/x/registration"
)

var (
	mbasics = module.NewBasicManager(
		append([]module.AppModuleBasic{
			authz.AppModuleBasic{},
			// accounts, fees.
			auth.AppModuleBasic{},
			// genesis utilities
			genutil.AppModuleBasic{},
			// tokens, token balance.
			bank.AppModuleBasic{},
			capability.AppModuleBasic{},
			// validator staking
			staking.AppModuleBasic{},
			// inflation
			mint.AppModuleBasic{},
			// distribution of fess and inflation
			distr.AppModuleBasic{},
			// governance functionality (voting)
			gov.NewAppModuleBasic(
				paramsclient.ProposalHandler, distrclient.ProposalHandler,
				upgradeclient.ProposalHandler, upgradeclient.CancelProposalHandler,
			),
			// chain parameters
			params.AppModuleBasic{},
			crisis.AppModuleBasic{},
			slashing.AppModuleBasic{},
			ibc.AppModuleBasic{},
			upgrade.AppModuleBasic{},
			evidence.AppModuleBasic{},
			transfer.AppModuleBasic{},
			vesting.AppModuleBasic{},
			feegrantmodule.AppModuleBasic{},
		},
			// our stuff
			customModuleBasics()...,
		)...,
	)
)

func customKVStoreKeys() []string {
	return []string{
		compute.StoreKey,
		registration.StoreKey,
	}
}

func customModuleBasics() []module.AppModuleBasic {
	return []module.AppModuleBasic{
		compute.AppModuleBasic{},
		registration.AppModuleBasic{},
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

func kvStoreKeys() map[string]*sdk.KVStoreKey {
	return sdk.NewKVStoreKeys(
		append([]string{
			authtypes.StoreKey,
			banktypes.StoreKey,
			stakingtypes.StoreKey,
			minttypes.StoreKey,
			distrtypes.StoreKey,
			slashingtypes.StoreKey,
			govtypes.StoreKey,
			paramstypes.StoreKey,
			ibchost.StoreKey,
			upgradetypes.StoreKey,
			evidencetypes.StoreKey,
			ibctransfertypes.StoreKey,
			capabilitytypes.StoreKey,
		},
			customKVStoreKeys()...,
		)...,
	)
}

func transientStoreKeys() map[string]*sdk.TransientStoreKey {
	return sdk.NewTransientStoreKeys(paramstypes.TStoreKey)
}

func memStoreKeys() map[string]*sdk.MemoryStoreKey {
	return sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)

}
