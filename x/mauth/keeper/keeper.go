package keeper

import (
	"cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"

	icacontrollerkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/keeper"
)

type Keeper struct {
	cdc codec.Codec

	storeService store.KVStoreService

	scopedKeeper        capabilitykeeper.ScopedKeeper
	icaControllerKeeper icacontrollerkeeper.Keeper
}

func NewKeeper(cdc codec.Codec, storeService store.KVStoreService, iaKeeper icacontrollerkeeper.Keeper, scopedKeeper capabilitykeeper.ScopedKeeper) Keeper {
	return Keeper{
		cdc:      cdc,
		storeService: storeService,

		scopedKeeper:        scopedKeeper,
		icaControllerKeeper: iaKeeper,
	}
}

// ClaimCapability claims the channel capability passed via the OnOpenChanInit callback
func (k *Keeper) ClaimCapability(ctx sdk.Context, cap *capabilitytypes.Capability, name string) error {
	return k.scopedKeeper.ClaimCapability(ctx, cap, name)
}
