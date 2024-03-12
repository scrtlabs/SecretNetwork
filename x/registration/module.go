package registration

import (
	"context"
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/client"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

var (
	_ module.AppModule              = AppModule{}
	_ module.HasName                = AppModule{}
	_ module.HasConsensusVersion    = AppModule{}
	_ module.HasGenesis             = AppModule{}
	_ module.HasServices            = AppModule{}
	_ module.AppModuleBasic         = AppModuleBasic{}
)

// AppModuleBasic defines the basic application module used by the compute module.
type AppModuleBasic struct{}

func (b AppModuleBasic) RegisterLegacyAminoCodec(amino *codec.LegacyAmino) {
	RegisterCodec(amino)
}

func (b AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, serveMux *runtime.ServeMux) {
	types.RegisterQueryHandlerClient(context.Background(), serveMux, types.NewQueryClient(clientCtx)) //nolint:errcheck
}

// Name returns the compute module's name.
func (AppModuleBasic) Name() string {
	return ModuleName
}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 { return 1 }

// DefaultGenesis returns default genesis state as raw bytes for the compute
// module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(&GenesisState{
		NodeExchMasterKey: &MasterKey{},
		IoMasterKey:       &MasterKey{},
	})
}

// ValidateGenesis performs genesis state validation for the compute module.
func (AppModuleBasic) ValidateGenesis(marshaler codec.JSONCodec, config client.TxEncodingConfig, message json.RawMessage) error { //nolint:all
	var data GenesisState
	err := marshaler.UnmarshalJSON(message, &data)
	if err != nil {
		return err
	}
	return ValidateGenesis(data)
}

// RegisterInterfaceTypes implements InterfaceModule
func (b AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

// ____________________________________________________________________________

// AppModule implements an application module for the compute module.
type AppModule struct {
	AppModuleBasic
	keeper Keeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(keeper Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{},
		keeper:         keeper,
	}
}

func (am AppModule) RegisterServices(configurator module.Configurator) {
	types.RegisterQueryServer(configurator.QueryServer(), NewQuerier(am.keeper))
}

// InitGenesis performs genesis initialization for the compute module. It returns
// no validator updates.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var genesisState GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	InitGenesis(ctx, am.keeper, genesisState)
}

// ExportGenesis returns the exported genesis state as raw bytes for the compute
// module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := ExportGenesis(ctx, am.keeper)
	return cdc.MustMarshalJSON(gs)
}

// IsAppModule implements the appmodule.AppModule interface.
func (AppModule) IsAppModule() {}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}
