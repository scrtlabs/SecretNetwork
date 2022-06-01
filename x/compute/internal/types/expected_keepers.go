package types

import sdk "github.com/cosmos/cosmos-sdk/types"

// ICS20TransferPortSource is a subset of the ibc transfer keeper.
type ICS20TransferPortSource interface {
	GetPort(ctx sdk.Context) string
}
