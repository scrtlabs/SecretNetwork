package v1_4

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
)

func revertTombstone(ctx sdk.Context, valAddress sdk.ValAddress, consAddress sdk.ConsAddress, slashingKeeper *slashingkeeper.Keeper) error {
	slashingKeeper.RevertTombstone(ctx, consAddress)
	err := slashingKeeper.Unjail(ctx, valAddress, true)
	if err != nil {
		return err
	}

	return nil
}

func mintLostTokens() error {
	// assaf this is all you
	return nil
}

func RevertCosTombstoning(ctx sdk.Context, valAddress sdk.ValAddress, consAddress sdk.ConsAddress, slashingKeeper *slashingkeeper.Keeper) error {
	err := revertTombstone(ctx, valAddress, consAddress, slashingKeeper)
	if err != nil {
		return err
	}

	err = mintLostTokens()
	if err != nil {
		return err
	}

	return nil
}
