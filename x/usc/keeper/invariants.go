package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
)

// RegisterInvariants registers all module's invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "redeeming-queue",
		RedeemingQueueInvariant(k))
	ir.RegisterRoute(types.ModuleName, "usc-supply",
		USCSupplyInvariant(k))
}

// AllInvariants runs all invariants of the module (used by tests).
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		res, stop := RedeemingQueueInvariant(k)(ctx)
		if stop {
			return res, stop
		}

		return USCSupplyInvariant(k)(ctx)
	}
}

// RedeemingQueueInvariant checks that the Redeeming pool balance equals to the sum of all queue entries.
// That ensures that the queue state is correct.
func RedeemingQueueInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		redeemPoolExpected := k.RedeemingPool(ctx)

		redeemPoolCalculated := sdk.NewCoins()
		k.IterateRedeemEntries(ctx, func(entry types.RedeemEntry) (stop bool) {
			for _, op := range entry.Operations {
				redeemPoolCalculated = redeemPoolCalculated.Add(op.CollateralAmount...)
			}

			return false
		})

		broken := !redeemPoolExpected.IsEqual(redeemPoolCalculated)
		msg := fmt.Sprintf(
			"\tRedeeming pool tokens: %s\n"+
				"\tSum of redeeming queue entry tokens: %s\n",
			redeemPoolExpected, redeemPoolCalculated,
		)

		return sdk.FormatInvariant(types.ModuleName, "Redeeming pool balance and redeeming queue", msg), broken
	}
}

// USCSupplyInvariant checks that x/bank USC supply equals to the sum of Active and Redeeming pools balance (collaterals converted to USC).
// That ensures that all minted / burned operations didn't lost any of USC tokens.
func USCSupplyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		uscSupplyExpected := k.bankKeeper.GetSupply(ctx, k.USCMeta(ctx).Denom)

		colPoolCurrent := k.ActivePool(ctx)
		uscPoolCalculated, _, err := k.ConvertCollateralsToUSC(ctx, colPoolCurrent)
		if err != nil {
			panic(err)
		}

		broken := !uscSupplyExpected.IsEqual(uscPoolCalculated)
		msg := fmt.Sprintf(
			"\tUSC supply tokens: %s\n"+
				"\tActive pool collateral tokens: %s\n"+
				"\tActive pool USC converted tokens: %s\n",
			uscSupplyExpected, colPoolCurrent, uscPoolCalculated,
		)

		return sdk.FormatInvariant(types.ModuleName, "USC supply and Active pool balance", msg), broken
	}
}
