package utils

import (
	"fmt"
	"strings"
	"github.com/ethereum/go-ethereum/common"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/app"
)

type API struct{}

// NewAPI creates an instance of the utils API.
func NewAPI() *API {
	return &API{}
}

// ConvertAddress converts provided address from bech32 format to hex
// and vice versa
func (a *API) ConvertAddress(address string) (string, error) {
	switch {
	case common.IsHexAddress(address):
		addrBytes := common.HexToAddress(address).Bytes()
		convertedAddr := sdk.AccAddress(addrBytes)
		return convertedAddr.String(), nil
	case strings.HasPrefix(address, app.AccountAddressPrefix):
		addrBytes, _ := sdk.AccAddressFromBech32(address)
		convertedAddr := common.BytesToAddress(addrBytes)
		return convertedAddr.String(), nil
	default:
		return "", fmt.Errorf("expected a valid hex or bech32 address")
	}
}