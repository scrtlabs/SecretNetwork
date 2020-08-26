package keeper

import (
	"fmt"

	sdk "github.com/enigmampc/cosmos-sdk/types"
	cosmwasm "github.com/enigmampc/SecretNetwork/go-cosmwasm"
)

var (
	CostHumanize  = 5 * GasMultiplier
	CostCanonical = 4 * GasMultiplier
)

func humanAddress(canon []byte) (string, uint64, error) {
	if len(canon) != sdk.AddrLen {
		return "", CostHumanize, fmt.Errorf("Expected %d byte address", sdk.AddrLen)
	}
	return sdk.AccAddress(canon).String(), CostHumanize, nil
}

func canonicalAddress(human string) ([]byte, uint64, error) {
	bz, err := sdk.AccAddressFromBech32(human)
	return bz, CostCanonical, err
}

var cosmwasmAPI = cosmwasm.GoAPI{
	HumanAddress:     humanAddress,
	CanonicalAddress: canonicalAddress,
}
