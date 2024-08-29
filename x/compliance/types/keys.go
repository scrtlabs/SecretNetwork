package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/kv"
)

const (
	// ModuleName defines the module name
	ModuleName = "compliance"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_compliance"
)

const (
	prefixIssuerDetails = iota + 1
	prefixAddressDetails
	prefixVerificationDetails
	prefixOperatorDetails
)

var (
	KeyPrefixOperatorDetails     = []byte{prefixOperatorDetails}
	KeyPrefixIssuerDetails       = []byte{prefixIssuerDetails}
	KeyPrefixAddressDetails      = []byte{prefixAddressDetails}
	KeyPrefixVerificationDetails = []byte{prefixVerificationDetails}
)

func AccAddressFromKey(key []byte) sdk.AccAddress {
	kv.AssertKeyAtLeastLength(key, 1)
	return key[1:]
}

func VerificationIdFromKey(key []byte) []byte {
	kv.AssertKeyAtLeastLength(key, 1)
	return key[1:]
}
