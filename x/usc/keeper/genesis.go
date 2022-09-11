package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
)

// InitGenesis performs module's genesis initialization.
func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState) {
	k.SetParams(ctx, genState.Params)

	for _, entry := range genState.RedeemEntries {
		accAddr, err := sdk.AccAddressFromBech32(entry.Address)
		if err != nil {
			panic(fmt.Errorf("parsing RedeemEntry.Address (%s): %w", entry.Address, err))
		}

		k.SetRedeemEntry(ctx, entry)
		for _, op := range entry.Operations {
			k.InsertToRedeemQueue(ctx, op.CompletionTime, accAddr)
		}
	}
}

// ExportGenesis returns the current module genesis state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params := k.GetParams(ctx)

	var entries []types.RedeemEntry
	k.IterateRedeemEntries(ctx, func(entry types.RedeemEntry) (stop bool) {
		entries = append(entries, entry)
		return false
	})

	return &types.GenesisState{
		Params:        params,
		RedeemEntries: entries,
	}
}
