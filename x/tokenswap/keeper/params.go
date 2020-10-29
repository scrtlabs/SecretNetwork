package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/tokenswap/types"
)

// GetParams returns the total set of distribution parameters.
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return params
}

// SetParams sets the distribution parameters to the param space.
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramSpace.SetParamSet(ctx, &params)
}

// GetCommunityTax returns the current distribution community tax.
func (k Keeper) GetMultisigApproveAddress(ctx sdk.Context) (addr sdk.AccAddress) {
	k.paramSpace.Get(ctx, types.ParamStoreKeyMultisigApproveAddress, &addr)
	return addr
}

// GetBaseProposerReward returns the current distribution base proposer rate.
func (k Keeper) GetMintingMultiplier(ctx sdk.Context) (percent sdk.Dec) {
	k.paramSpace.Get(ctx, types.ParamStoreKeyMintingMultiplier, &percent)
	return percent
}

// GetBonusProposerReward returns the current distribution bonus proposer reward
// rate.
func (k Keeper) GetMintingEnabled(ctx sdk.Context) (enabled bool) {
	k.paramSpace.Get(ctx, types.ParamStoreKeyMintingEnabled, &enabled)
	return enabled
}
