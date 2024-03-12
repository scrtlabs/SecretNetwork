package types

import (
	"cosmossdk.io/errors"
)

var (
	ErrIBCAccountAlreadyExist = errors.Register(ModuleName, 2, "interchain account already registered")
	ErrIBCAccountNotExist     = errors.Register(ModuleName, 3, "interchain account not exist")
)
