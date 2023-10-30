package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

func (i *Keeper) GetPauserAddress(ctx sdk.Context) (pauser string) {
	return i.GetParams(ctx).PauserAddress
}

//func (i *Keeper) GetSwitchStatus(ctx sdk.Context) (status string) {
//	return i.GetParams(ctx).SwitchStatus
//}

func (i *Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	// This was previously done via i.paramSpace.GetParamSet(ctx, &params). That will
	// panic if the params don't exist. This is a workaround to avoid that panic.
	// Params should be refactored to just use a raw kvstore.
	empty := types.Params{}
	for _, pair := range params.ParamSetPairs() {
		i.paramSpace.GetIfExists(ctx, pair.Key, pair.Value)
	}
	if params == empty {
		return types.DefaultParams()
	}
	return params
}

func (i *Keeper) GetSwitchStatus(ctx sdk.Context) (status string) {
	i.paramSpace.Get(ctx, types.KeySwitchStatus, &status)
	return
}

func (i *Keeper) SetSwitchStatus(ctx sdk.Context, value string) {
	i.paramSpace.Set(ctx, types.KeySwitchStatus, value)
}

func (i *Keeper) SetParams(ctx sdk.Context, params types.Params) {
	i.paramSpace.SetParamSet(ctx, &params)
}
