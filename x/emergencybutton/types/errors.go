package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	ErrIbcOff             = sdkerrors.Register(ModuleName, 1, "ibc processing failed")
	ErrUnauthorizedToggle = sdkerrors.Register(ModuleName, 2, "emergencybutton toggle failed")
	ErrPauserUnset        = sdkerrors.Register(ModuleName, 3, "emergencybutton toggle failed")
)
