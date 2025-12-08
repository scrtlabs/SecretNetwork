package keeper

import (
	"context"

	"github.com/scrtlabs/SecretNetwork/x/tss/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	// Set params
	if err := k.Params.Set(ctx, genState.Params); err != nil {
		return err
	}

	// Import all KeySets
	for _, keySet := range genState.KeySets {
		if err := k.SetKeySet(ctx, *keySet); err != nil {
			return err
		}
	}

	return nil
}

// ExportGenesis returns the module's exported genesis.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	var err error

	genesis := types.DefaultGenesis()
	genesis.Params, err = k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	// Export all KeySets
	keySets, err := k.GetAllKeySets(ctx)
	if err != nil {
		return nil, err
	}
	for i := range keySets {
		genesis.KeySets = append(genesis.KeySets, &keySets[i])
	}

	return genesis, nil
}
