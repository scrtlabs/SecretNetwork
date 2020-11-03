package types

import (
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Codes for wasm contract errors
var (
	DefaultCodespace = ModuleName

	// ErrSeedInitFailed error when the seed initialization fails
	ErrSeedInitFailed = sdkErrors.Register(DefaultCodespace, 1, "Initialize node seed failed")

	ErrAuthenticateFailed = sdkErrors.Register(DefaultCodespace, 2, "Failed to authenticate node")

	// ErrSeedInitFailed error when the seed initialization fails
	ErrSeedValidationParams = sdkErrors.Register(DefaultCodespace, 3, "Failed to validate seed parameters")

	// ErrSeedInitFailed error when the seed initialization fails
	BootstrapInitFailed = sdkErrors.Register(DefaultCodespace, 4, "Failed to initialize bootstrap")

	ErrInvalidType = sdkErrors.Register(DefaultCodespace, 5, "Type error")

	ErrCertificateInvalid = sdkErrors.Register(DefaultCodespace, 6, "Certificate invalid or does not exist")

	ErrNotFound = sdkErrors.Register(DefaultCodespace, 7, "not found")

	ErrInvalid = sdkErrors.Register(DefaultCodespace, 8, "invalid")
)
