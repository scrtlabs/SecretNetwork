package types

import (
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	MockTimestamp = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

	ValidDenom   = "usdc"
	InvalidDenom = "#Invalid"

	ValidAddr   = sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address()).String()
	InvalidAddr = "InvalidAddress"
)
