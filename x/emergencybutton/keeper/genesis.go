package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

// InitGenesis initializes the x/emergencybutton's module's state from a provided genesis
// state, which includes the parameter for the pauser address and for the switch status.
func (i *Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) {
	i.SetParams(ctx, genState.Params) //nolint:errcheck
}

// ExportGenesis returns the x/emergencybutton module's exported genesis.
func (i *Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params: i.GetParams(ctx),
	}
}
