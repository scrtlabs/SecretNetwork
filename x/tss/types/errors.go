package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/tss module sentinel errors
var (
	// General errors
	ErrInvalidSigner = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")

	// DKG/KeySet errors (from x/mpc)
	ErrInvalidThreshold = errors.Register(ModuleName, 1101, "invalid threshold or max_signers parameters")

	// Signing errors (from x/signing)
	ErrUnauthorizedKeySet = errors.Register(ModuleName, 1200, "requester is not the owner of the specified KeySet")
	ErrKeySetNotFound     = errors.Register(ModuleName, 1201, "KeySet not found")
)
