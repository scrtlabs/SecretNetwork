package ibc_switch

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/scrtlabs/SecretNetwork/x/ibc-switch/types"
)

// InitGenesis initializes the x/ibc-switch's module's state from a provided genesis
// state, which includes the parameter for the pauser address and for the switch status.
func (i *ICS4Wrapper) InitGenesis(ctx sdk.Context, genState types.GenesisState) {
	i.SetParams(ctx, genState.Params)
}

// ExportGenesis returns the x/ibc-switch module's exported genesis.
func (i *ICS4Wrapper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params: i.GetParams(ctx),
	}
}
