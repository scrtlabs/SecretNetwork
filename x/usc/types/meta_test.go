package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenMeta_NewZeroCoin(t *testing.T) {
	m := TokenMeta{Denom: "usc", Decimals: 3}
	coinReceived := m.NewZeroCoin()

	assert.Equal(t, "usc", coinReceived.Denom)
	assert.True(t, coinReceived.Amount.IsZero())
}

func TestTokenMeta_DecUnit(t *testing.T) {
	m := TokenMeta{Denom: "usc", Decimals: 3}
	unitReceived := m.DecUnit()
	unitExpected, err := sdk.NewDecFromStr("0.001")
	require.NoError(t, err)

	assert.EqualValues(t, unitExpected, unitReceived)
}

func TestTokenMeta_ConvertCoin(t *testing.T) {
	type testCase struct {
		name          string
		srcMeta       TokenMeta
		dstMeta       TokenMeta
		coinToConvert sdk.Coin
		//
		errExpected  bool
		coinExpected sdk.Coin
	}

	denomSrc, denomDst := "src", "dst"

	testCases := []testCase{
		{
			name:          "OK: 0.001src -> 0.001dst",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert: sdk.NewCoin(denomSrc, sdk.NewInt(1)),
			coinExpected:  sdk.NewCoin(denomDst, sdk.NewInt(1)),
		},
		{
			name:          "OK: 0.001src -> 0.001000dst",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 6},
			coinToConvert: sdk.NewCoin(denomSrc, sdk.NewInt(1)),
			coinExpected:  sdk.NewCoin(denomDst, sdk.NewInt(1000)),
		},
		{
			name:          "OK: 0.001000src -> 0.001dst",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 6},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert: sdk.NewCoin(denomSrc, sdk.NewInt(1000)),
			coinExpected:  sdk.NewCoin(denomDst, sdk.NewInt(1)),
		},
		{
			name:          "OK: 0.000001src -> 0.000dst",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 6},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert: sdk.NewCoin(denomSrc, sdk.NewInt(1)),
			coinExpected:  sdk.NewCoin(denomDst, sdk.ZeroInt()),
		},
		{
			name:          "Fail: coin.Denom != srcMeta.Denom",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert: sdk.NewCoin(denomDst, sdk.NewInt(1)),
			errExpected:   true,
		},
		{
			name:          "Fail: invalid srcMeta.Denom",
			srcMeta:       TokenMeta{Denom: InvalidDenom, Decimals: 3},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert: sdk.Coin{Denom: InvalidDenom, Amount: sdk.NewInt(1)},
			errExpected:   true,
		},
		{
			name:          "Fail: invalid dstMeta.Denom",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:       TokenMeta{Denom: InvalidDenom, Decimals: 3},
			coinToConvert: sdk.Coin{Denom: denomSrc, Amount: sdk.NewInt(1)},
			errExpected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coinReceived, err := tc.srcMeta.ConvertCoin(tc.coinToConvert, tc.dstMeta)
			if tc.errExpected {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.EqualValues(t, tc.coinExpected, coinReceived)
		})
	}
}

func TestTokenMeta_NormalizeCoin(t *testing.T) {
	type testCase struct {
		name          string
		srcMeta       TokenMeta
		dstMeta       TokenMeta
		coinToConvert sdk.Coin
		//
		errExpected  bool
		coinExpected sdk.Coin
	}

	denomSrc, denomDst := "src", "dst"

	testCases := []testCase{
		{
			name:          "OK: 0.001src -> 0.001dst",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert: sdk.NewCoin(denomSrc, sdk.NewInt(1)),
			coinExpected:  sdk.NewCoin(denomSrc, sdk.NewInt(1)),
		},
		{
			name:          "OK: 0.001src -> 0.001000dst",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 6},
			coinToConvert: sdk.NewCoin(denomSrc, sdk.NewInt(1)),
			coinExpected:  sdk.NewCoin(denomSrc, sdk.NewInt(1000)),
		},
		{
			name:          "Fail: dstMeta.Decimals < srcMeta.Decimals",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 6},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert: sdk.NewCoin(denomSrc, sdk.NewInt(1)),
			errExpected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coinReceived, err := tc.srcMeta.NormalizeCoin(tc.coinToConvert, tc.dstMeta)
			if tc.errExpected {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.EqualValues(t, tc.coinExpected, coinReceived)
		})
	}
}

func TestTokenMeta_ConvertCoin2(t *testing.T) {
	type testCase struct {
		name          string
		srcMeta       TokenMeta
		dstMeta       TokenMeta
		coinToConvert sdk.Coin
		//
		errExpected           bool
		coinConvertedExpected sdk.Coin
		coinDrainedExpected   sdk.Coin
	}

	denomSrc, denomDst := "src", "dst"

	testCases := []testCase{
		{
			name:                  "OK: 0.001src -> 0.001dst",
			srcMeta:               TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:               TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert:         sdk.NewCoin(denomSrc, sdk.NewInt(1)),
			coinConvertedExpected: sdk.NewCoin(denomDst, sdk.NewInt(1)),
			coinDrainedExpected:   sdk.NewCoin(denomSrc, sdk.NewInt(1)),
		},
		{
			name:                  "OK: 0.001src -> 0.001000dst",
			srcMeta:               TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:               TokenMeta{Denom: denomDst, Decimals: 6},
			coinToConvert:         sdk.NewCoin(denomSrc, sdk.NewInt(1)),
			coinConvertedExpected: sdk.NewCoin(denomDst, sdk.NewInt(1000)),
			coinDrainedExpected:   sdk.NewCoin(denomSrc, sdk.NewInt(1)),
		},
		{
			name:                  "OK: 0.001000src -> 0.001dst",
			srcMeta:               TokenMeta{Denom: denomSrc, Decimals: 6},
			dstMeta:               TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert:         sdk.NewCoin(denomSrc, sdk.NewInt(1000)),
			coinConvertedExpected: sdk.NewCoin(denomDst, sdk.NewInt(1)),
			coinDrainedExpected:   sdk.NewCoin(denomSrc, sdk.NewInt(1000)),
		},
		{
			name:                  "OK: 1.000100src -> 1.000dst",
			srcMeta:               TokenMeta{Denom: denomSrc, Decimals: 6},
			dstMeta:               TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert:         sdk.NewCoin(denomSrc, sdk.NewInt(1000100)),
			coinConvertedExpected: sdk.NewCoin(denomDst, sdk.NewInt(1000)),
			coinDrainedExpected:   sdk.NewCoin(denomSrc, sdk.NewInt(1000000)),
		},
		{
			name:                  "OK: 0.001200src -> 0.001dst",
			srcMeta:               TokenMeta{Denom: denomSrc, Decimals: 6},
			dstMeta:               TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert:         sdk.NewCoin(denomSrc, sdk.NewInt(1200)),
			coinConvertedExpected: sdk.NewCoin(denomDst, sdk.NewInt(1)),
			coinDrainedExpected:   sdk.NewCoin(denomSrc, sdk.NewInt(1000)),
		},
		{
			name:                  "OK: 0.000001src -> 0.000dst",
			srcMeta:               TokenMeta{Denom: denomSrc, Decimals: 6},
			dstMeta:               TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert:         sdk.NewCoin(denomSrc, sdk.NewInt(1)),
			coinConvertedExpected: sdk.NewCoin(denomDst, sdk.NewInt(0)),
			coinDrainedExpected:   sdk.NewCoin(denomSrc, sdk.NewInt(0)),
		},
		{
			name:          "Fail: coin.Denom != srcMeta.Denom",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert: sdk.NewCoin(denomDst, sdk.NewInt(1)),
			errExpected:   true,
		},
		{
			name:          "Fail: invalid srcMeta.Denom",
			srcMeta:       TokenMeta{Denom: InvalidDenom, Decimals: 3},
			dstMeta:       TokenMeta{Denom: denomDst, Decimals: 3},
			coinToConvert: sdk.Coin{Denom: InvalidDenom, Amount: sdk.NewInt(1)},
			errExpected:   true,
		},
		{
			name:          "Fail: invalid dstMeta.Denom",
			srcMeta:       TokenMeta{Denom: denomSrc, Decimals: 3},
			dstMeta:       TokenMeta{Denom: InvalidDenom, Decimals: 3},
			coinToConvert: sdk.Coin{Denom: denomSrc, Amount: sdk.NewInt(1)},
			errExpected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coinConvertedReceived, coinDrainedReceived, err := tc.srcMeta.ConvertCoin2(tc.coinToConvert, tc.dstMeta)
			if tc.errExpected {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.EqualValuesf(t, tc.coinConvertedExpected, coinConvertedReceived, "converted: exp: %s / rcv: %s", tc.coinConvertedExpected.String(), coinConvertedReceived.String())
			require.EqualValuesf(t, tc.coinDrainedExpected, coinDrainedReceived, "drained: exp: %s / rcv: %s", tc.coinDrainedExpected.String(), coinConvertedReceived.String())
		})
	}
}
