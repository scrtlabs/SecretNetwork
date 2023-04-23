package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	ErrIbcOff             = sdkerrors.Register(ModuleName, 1, "ibc processing failed")
	ErrUnauthorizedToggle = sdkerrors.Register(ModuleName, 2, "ibc-switch toggle failed")
	ErrPauserUnset        = sdkerrors.Register(ModuleName, 3, "ibc-switch toggle failed")
)
