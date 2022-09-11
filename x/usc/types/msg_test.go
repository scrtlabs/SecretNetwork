package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
)

func TestMsgMintUSCValidateBasic(t *testing.T) {
	type testCase struct {
		name             string
		address          string
		collateralAmount []sdk.Coin
		//
		errExpected bool
	}

	testCases := []testCase{
		{
			name:             "OK",
			address:          ValidAddr,
			collateralAmount: sdk.NewCoins(sdk.NewCoin(ValidDenom, sdk.OneInt())),
		},
		{
			name:             "Fail: coins with zero amt",
			address:          ValidAddr,
			collateralAmount: sdk.NewCoins(sdk.NewCoin(ValidDenom, sdk.ZeroInt())),
			errExpected:      true,
		},
		{
			name:             "Fail: empty coins 1",
			address:          ValidAddr,
			collateralAmount: sdk.NewCoins(),
			errExpected:      true,
		},
		{
			name:             "Fail: empty coins 2",
			address:          ValidAddr,
			collateralAmount: nil,
			errExpected:      true,
		},
		{
			name:             "Fail: invalid address",
			address:          InvalidAddr,
			collateralAmount: sdk.NewCoins(sdk.NewCoin(ValidDenom, sdk.OneInt())),
			errExpected:      true,
		},
		{
			name:             "Fail: negative amount",
			address:          InvalidAddr,
			collateralAmount: []sdk.Coin{{Denom: ValidDenom, Amount: sdk.NewInt(-1)}},
			errExpected:      true,
		},
		{
			name:             "Fail: invalid Denom",
			address:          InvalidAddr,
			collateralAmount: []sdk.Coin{{Denom: InvalidDenom, Amount: sdk.OneInt()}},
			errExpected:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := MsgMintUSC{
				Address:          tc.address,
				CollateralAmount: tc.collateralAmount,
			}

			if tc.errExpected {
				assert.Error(t, msg.ValidateBasic())
				return
			}
			assert.NoError(t, msg.ValidateBasic())
		})
	}
}

func TestMsgRedeemCollateralValidateBasic(t *testing.T) {
	type testCase struct {
		name      string
		address   string
		usdAmount sdk.Coin
		//
		errExpected bool
	}

	testCases := []testCase{
		{
			name:      "OK",
			address:   ValidAddr,
			usdAmount: sdk.NewCoin(ValidDenom, sdk.OneInt()),
		},
		{
			name:        "Fail: coin with zero amt",
			address:     ValidAddr,
			usdAmount:   sdk.NewCoin(ValidDenom, sdk.ZeroInt()),
			errExpected: true,
		},
		{
			name:        "Fail: invalid address",
			address:     InvalidAddr,
			usdAmount:   sdk.NewCoin(ValidDenom, sdk.OneInt()),
			errExpected: true,
		},
		{
			name:        "Fail: negative amount",
			address:     ValidAddr,
			usdAmount:   sdk.Coin{Denom: ValidDenom, Amount: sdk.NewInt(-1)},
			errExpected: true,
		},
		{
			name:        "Fail: invalid Denom",
			address:     ValidAddr,
			usdAmount:   sdk.Coin{Denom: InvalidDenom, Amount: sdk.OneInt()},
			errExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := MsgRedeemCollateral{
				Address:   tc.address,
				UscAmount: tc.usdAmount,
			}

			if tc.errExpected {
				assert.Error(t, msg.ValidateBasic())
				return
			}
			assert.NoError(t, msg.ValidateBasic())
		})
	}
}
