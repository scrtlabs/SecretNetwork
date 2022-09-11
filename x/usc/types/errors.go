package types

import sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"

// Module specific errors.
var (
	ErrInternal              = sdkErrors.Register(ModuleName, 1, "internal")
	ErrUnsupportedCollateral = sdkErrors.Register(ModuleName, 2, "unsupported collateral denom")
	ErrInvalidUSC            = sdkErrors.Register(ModuleName, 3, "invalid USC denom")
	ErrMaxRedeemEntries      = sdkErrors.Register(ModuleName, 4, "max number of redeem entries reached")
)
