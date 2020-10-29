package tokenswap

import (
	"encoding/json"
	"fmt"

	"github.com/enigmampc/SecretNetwork/x/tokenswap/client"
	"github.com/enigmampc/SecretNetwork/x/tokenswap/types"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
)

// AppModuleBasic defines the basic application module used by the tokenswap module.
type AppModuleBasic struct{}

var _ module.AppModuleBasic = AppModuleBasic{}

// Name returns the tokenswap module's name.
func (AppModuleBasic) Name() string {
	return ModuleName
}

// RegisterCodec registers the tokenswap module's types for the given codec.
func (AppModuleBasic) RegisterCodec(cdc *codec.Codec) {
	RegisterCodec(cdc)
}

// DefaultGenesis returns default genesis state as raw bytes for the distribution
// module.
func (amb AppModuleBasic) DefaultGenesis() json.RawMessage {
	return ModuleCdc.MustMarshalJSON(DefaultGenesisState())
}

// ValidateGenesis performs genesis state validation for the tokenswap module.
func (AppModuleBasic) ValidateGenesis(bz json.RawMessage) error {
	var data GenesisState
	if err := ModuleCdc.UnmarshalJSON(bz, &data); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", ModuleName, err)
	}
	return types.ValidateGenesis(data)
}

// RegisterRESTRoutes registers the REST routes for the tokenswap module.
func (AppModuleBasic) RegisterRESTRoutes(ctx context.CLIContext, rtr *mux.Router) {
	client.RegisterRESTRoutes(ctx, rtr, StoreKey)
}

// GetTxCmd returns the root tx command for the tokenswap module.
func (AppModuleBasic) GetTxCmd(cdc *codec.Codec) *cobra.Command {
	return client.GetTxCmd(StoreKey, cdc)
}

// GetQueryCmd returns no root query command for the tokenswap module.
func (AppModuleBasic) GetQueryCmd(cdc *codec.Codec) *cobra.Command {
	return client.GetQueryCmd(StoreKey, cdc)
}

//____________________________________________________________________________

// AppModuleSimulation defines the module simulation functions used by the tokenswap module.
type AppModuleSimulation struct{}

// AppModule implements an application module for the tokenswap module.
type AppModule struct {
	AppModuleBasic
	AppModuleSimulation

	swapKeeper    SwapKeeper
	supplyKeeper  SupplyKeeper
	accountKeeper AccountKeeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(
	swapKeeper SwapKeeper,
	supplyKeeper SupplyKeeper,
	accountKeeper AccountKeeper,
) AppModule {

	return AppModule{
		AppModuleBasic:      AppModuleBasic{},
		AppModuleSimulation: AppModuleSimulation{},

		supplyKeeper:  supplyKeeper,
		accountKeeper: accountKeeper,
		swapKeeper:    swapKeeper,
	}
}

// Name returns the tokenswap module's name.
func (AppModule) Name() string {
	return ModuleName
}

// RegisterInvariants registers the tokenswap module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
}

// Route returns the message routing key for the tokenswap module.
func (AppModule) Route() string {
	return RouterKey
}

// NewHandler returns an sdk.Handler for the tokenswap module.
func (am AppModule) NewHandler() sdk.Handler {
	return NewHandler(am.swapKeeper)
}

// QuerierRoute returns the tokenswap module's querier route name.
func (AppModule) QuerierRoute() string {
	return QuerierRoute
}

// NewQuerierHandler returns the tokenswap module sdk.Querier.
func (am AppModule) NewQuerierHandler() sdk.Querier {
	return NewQuerier(am.swapKeeper, ModuleCdc)
}

// InitGenesis performs genesis initialization for the tokenswap module.
func (am AppModule) InitGenesis(ctx sdk.Context, data json.RawMessage) []abci.ValidatorUpdate {
	var genesisState GenesisState
	ModuleCdc.MustUnmarshalJSON(data, &genesisState)
	InitGenesis(ctx, am.supplyKeeper, am.swapKeeper, genesisState)
	return []abci.ValidatorUpdate{}
}

// ExportGenesis returns the exported genesis state as raw bytes for the tokenswap
// module.
func (am AppModule) ExportGenesis(ctx sdk.Context) json.RawMessage {
	gs := ExportGenesis(ctx, am.swapKeeper)
	return ModuleCdc.MustMarshalJSON(gs)
}

// BeginBlock returns the begin blocker for the tokenswap module.
func (AppModule) BeginBlock(_ sdk.Context, _ abci.RequestBeginBlock) {}

// EndBlock returns the end blocker for the tokenswap module. It returns no validator
// updates.
func (am AppModule) EndBlock(ctx sdk.Context, _ abci.RequestEndBlock) []abci.ValidatorUpdate {
	return nil
}
