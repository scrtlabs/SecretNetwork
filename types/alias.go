package types

import (
	"github.com/enigmampc/SecretNetwork/types/util"
)

// Aliases for internal types
const (
	Bech32PrefixAccAddr  = util.Bech32PrefixAccAddr
	Bech32PrefixAccPub   = util.Bech32PrefixAccPub
	Bech32PrefixValAddr  = util.Bech32PrefixValAddr
	Bech32PrefixValPub   = util.Bech32PrefixValPub
	Bech32PrefixConsAddr = util.Bech32PrefixConsAddr
	Bech32PrefixConsPub  = util.Bech32PrefixConsPub
	CoinType             = util.CoinType
	CoinPurpose          = util.CoinPurpose
)

// functions aliases
var (
	AddressVerifier = util.AddressVerifier
)
