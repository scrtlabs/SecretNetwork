package types

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
)

func TestUSCRedeemEntryOperationValidate(t *testing.T) {
	type testCase struct {
		name           string
		creationTime   int64
		completionTime time.Time
		amount         sdk.Coins
		//
		errExpected bool
	}

	validCoins := sdk.NewCoins(
		sdk.NewCoin("usdt", sdk.OneInt()),
	)

	testCases := []testCase{
		{
			name:           "OK",
			creationTime:   1,
			completionTime: MockTimestamp,
			amount:         validCoins,
		},
		{
			name:           "Fail: CreationHeight < 0",
			creationTime:   -1,
			completionTime: MockTimestamp,
			amount:         validCoins,
			errExpected:    true,
		},
		{
			name:           "Fail: CompletionTime is empty",
			creationTime:   -1,
			completionTime: time.Time{},
			amount:         validCoins,
			errExpected:    true,
		},
		{
			name:           "Fail: CollateralAmount: invalid denom",
			creationTime:   1,
			completionTime: MockTimestamp,
			amount: sdk.Coins{
				sdk.Coin{Denom: InvalidDenom, Amount: sdk.OneInt()},
			},
			errExpected: true,
		},
		{
			name:           "Fail: CollateralAmount: amount EQ 0",
			creationTime:   1,
			completionTime: MockTimestamp,
			amount: sdk.Coins{
				sdk.Coin{Denom: ValidDenom, Amount: sdk.ZeroInt()},
			},
			errExpected: true,
		},
		{
			name:           "Fail: CollateralAmount: amount LT 0",
			creationTime:   1,
			completionTime: MockTimestamp,
			amount: sdk.Coins{
				sdk.Coin{Denom: ValidDenom, Amount: sdk.NewInt(-1)},
			},
			errExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			op := NewRedeemEntryOperation(tc.creationTime, tc.completionTime, tc.amount)
			if tc.errExpected {
				assert.Error(t, op.Validate())
				return
			}
			assert.NoError(t, op.Validate())
		})
	}
}

func TestUSCRedeemEntryValidate(t *testing.T) {
	type testCase struct {
		name string
		addr string
		op   RedeemEntryOperation
		//
		errExpected bool
	}

	validOp := NewRedeemEntryOperation(1, MockTimestamp, sdk.NewCoins(sdk.NewCoin(ValidDenom, sdk.OneInt())))
	invalidOp := NewRedeemEntryOperation(-1, MockTimestamp, sdk.NewCoins(sdk.NewCoin(ValidDenom, sdk.OneInt())))

	testCases := []testCase{
		{
			name: "OK",
			addr: ValidAddr,
			op:   validOp,
		},
		{
			name:        "Fail: invalid addr",
			addr:        InvalidAddr,
			op:          validOp,
			errExpected: true,
		},
		{
			name:        "Fail: invalid operation",
			addr:        InvalidAddr,
			op:          invalidOp,
			errExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := RedeemEntry{
				Address:    tc.addr,
				Operations: []RedeemEntryOperation{tc.op},
			}
			if tc.errExpected {
				assert.Error(t, entry.Validate())
				return
			}
			assert.NoError(t, entry.Validate())
		})
	}
}
