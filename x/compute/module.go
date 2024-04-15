package compute

import (
	"context"
	"encoding/json"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/scrtlabs/SecretNetwork/x/compute/internal/keeper"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

var (
	_ module.AppModule           = AppModule{}
	_ module.HasName             = AppModule{}
	_ module.HasGenesis          = AppModule{}
	_ module.HasServices         = AppModule{}
	_ module.HasConsensusVersion = AppModule{}
	_ module.AppModuleBasic      = AppModuleBasic{}
)

// AppModuleBasic defines the basic application module used by the compute module.
type AppModuleBasic struct{}

func (b AppModuleBasic) RegisterLegacyAminoCodec(amino *codec.LegacyAmino) {
	RegisterCodec(amino)
}

func (b AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, serveMux *runtime.ServeMux) {
	_ = types.RegisterQueryHandlerClient(context.Background(), serveMux, types.NewQueryClient(clientCtx))
}

// Name returns the compute module's name.
func (AppModuleBasic) Name() string {
	return ModuleName
}

// DefaultGenesis returns default genesis state as raw bytes for the compute
// module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(&GenesisState{
		// Params: DefaultParams(),
	})
}

// ValidateGenesis performs genesis state validation for the compute module.
func (AppModuleBasic) ValidateGenesis(marshaler codec.JSONCodec, _ client.TxEncodingConfig, message json.RawMessage) error {
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

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 { return 5 }

func (am AppModule) RegisterServices(configurator module.Configurator) {
	types.RegisterMsgServer(configurator.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(configurator.QueryServer(), NewQuerier(am.keeper))

	m := keeper.NewMigrator(am.keeper)
	err := configurator.RegisterMigration(types.ModuleName, 1, m.Migrate1to2)
	if err != nil {
		panic(err)
	}
	err = configurator.RegisterMigration(types.ModuleName, 2, m.Migrate2to3)
	if err != nil {
		panic(err)
	}

	err = configurator.RegisterMigration(types.ModuleName, 3, m.Migrate3to4)
	if err != nil {
		panic(err)
	}

	err = configurator.RegisterMigration(types.ModuleName, 4, m.Migrate4to5)
	if err != nil {
		panic(err)
	}
}

// InitGenesis performs genesis initialization for the compute module. It returns
// no validator updates.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var genesisState GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	if err := InitGenesis(ctx, am.keeper, genesisState); err != nil {
		panic(err)
	}
}

// ExportGenesis returns the exported genesis state as raw bytes for the compute
// module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := ExportGenesis(ctx, am.keeper)
	return cdc.MustMarshalJSON(gs)
}

// BeginBlock returns the begin blocker for the compute module.
/*
// TODO:FIX
func (am AppModule) BeginBlock(ctx sdk.Context, beginBlock abci.RequestBeginBlock) {
	header, err := beginBlock.Header.Marshal()
	if err != nil {
		ctx.Logger().Error("Failed to marshal header")
		panic(err)
	}

	// There is a possibility, specifically was found on upgrade block, when there are no pre-commits at all (beginBlock.Commit == nil)
	// In this case Marshal will fail with a Seg Fault.
	// The fix below it a temporary fix until we will investigate the issue in tendermint.
	if beginBlock.Commit == nil {
		ctx.Logger().Info(fmt.Sprintf("Skipping commit submission to the enclave for block %d\n", beginBlock.Header.Height))
		return
	}

	commit, err := beginBlock.Commit.Marshal()
	if err != nil {
		ctx.Logger().Error("Failed to marshal commit")
		panic(err)
	}

	data, err := beginBlock.Data.Marshal()
	if err != nil {
		ctx.Logger().Error("Failed to marshal data")
		panic(err)
	}

	if beginBlock.Header.EncryptedRandom != nil {
		randomAndProof := append(beginBlock.Header.EncryptedRandom.Random, beginBlock.Header.EncryptedRandom.Proof...) //nolint:all
		random, err := api.SubmitBlockSignatures(header, commit, data, randomAndProof)
		if err != nil {
			ctx.Logger().Error("Failed to submit block signatures")
			panic(err)
		}

		am.keeper.SetRandomSeed(ctx, random)
	} else {
		println("No random got from TM header")
	}
}*/

// IsAppModule implements the appmodule.AppModule interface.
func (AppModule) IsAppModule() {}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}
