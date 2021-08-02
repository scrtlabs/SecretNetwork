package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	cosmwasm "github.com/enigmampc/SecretNetwork/go-cosmwasm"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

var (
	CostHumanize  = 5 * types.GasMultiplier
	CostCanonical = 4 * types.GasMultiplier
)

func humanAddress(canon []byte) (string, uint64, error) {
	err := sdk.VerifyAddressFormat(canon)
	if err != nil {
		return "", CostHumanize, nil
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
