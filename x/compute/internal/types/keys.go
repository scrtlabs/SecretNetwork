package types

import (
	"encoding/binary"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName is the name of the contract module
	ModuleName = "compute"

	// StoreKey is the string store representation
	StoreKey = ModuleName

	// TStoreKey is the string transient store representation
	TStoreKey = "transient_" + ModuleName

	// QuerierRoute is the querier route for the staking module
	QuerierRoute = ModuleName

	// RouterKey is the msg router key for the staking module
	RouterKey = ModuleName
)

var (
	CodeKeyPrefix                                  = []byte{0x01}
	ContractKeyPrefix                              = []byte{0x02}
	ContractStorePrefix                            = []byte{0x03}
	SequenceKeyPrefix                              = []byte{0x04}
	ContractEnclaveIdPrefix                        = []byte{0x06}
	ContractLabelPrefix                            = []byte{0x07}
	TXCounterPrefix                                = []byte{0x08}
	ContractCodeHistoryElementPrefix               = []byte{0x09}
	ContractByCodeIDAndCreatedSecondaryIndexPrefix = []byte{0x0A}
	RandomPrefix                                   = []byte{0xFF}

	KeyLastCodeID     = append(SequenceKeyPrefix, []byte("lastCodeId")...)
	KeyLastInstanceID = append(SequenceKeyPrefix, []byte("lastContractId")...)
)

// GetCodeKey constructs the key for retreiving the ID for the WASM code
func GetCodeKey(codeID uint64) []byte {
	contractIDBz := sdk.Uint64ToBigEndian(codeID)
	return append(CodeKeyPrefix, contractIDBz...)
}

func decodeCodeKey(src []byte) uint64 {
	return binary.BigEndian.Uint64(src[len(CodeKeyPrefix):])
}

// GetContractAddressKey returns the key for the WASM contract instance
func GetContractAddressKey(addr sdk.AccAddress) []byte {
	return append(ContractKeyPrefix, addr...)
}

// GetRandomKey returns the key for the random seed for each block
func GetRandomKey(height int64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(height))
	return append(RandomPrefix, b...)
}

// GetContractAddressKey returns the key for the WASM contract instance
func GetContractEnclaveKey(addr sdk.AccAddress) []byte {
	return append(ContractEnclaveIdPrefix, addr...)
}

// GetContractStorePrefixKey returns the store prefix for the WASM contract instance
func GetContractStorePrefixKey(addr sdk.AccAddress) []byte {
	return append(ContractStorePrefix, addr...)
}

// GetContractStorePrefixKey returns the store prefix for the WASM contract instance
func GetContractLabelPrefix(addr string) []byte {
	return append(ContractLabelPrefix, []byte(addr)...)
}

// GetContractCodeHistoryElementPrefix returns the key prefix for a contract code history entry: `<prefix><contractAddr>`
func GetContractCodeHistoryElementPrefix(contractAddr sdk.AccAddress) []byte {
	prefixLen := len(ContractCodeHistoryElementPrefix)
	contractAddrLen := len(contractAddr)
	r := make([]byte, prefixLen+contractAddrLen)
	copy(r[0:], ContractCodeHistoryElementPrefix)
	copy(r[prefixLen:], contractAddr)
	return r
}

// GetContractByCreatedSecondaryIndexKey returns the key for the secondary index:
// `<prefix><codeID><created/last-migrated><contractAddr>`
func GetContractByCreatedSecondaryIndexKey(contractAddr sdk.AccAddress, c ContractCodeHistoryEntry) []byte {
	prefix := GetContractByCodeIDSecondaryIndexPrefix(c.CodeID)
	prefixLen := len(prefix)
	contractAddrLen := len(contractAddr)
	r := make([]byte, prefixLen+AbsoluteTxPositionLen+contractAddrLen)
	copy(r[0:], prefix)
	copy(r[prefixLen:], c.Updated.Bytes())
	copy(r[prefixLen+AbsoluteTxPositionLen:], contractAddr)
	return r
}

// GetContractByCodeIDSecondaryIndexPrefix returns the prefix for the second index: `<prefix><codeID>`
func GetContractByCodeIDSecondaryIndexPrefix(codeID uint64) []byte {
	prefixLen := len(ContractByCodeIDAndCreatedSecondaryIndexPrefix)
	const codeIDLen = 8
	r := make([]byte, prefixLen+codeIDLen)
	copy(r[0:], ContractByCodeIDAndCreatedSecondaryIndexPrefix)
	copy(r[prefixLen:], sdk.Uint64ToBigEndian(codeID))
	return r
}

// GetContractCodeHistoryElementKey returns the key a contract code history entry: `<prefix><contractAddr><position>`
func GetContractCodeHistoryElementKey(contractAddr sdk.AccAddress, pos uint64) []byte {
	prefix := GetContractCodeHistoryElementPrefix(contractAddr)
	prefixLen := len(prefix)
	r := make([]byte, prefixLen+8)
	copy(r[0:], prefix)
	copy(r[prefixLen:], sdk.Uint64ToBigEndian(pos))
	return r
}
