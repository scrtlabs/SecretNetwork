package ibc_hooks

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"github.com/scrtlabs/SecretNetwork/x/ibc-hooks/types"

	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_ module.AppModule           = AppModule{}
	_ module.AppModuleBasic      = AppModuleBasic{}
	_ module.HasName             = AppModule{}
	_ module.HasConsensusVersion = AppModule{}
)

// AppModuleBasic defines the basic application module used by the ibc-hooks module.
type AppModuleBasic struct{}

// Name returns the ibc-hooks module's name.
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the ibc-hooks module's types on the given LegacyAmino codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

// RegisterInterfaces registers the module's interface types.
func (b AppModuleBasic) RegisterInterfaces(_ cdctypes.InterfaceRegistry) {}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the ibc-hooks module.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {} //nolint:all

// ___________________________________________________________________________

// AppModule implements an application module for the ibc-hooks module.
type AppModule struct {
	AppModuleBasic

	authKeeper AccountKeeper
}

// NewAppModule creates a new AppModule object.
func NewAppModule(ak AccountKeeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{},
		authKeeper:     ak,
	}
}

// Name returns the ibc-hooks module's name.
func (AppModule) Name() string {
	return types.ModuleName
}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 { return 1 }

type AccountKeeper interface {
	NewAccount(sdk.Context, sdk.AccountI) sdk.AccountI

	GetAccount(ctx sdk.Context, addr sdk.AccAddress) sdk.AccountI
	SetAccount(ctx sdk.Context, acc sdk.AccountI)
}

// IsAppModule implements the appmodule.AppModule interface.
func (AppModule) IsAppModule() {}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}
