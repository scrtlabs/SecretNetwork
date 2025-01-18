package util

import sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

const (
	// Bech32PrefixAccAddr defines the Bech32 prefix of an account's address
	Bech32PrefixAccAddr = "secret"
	// Bech32PrefixAccPub defines the Bech32 prefix of an account's public key
	Bech32PrefixAccPub = "secretpub"
	// Bech32PrefixValAddr defines the Bech32 prefix of a validator's operator address
	Bech32PrefixValAddr = "secretvaloper"
	// Bech32PrefixValPub defines the Bech32 prefix of a validator's operator public key
	Bech32PrefixValPub = "secretvaloperpub"
	// Bech32PrefixConsAddr defines the Bech32 prefix of a consensus node address
	Bech32PrefixConsAddr = "secretvalcons"
	// Bech32PrefixConsPub defines the Bech32 prefix of a consensus node public key
	Bech32PrefixConsPub = "secretvalconspub"
	CoinType            = 529
	CoinPurpose         = 44
)

// AddressVerifier secret address verifier
var AddressVerifier = func(bytes []byte) error {
	// 20 bytes = module accounts, base accounts, secret contracts
	// 32 bytes = ICA accounts
	if len(bytes) == 45 {
		return nil
	}

	if len(bytes) != 20 && len(bytes) != 32 {
		return sdkerrors.ErrUnknownAddress.Wrapf("address length must be 20 or 32 bytes, got %d", len(bytes))
	}

	return nil
}
