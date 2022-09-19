package types

import (
	"strings"

	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Codes for wasm contract errors
// 1-5 are errors that contain an encrypted payload. If you add more to the list, add it at the end so we don't rename
// the error codes every other day (though nothing outside this file actually depends on them) and update the
// IsEncryptedErrorCode function.
var (
	DefaultCodespace = ModuleName

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

	// ErrEmpty error for empty content
	ErrEmpty = sdkErrors.Register(DefaultCodespace, 11, "empty")

	// ErrLimit error for content that exceeds a limit
	ErrLimit = sdkErrors.Register(DefaultCodespace, 12, "exceeds limit")

	// ErrInvalid error for content that is invalid in this context
	ErrInvalid = sdkErrors.Register(DefaultCodespace, 13, "invalid")

	// ErrDuplicate error for content that exsists
	ErrDuplicate = sdkErrors.Register(DefaultCodespace, 14, "duplicate")

	// ErrCreateFailed error for wasm code that has already been uploaded or failed
	ErrCreateFailed = sdkErrors.Register(DefaultCodespace, 15, "create contract failed")

	// ErrSigFailed error for wasm code that has already been uploaded or failed
	ErrSigFailed = sdkErrors.Register(DefaultCodespace, 16, "parse signature failed")

	// ErrUnsupportedForContract error when a feature is used that is not supported for/ by this contract
	ErrUnsupportedForContract = sdkErrors.Register(DefaultCodespace, 17, "unsupported for this contract")

	// ErrUnknownMsg error by a message handler to show that it is not responsible for this message type
	ErrUnknownMsg = sdkErrors.Register(DefaultCodespace, 18, "unknown message from the contract")

	// ErrReplyFailed error for rust execution contract failure
	ErrReplyFailed = sdkErrors.Register(DefaultCodespace, 19, "reply to contract failed")

	// ErrInvalidEvent error if an attribute/event from the contract is invalid
	ErrInvalidEvent = sdkErrors.Register(DefaultCodespace, 21, "invalid event")

	// ErrMaxIBCChannels error for maximum number of ibc channels reached
	ErrMaxIBCChannels = sdkErrors.Register(DefaultCodespace, 22, "max transfer channels")
)

func IsEncryptedErrorCode(code uint32) bool {
	return 5 >= code && code > 1
}

func ErrContainsQueryError(err error) bool {
	return strings.Contains(err.Error(), ErrQueryFailed.Error())
}

// ** Warning **
// Below are functions that check for magic strings that depends on the output of the enclave.
// Beware when changing this, or the rust error string
func ContainsEnclaveError(str string) bool {
	return strings.Contains(str, "Enclave")
}

func ContainsEncryptedString(str string) bool {
	return strings.Contains(str, "encrypted: ")
}
