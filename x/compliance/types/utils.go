package types

import (
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// ParseAddress tries to convert provided bech32 or hex address into sdk.AccAddress
func ParseAddress(input string) (sdk.AccAddress, error) {
	cfg := sdk.GetConfig()

	var err error
	if !strings.HasPrefix(input, cfg.GetBech32AccountAddrPrefix()) {
		// Assume that was provided eth address
		hexAddress := common.HexToAddress(input)
		return hexAddress.Bytes(), nil
	}

	// Assume that was provided bech32 address
	accAddress, err := sdk.AccAddressFromBech32(input)
	if err != nil {
		return nil, err
	}

	return accAddress, nil
}
