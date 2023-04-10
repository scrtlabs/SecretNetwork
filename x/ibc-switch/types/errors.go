package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	ErrIbcOff = sdkerrors.Register(ModuleName, 1, "Ibc is temporarily paused")
)
