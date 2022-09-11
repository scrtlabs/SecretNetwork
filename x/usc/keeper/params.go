package keeper

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
)

// RedeemDur returns USC -> collateral coins redeem timeout duration.
func (k Keeper) RedeemDur(ctx sdk.Context) (res time.Duration) {
	k.paramStore.Get(ctx, types.ParamsKeyRedeemDur, &res)
	return
}

// MaxRedeemEntries returns the max number of redeem entries per account.
func (k Keeper) MaxRedeemEntries(ctx sdk.Context) (res uint32) {
	k.paramStore.Get(ctx, types.ParamsKeyMaxRedeemEntries, &res)
	return
}

// CollateralMetas returns supported collateral token metas.
func (k Keeper) CollateralMetas(ctx sdk.Context) (res []types.TokenMeta) {
	k.paramStore.Get(ctx, types.ParamsKeyCollateralMetas, &res)
	return
}

// Enabled returns whether the module is enabled or not.
func (k Keeper) Enabled(ctx sdk.Context) (res bool) {
	k.paramStore.Get(ctx, types.ParamsKeyEnabled, &res)
	return
}

// BaseMeta returns meta with the minimum decimals amount (to normalize coins).
func (k Keeper) BaseMeta(ctx sdk.Context) types.TokenMeta {
	uscMeta := k.USCMeta(ctx)
	minMeta := uscMeta

	for _, meta := range k.CollateralMetas(ctx) {
		if meta.Decimals > minMeta.Decimals {
			minMeta = meta
		}
	}

	return minMeta
}

// CollateralMetasSet returns supported collateral token metas set (key: denom).
func (k Keeper) CollateralMetasSet(ctx sdk.Context) map[string]types.TokenMeta {
	metas := k.CollateralMetas(ctx)

	set := make(map[string]types.TokenMeta, len(metas))
	for _, meta := range metas {
		set[meta.Denom] = meta
	}

	return set
}

// USCMeta returns the USC token meta.
func (k Keeper) USCMeta(ctx sdk.Context) (res types.TokenMeta) {
	k.paramStore.Get(ctx, types.ParamsKeyUSCMeta, &res)
	return
}

// GetParams returns all module parameters.
func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	return types.NewParams(
		k.RedeemDur(ctx),
		k.MaxRedeemEntries(ctx),
		k.CollateralMetas(ctx),
		k.USCMeta(ctx),
		k.Enabled(ctx),
	)
}

// SetParams sets all module parameters.
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramStore.SetParamSet(ctx, &params)
}
