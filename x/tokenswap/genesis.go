package tokenswap

import (
	sdk "github.com/Cashmaney/cosmos-sdk/types"
	"github.com/Cashmaney/cosmos-sdk/x/supply"
	swtypes "github.com/enigmampc/EnigmaBlockchain/x/tokenswap/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

func InitGenesis(ctx sdk.Context, supplyKeeper SupplyKeeper, keeper SwapKeeper, data GenesisState) []abci.ValidatorUpdate {
	tokenSwapAccount := supply.NewEmptyModuleAccount(ModuleName, supply.Burner, supply.Minter)
	supplyKeeper.SetModuleAccount(ctx, tokenSwapAccount)
	keeper.SetParams(ctx, data.Params)

	for _, swap := range data.Swaps {
		keeper.SetSwap(ctx, swap)
	}

	return nil
}

// ExportGenesis returns a GenesisState for a given context and keeper.
func ExportGenesis(ctx sdk.Context, keeper SwapKeeper) GenesisState {
	params := keeper.GetParams(ctx)

	var swaps []swtypes.TokenSwapRecord

	iter := keeper.GetTokenSwapRecordsIterator(ctx)
	for ; iter.Valid(); iter.Next() {
		var swap swtypes.TokenSwapRecord
		keeper.Cdc().MustUnmarshalBinaryBare(iter.Value(), &swap)
		// cb returns true to stop early
		swaps = append(swaps, swap)
	}

	return NewGenesisState(params, swaps)
}
