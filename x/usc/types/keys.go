package types

import (
	"bytes"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName is the module name.
	ModuleName = "usc"

	// StoreKey defines the primary module store key.
	StoreKey = ModuleName

	// RouterKey is the msg router key for the module.
	RouterKey = ModuleName

	// QuerierRoute is the querier route for the module.
	QuerierRoute = ModuleName

	// ActivePoolName defines module name for storing collateral coins.
	ActivePoolName = "usc_active_pool"

	// RedeemingPoolName defines module name for storing collateral coins which are queued to be redeemed.
	RedeemingPoolName = "usc_redeeming_pool"
)

var (
	// RedeemingQueueKey is a storage prefix for the redeeming queue keys.
	RedeemingQueueKey = []byte{0x10}
	// RedeemEntryKey is a storage prefix for storing RedeemEntry objects.
	RedeemEntryKey = []byte{0x11}
)

// GetRedeemingQueueKey creates a storage key for the redeeming queue RedeemingQueueData object.
// The redeeming queue is an array of timeSlices.
func GetRedeemingQueueKey(timestamp time.Time) []byte {
	bz := sdk.FormatTimeBytes(timestamp)

	return append(RedeemingQueueKey, bz...)
}

// ParseRedeemingQueueKey parses the redeeming queue timeSlice storage key.
func ParseRedeemingQueueKey(key []byte) time.Time {
	if len(key) == 0 {
		panic(fmt.Errorf("parsing timeSlice key: empty key"))
	}

	prefix, bz := key[:1], key[1:]
	if !bytes.Equal(prefix, RedeemingQueueKey) {
		panic(fmt.Errorf("parsing timeSlice key (%v): unexpected prefix", key))
	}

	timestamp, err := sdk.ParseTimeBytes(bz)
	if err != nil {
		panic(fmt.Errorf("parsing timeSlice key (%v): %w", key, err))
	}

	return timestamp
}

// GetRedeemEntryKey creates a storage key for RedeemEntry object.
func GetRedeemEntryKey(accAddr sdk.AccAddress) []byte {
	accAddrBz := accAddr.Bytes()

	return append(RedeemEntryKey, accAddrBz...)
}
