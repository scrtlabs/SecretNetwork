package types

import (
	sdkErrors "github.com/enigmampc/cosmos-sdk/types/errors"
	"strings"
)

// Codes for wasm contract errors
var (
	DefaultCodespace = ModuleName

	// ErrCreateFailed error for wasm code that has already been uploaded or failed
	ErrCreateFailed = sdkErrors.Register(DefaultCodespace, 1, "create contract failed")

	// ErrInstantiateFailed error for rust instantiate contract failure
	ErrInstantiateFailed = sdkErrors.Register(DefaultCodespace, 2, "instantiate contract failed")

	// ErrExecuteFailed error for rust execution contract failure
	ErrExecuteFailed = sdkErrors.Register(DefaultCodespace, 3, "execute contract failed")

	// ErrQueryFailed error for rust smart query contract failure
	ErrQueryFailed = sdkErrors.Register(DefaultCodespace, 4, "query contract failed")

	// ErrMigrationFailed error for rust execution contract failure
	ErrMigrationFailed = sdkErrors.Register(DefaultCodespace, 5, "migrate contract failed")

	// ErrAccountExists error for a contract account that already exists
	ErrAccountExists = sdkErrors.Register(DefaultCodespace, 6, "contract account already exists")

	// ErrGasLimit error for out of gas
	ErrGasLimit = sdkErrors.Register(DefaultCodespace, 7, "insufficient gas")

	// ErrInvalidGenesis error for invalid genesis file syntax
	ErrInvalidGenesis = sdkErrors.Register(DefaultCodespace, 8, "invalid genesis")

	// ErrNotFound error for an entry not found in the store
	ErrNotFound = sdkErrors.Register(DefaultCodespace, 9, "not found")

	// ErrInvalidMsg error when we cannot process the error returned from the contract
	ErrInvalidMsg = sdkErrors.Register(DefaultCodespace, 10, "invalid CosmosMsg from the contract")
)

func IsEncryptedError(code uint32) bool {
	return 5 >= code && code > 0
}

func ErrContainsQueryError(err error) bool {
	return strings.Contains(err.Error(), ErrQueryFailed.Error())
}

// ** Warning **
// This is a magic string that depends on the output of the enclave. Beware when changing this, or the rust error string
func ContainsEnclaveError(str string) bool {
	return strings.Contains(str, "EnclaveErr")
}
