package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUSCKeeperMsgMintUSC(t *testing.T) {
	type testCase struct {
		name           string
		colCoinsToSwap string
		//
		errExpected     error
		uscAmtExpected  string
		colUsedExpected string
	}

	testCases := []testCase{
		{
			name:            "OK: mint 3.0 USC tokens (no collateral leftovers)",
			colCoinsToSwap:  "1000000000nbusd,1000000uusdt,1000musdc", // 1.0 BUSD, 1.0 USDT, 1.0 USDC
			errExpected:     nil,
			uscAmtExpected:  "3000000",                                // 3.0 USC
			colUsedExpected: "1000000000nbusd,1000000uusdt,1000musdc", // all
		},
		{
			name:            "OK: mint 3.0 USC tokens (with collateral leftovers)",
			colCoinsToSwap:  "1000000100nbusd,1000000uusdt,1000musdc", // 1.000000100 BUSD, 1.0 USDT, 1.0 USDC
			errExpected:     nil,
			uscAmtExpected:  "3000000",                                // 3.0 USC
			colUsedExpected: "1000000000nbusd,1000000uusdt,1000musdc", // 1.0 BUSD, 1.0 USDT, 1.0 USDC
		},
		{
			name:           "Fail: collateral is too small",
			colCoinsToSwap: "100nbusd", // 1.000000100 BUSD
			errExpected:    sdkErrors.ErrInsufficientFunds,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fixtures
			te := NewTestEnv(t)

			accAddr, accCoins := te.AddAccount(t, tc.colCoinsToSwap)
			swapColCoins := accCoins

			// Send msg
			msg := types.NewMsgMintUSC(accAddr, swapColCoins)
			require.NoError(t, msg.ValidateBasic())

			res, err := te.msgServer.MintUSC(sdk.WrapSDKContext(te.ctx), msg)
			if tc.errExpected != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tc.errExpected)

				return
			}
			require.NoError(t, err)
			require.NotNil(t, res)

			// Verify minted USC
			uscAmtExpected, ok := sdk.NewIntFromString(tc.uscAmtExpected)
			require.True(t, ok)
			uscMintedCoinExpected := sdk.NewCoin(types.DefaultUSCDenom, uscAmtExpected)

			assert.Equal(t,
				uscMintedCoinExpected.String(),
				res.MintedAmount.String(),
			)

			// Verify collaterals used
			colUsedExpected, err := sdk.ParseCoinsNormalized(tc.colUsedExpected)
			require.NoError(t, err)

			assert.Equal(t,
				colUsedExpected.String(),
				sdk.NewCoins(res.CollateralsAmount...).String(),
			)

			// Verify account balance
			assert.Equal(t, uscMintedCoinExpected.String(),
				te.app.BankKeeper.GetBalance(te.ctx, accAddr, types.DefaultUSCDenom).String(),
			)

			// Verify Active pool balance
			assert.Equal(t,
				colUsedExpected.String(),
				te.app.USCKeeper.ActivePool(te.ctx).String(),
			)
		})
	}
}

func TestUSCKeeperMsgRedeemCollateral(t *testing.T) {
	type testCase struct {
		name            string
		uscAmtToRedeem  string
		activePoolCoins string
		//
		errExpected              error
		uscAmtLeftExpected       string
		colCoinsRedeemedExpected string
	}

	testCases := []testCase{
		{
			name:                     "OK: partially filled",
			uscAmtToRedeem:           "10020",          // 0.010020 USC
			activePoolCoins:          "5musdc,10uusdt", // 0.005 USDC, 0.000010 USDT
			uscAmtLeftExpected:       "5010",           // 0.005010 USC
			colCoinsRedeemedExpected: "5musdc,10uusdt",
		},
		{
			name:                     "OK: fully filled",
			uscAmtToRedeem:           "130000000",                                 // 130.0 USC
			activePoolCoins:          "75000000000nbusd,50000000uusdt,25000musdc", // 75.0 BUSD, 50.0 USDT, 25.0 USDC
			uscAmtLeftExpected:       "0",                                         // none
			colCoinsRedeemedExpected: "75000000000nbusd,50000000uusdt,5000musdc",  // 75.0 BUSD, 50.0 USDT, 5.0 USDC
		},
		{
			name:            "Fail: USC amount is too small",
			uscAmtToRedeem:  "1",        // 0.000001 USC
			activePoolCoins: "999nbusd", // 0.000000999 BUSD
			errExpected:     sdkErrors.ErrInsufficientFunds,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fixtures
			te := NewTestEnv(t)

			accAddr, accCoins := te.AddAccount(t, tc.uscAmtToRedeem+types.DefaultUSCDenom)
			uscRedeemCoin := accCoins[0]

			activePoolCoins := te.AddActivePoolBalance(t, tc.activePoolCoins)

			// Send msg
			msg := types.NewMsgRedeemCollateral(accAddr, uscRedeemCoin)
			require.NoError(t, msg.ValidateBasic())

			res, err := te.msgServer.RedeemCollateral(sdk.WrapSDKContext(te.ctx), msg)
			if tc.errExpected != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.errExpected)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, res)

			// Build expect value
			uscLeftAmtExpected, ok := sdk.NewIntFromString(tc.uscAmtLeftExpected)
			require.True(t, ok)
			uscLeftCoinExpected := sdk.NewCoin(types.DefaultUSCDenom, uscLeftAmtExpected)

			uscBurnedExpected := uscRedeemCoin.Sub(uscLeftCoinExpected)
			completionTimeExpected := MockTimestamp.Add(te.app.USCKeeper.RedeemDur(te.ctx))

			colRedeemedCoinsExpected, err := sdk.ParseCoinsNormalized(tc.colCoinsRedeemedExpected)
			require.NoError(t, err)

			// Verify the result
			assert.Equal(t,
				uscBurnedExpected.String(),
				res.BurnedAmount.String(),
			)
			assert.Equal(t,
				colRedeemedCoinsExpected.String(),
				sdk.NewCoins(res.RedeemedAmount...).String(),
			)
			assert.EqualValues(t,
				completionTimeExpected,
				res.CompletionTime,
			)

			// Verify account balance
			assert.Equal(t,
				uscLeftCoinExpected.String(),
				te.app.BankKeeper.GetBalance(te.ctx, accAddr, types.DefaultUSCDenom).String(),
			)

			// Verify Active pool balance
			assert.Equal(t,
				activePoolCoins.Sub(colRedeemedCoinsExpected).String(),
				te.app.USCKeeper.ActivePool(te.ctx).String(),
			)

			// Verify Redeeming pool balance
			assert.Equal(t,
				colRedeemedCoinsExpected.String(),
				te.app.USCKeeper.RedeemingPool(te.ctx).String(),
			)
		})
	}
}
