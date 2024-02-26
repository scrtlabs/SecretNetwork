package types

import (
	"cosmossdk.io/errors"
)

// Codes for wasm contract errors
var (
	DefaultCodespace = ModuleName

	// ErrSeedInitFailed error when the seed initialization fails
	ErrSeedInitFailed = errors.Register(DefaultCodespace, 1, "Initialize node seed failed")

	ErrAuthenticateFailed = errors.Register(DefaultCodespace, 2, "Failed to authenticate node")

	// ErrSeedInitFailed error when the seed initialization fails
	ErrSeedValidationParams = errors.Register(DefaultCodespace, 3, "Failed to validate seed parameters")

	// ErrSeedInitFailed error when the seed initialization fails
	BootstrapInitFailed = errors.Register(DefaultCodespace, 4, "Failed to initialize bootstrap")

	ErrInvalidType = errors.Register(DefaultCodespace, 5, "Type error")

	ErrCertificateInvalid = errors.Register(DefaultCodespace, 6, "Certificate invalid or does not exist")

	ErrNotFound = errors.Register(DefaultCodespace, 7, "not found")

	ErrInvalid = errors.Register(DefaultCodespace, 8, "invalid")
)
