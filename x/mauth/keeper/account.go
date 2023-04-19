package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterInterchainAccount invokes the InitInterchainAccount entrypoint.
// InitInterchainAccount binds a new controller port and initiates a new ICS-27 channel handshake
func (k Keeper) RegisterInterchainAccount(ctx sdk.Context, owner sdk.AccAddress, connectionID string, version string) error {
	return k.icaControllerKeeper.RegisterInterchainAccount(ctx, connectionID, owner.String(), version)
}
