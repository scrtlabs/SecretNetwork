package keeper_test

import (
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/usc/keeper"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUSCUSCSupplyInvariant(t *testing.T) {
	type (
		account struct {
			accAddr    sdk.AccAddress
			uscBalance sdk.Coin
		}

		testCase struct {
			name      string
			poolCoins sdk.Coins
			accs      []account
			//
			brokenExpected bool
		}
	)

	accAddr1 := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address())
	accAddr2 := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address())
	accAddr3 := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address())

	busdSupplyAmt := sdk.NewInt(3000000000) // 3.0 BUSD
	busdConvertedAmt := sdk.NewInt(3000000) // 3.0 USC

	usdtSupplyuAmt := sdk.NewInt(2000000)   // 2.0 USDT
	usdtConvertedAmt := sdk.NewInt(2000000) // 2.0 USC

	usdcSupplyAmt := sdk.NewInt(1000)       // 1.0 USDC
	usdcConvertedAmt := sdk.NewInt(1000000) // 1.0 USC

	testCases := []testCase{
		{
			name: "OK",
			poolCoins: sdk.NewCoins(
				sdk.NewCoin("nbusd", busdSupplyAmt),
				sdk.NewCoin("uusdt", usdtSupplyuAmt),
				sdk.NewCoin("musdc", usdcSupplyAmt),
			),
			accs: []account{
				{
					accAddr:    accAddr1,
					uscBalance: sdk.NewCoin(types.DefaultUSCDenom, busdConvertedAmt),
				},
				{
					accAddr:    accAddr2,
					uscBalance: sdk.NewCoin(types.DefaultUSCDenom, usdtConvertedAmt),
				},
				{
					accAddr:    accAddr3,
					uscBalance: sdk.NewCoin(types.DefaultUSCDenom, usdcConvertedAmt),
				},
			},
		},
		{
			name: "Fail",
			poolCoins: sdk.NewCoins(
				sdk.NewCoin("nbusd", busdSupplyAmt),
				sdk.NewCoin("uusdt", usdtSupplyuAmt),
				sdk.NewCoin("musdc", usdcSupplyAmt),
			),
			accs: []account{
				{
					accAddr:    accAddr1,
					uscBalance: sdk.NewCoin(types.DefaultUSCDenom, busdConvertedAmt),
				},
				{
					accAddr:    accAddr2,
					uscBalance: sdk.NewCoin(types.DefaultUSCDenom, usdtConvertedAmt),
				},
			},
			brokenExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			te := NewTestEnv(t)
			ctx, k, bankKeeper := te.ctx, te.app.USCKeeper, te.app.BankKeeper

			te.AddActivePoolBalance(t, tc.poolCoins.String())

			for _, acc := range tc.accs {
				uscCoin := te.AddActivePoolBalance(t, acc.uscBalance.String())
				require.NoError(t, bankKeeper.SendCoinsFromModuleToAccount(te.ctx, types.ActivePoolName, acc.accAddr, uscCoin))
			}

			msg, broken := keeper.USCSupplyInvariant(k)(ctx)
			assert.Equalf(t, tc.brokenExpected, broken, msg)
		})
	}
}

func TestUSCRedeemingQueueInvariant(t *testing.T) {
	type (
		testCase struct {
			name      string
			poolCoins sdk.Coins
			entries   []types.RedeemEntry
			//
			brokenExpected bool
		}
	)

	accAddr1 := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address())
	accAddr2 := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address())
	accAddr3 := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address())

	testCases := []testCase{
		{
			name: "OK",
			poolCoins: sdk.NewCoins(
				sdk.NewCoin("usdt", sdk.NewInt(100)),
				sdk.NewCoin("msdc", sdk.NewInt(1)),
			),
			entries: []types.RedeemEntry{
				{
					Address: accAddr1.String(),
					Operations: []types.RedeemEntryOperation{
						{
							CreationHeight: 0,
							CompletionTime: MockTimestamp,
							CollateralAmount: sdk.NewCoins(
								sdk.NewCoin("usdt", sdk.NewInt(50)),
							),
						},
					},
				},
				{
					Address: accAddr2.String(),
					Operations: []types.RedeemEntryOperation{
						{
							CreationHeight: 0,
							CompletionTime: MockTimestamp.Add(1 * time.Second),
							CollateralAmount: sdk.NewCoins(
								sdk.NewCoin("usdt", sdk.NewInt(50)),
							),
						},
					},
				},
				{
					Address: accAddr3.String(),
					Operations: []types.RedeemEntryOperation{
						{
							CreationHeight: 0,
							CompletionTime: MockTimestamp.Add(2 * time.Second),
							CollateralAmount: sdk.NewCoins(
								sdk.NewCoin("msdc", sdk.NewInt(1)),
							),
						},
					},
				},
			},
		},
		{
			name: "Fail",
			poolCoins: sdk.NewCoins(
				sdk.NewCoin("usdt", sdk.NewInt(100)),
			),
			entries: []types.RedeemEntry{
				{
					Address: accAddr1.String(),
					Operations: []types.RedeemEntryOperation{
						{
							CreationHeight: 0,
							CompletionTime: MockTimestamp,
							CollateralAmount: sdk.NewCoins(
								sdk.NewCoin("usdt", sdk.NewInt(50)),
							),
						},
					},
				},
			},
			brokenExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			te := NewTestEnv(t)
			ctx, k := te.ctx, te.app.USCKeeper

			te.AddRedeemingPoolBalance(t, tc.poolCoins.String())

			for _, entry := range tc.entries {
				k.SetRedeemEntry(ctx, entry)
			}

			msg, broken := keeper.RedeemingQueueInvariant(k)(ctx)
			assert.Equalf(t, tc.brokenExpected, broken, msg)
		})
	}
}
