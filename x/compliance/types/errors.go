package types

import (
	"cosmossdk.io/errors"
)

const (
	codeErrBadRequest = uint32(iota) + 2 // NOTE: code 1 is reserved for internal errors
	codeErrInvalidSignature
	codeErrSignatureNotFound
	codeErrBasicValidation
	codeErrInvalidParam
	codeErrNotAuthorized
	codeErrInvalidOperator
	codeErrNotOperator
	codeErrNotOperatorOrIssuer
	codeErrInvalidIssuer
)

var (
	ErrBadRequest                 = errors.Register(ModuleName, codeErrBadRequest, "bad request")
	ErrInvalidSignature           = errors.Register(ModuleName, codeErrInvalidSignature, "invalid signature detected")
	ErrSignatureNotFound          = errors.Register(ModuleName, codeErrSignatureNotFound, "signature is required but not found")
	ErrBasicValidation            = errors.Register(ModuleName, codeErrBasicValidation, "basic validation failed")
	ErrInvalidParam               = errors.Register(ModuleName, codeErrInvalidParam, "invalid param provided")
	ErrNotAuthorized              = errors.Register(ModuleName, codeErrNotAuthorized, "not authorized")
	ErrInvalidOperator            = errors.Register(ModuleName, codeErrInvalidOperator, "invalid operator")
	ErrNotOperatorOrIssuerCreator = errors.Register(ModuleName, codeErrNotOperatorOrIssuer, "signer is not operator or issuer creator")
	ErrNotOperator                = errors.Register(ModuleName, codeErrNotOperator, "signer is not operator")
	ErrInvalidIssuer              = errors.Register(ModuleName, codeErrInvalidIssuer, "invalid issuer")
)
