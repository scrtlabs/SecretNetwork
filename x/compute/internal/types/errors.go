package types

import (
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Codes for wasm contract errors
var (
	DefaultCodespace = ModuleName

	// ErrCreateFailed error for wasm code that has already been uploaded or failed
	ErrCreateFailed = sdkErrors.Register(DefaultCodespace, 1, "create wasm contract failed")

	// ErrAccountExists error for a contract account that already exists
	ErrAccountExists = sdkErrors.Register(DefaultCodespace, 2, "contract account already exists")

	// ErrInstantiateFailed error for rust instantiate contract failure
	ErrInstantiateFailed = sdkErrors.Register(DefaultCodespace, 3, "instantiate wasm contract failed")

	// ErrExecuteFailed error for rust execution contract failure
	ErrExecuteFailed = sdkErrors.Register(DefaultCodespace, 4, "execute wasm contract failed")

	// ErrGasLimit error for out of gas
	ErrGasLimit = sdkErrors.Register(DefaultCodespace, 5, "insufficient gas")

	// ErrInvalidGenesis error for invalid genesis file syntax
	ErrInvalidGenesis = sdkErrors.Register(DefaultCodespace, 6, "invalid genesis")

	// ErrNotFound error for an entry not found in the store
	ErrNotFound = sdkErrors.Register(DefaultCodespace, 7, "not found")

	// ErrQueryFailed error for rust smart query contract failure
	ErrQueryFailed = sdkErrors.Register(DefaultCodespace, 8, "query wasm contract failed")

	// ErrInvalidMsg error when we cannot process the error returned from the contract
	ErrInvalidMsg = sdkErrors.Register(DefaultCodespace, 9, "invalid CosmosMsg from the contract")

	// ErrMigrationFailed error for rust execution contract failure
	ErrMigrationFailed = sdkErrors.Register(DefaultCodespace, 10, "migrate wasm contract failed")
)
