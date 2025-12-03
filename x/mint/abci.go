package mint

import (
	"context"
	"time"

	"cosmossdk.io/math"
	"github.com/scrtlabs/SecretNetwork/x/mint/keeper"
	"github.com/scrtlabs/SecretNetwork/x/mint/types"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BeginBlocker mints new tokens for the previous block.
func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)

	// Fetch stored minter
	minter, err := k.GetMinter(ctx)
	if err != nil {
		return err
	}

	// Fetch params
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	// Calculate annual provisions for informational purposes
	// AnnualProvisions = FixedBlockReward * BlocksPerYear
	annualProvisions := math.LegacyNewDecFromInt(params.FixedBlockReward).
		Mul(math.LegacyNewDec(int64(params.BlocksPerYear)))
	minter.AnnualProvisions = annualProvisions

	// Save updated minter
	if err := k.SetMinter(ctx, minter); err != nil {
		return err
	}

	// Mint coins for this block - FIXED amount regardless of supply
	mintedCoin := minter.BlockProvision(params)
	mintedCoins := sdk.NewCoins(mintedCoin)

	// Mint the coins
	if err := k.MintCoins(ctx, mintedCoins); err != nil {
		return err
	}

	// Send minted coins to the fee collector account
	if err := k.AddCollectedFees(ctx, mintedCoins); err != nil {
		return err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if mintedCoin.Amount.IsInt64() {
		defer telemetry.ModuleSetGauge(types.ModuleName, float32(mintedCoin.Amount.Int64()), "minted_tokens")
	}

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.ModuleName,
			sdk.NewAttribute(types.KeyMintDenom, params.MintDenom),
			sdk.NewAttribute(types.KeyFixedBlockReward, params.FixedBlockReward.String()),
			sdk.NewAttribute("amount", mintedCoin.Amount.String()),
		),
	)

	return nil
}
