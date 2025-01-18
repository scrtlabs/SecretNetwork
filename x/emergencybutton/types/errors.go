package types

import (
	"cosmossdk.io/errors"
)

var (
	ErrIbcOff             = errors.Register(ModuleName, 1, "ibc processing failed")
	ErrUnauthorizedToggle = errors.Register(ModuleName, 2, "emergency button toggle failed")
	ErrPauserUnset        = errors.Register(ModuleName, 3, "emergency button toggle failed")
)
