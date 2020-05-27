package types

const (
	// ModuleName is the name of the contract module
	ModuleName = "register"

	// StoreKey is the string store representation
	StoreKey = ModuleName

	// TStoreKey is the string transient store representation
	TStoreKey = "transient_" + ModuleName

	// QuerierRoute is the querier route for the staking module
	QuerierRoute = ModuleName

	// RouterKey is the msg router key for the staking module
	RouterKey = ModuleName
)

// nolint
var (
	RegistrationStorePrefix = []byte{0x01}
	MasterKeyPrefix         = []byte{0x02}
)

func RegistrationKeyPrefix(key []byte) []byte {
	return append(RegistrationStorePrefix, key...)
}

func MasterCertPrefix(key string) []byte {
	return append(MasterKeyPrefix, []byte(key)...)
}
