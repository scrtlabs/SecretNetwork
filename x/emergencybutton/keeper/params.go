package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

func (i *Keeper) GetPauserAddress(ctx sdk.Context) (pauser string) {
	return i.GetParams(ctx).PauserAddress
}

func (i *Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	store := ctx.KVStore(i.storeKey)
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return params
	}

	i.cdc.MustUnmarshal(bz, &params)
	return params
}

func (i *Keeper) GetSwitchStatus(ctx sdk.Context) (status string) {
	return i.GetParams(ctx).SwitchStatus
}

func (i *Keeper) SetSwitchStatus(ctx sdk.Context, value string) {
	params := i.GetParams(ctx)
	params.SwitchStatus = value
	i.SetParams(ctx, params)
}

// SetParams sets the x/emergencybutton module parameters.
func (i *Keeper) SetParams(ctx sdk.Context, p types.Params) error {
	if err := p.Validate(); err != nil {
		return err
	}

	store := ctx.KVStore(i.storeKey)
	bz := i.cdc.MustMarshal(&p)
	store.Set(types.ParamsKey, bz)

	return nil
}
