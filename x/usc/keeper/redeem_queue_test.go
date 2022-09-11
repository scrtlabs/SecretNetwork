package keeper_test

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUSCKeeperBeginRedeeming(t *testing.T) {
	te := NewTestEnv(t)
	ctx, keeper := te.ctx, te.app.USCKeeper

	// Fixtures
	accAddr1, _ := te.AddAccount(t, "")
	redeemCoins11 := sdk.NewCoins(
		sdk.NewCoin("uusdt", sdk.NewInt(1000)),
		sdk.NewCoin("musdc", sdk.NewInt(100)),
	)
	redeemCoins12 := sdk.NewCoins(
		sdk.NewCoin("abusd", sdk.NewInt(100000)),
	)
	redeemStartBlock1 := int64(1)
	redeemStartTime1 := MockTimestamp.Add(5 * time.Second)

	accAddr2, _ := te.AddAccount(t, "")
	redeemCoins2 := sdk.NewCoins(
		sdk.NewCoin("abusd", sdk.NewInt(1)),
		sdk.NewCoin("musdc", sdk.NewInt(10)),
	)
	redeemStartBlock2 := int64(2)
	redeemStartTime2 := redeemStartTime1.Add(5 * time.Second)

	// Add two redeems for account1
	ctx = ctx.WithBlockHeight(redeemStartBlock1)
	ctx = ctx.WithBlockTime(redeemStartTime1)

	_, err := keeper.BeginRedeeming(ctx, accAddr1, redeemCoins11)
	require.NoError(t, err)
	_, err = keeper.BeginRedeeming(ctx, accAddr1, redeemCoins12)
	require.NoError(t, err)

	// Add one redeem for account2
	ctx = ctx.WithBlockHeight(redeemStartBlock2)
	ctx = ctx.WithBlockTime(redeemStartTime2)

	_, err = keeper.BeginRedeeming(ctx, accAddr2, redeemCoins2)
	require.NoError(t, err)

	// Validate redeemEntry and queue data integrity for account1
	{
		expectedCompletionTime := redeemStartTime1.Add(keeper.RedeemDur(ctx))

		entry, found := keeper.GetRedeemEntry(ctx, accAddr1)
		require.True(t, found)

		assert.Equal(t, accAddr1.String(), entry.Address)

		require.Len(t, entry.Operations, 2)

		assert.Equal(t, redeemStartBlock1, entry.Operations[0].CreationHeight)
		assert.EqualValues(t, expectedCompletionTime, entry.Operations[0].CompletionTime)
		assert.EqualValues(t, redeemCoins11, entry.Operations[0].CollateralAmount)

		assert.Equal(t, redeemStartBlock1, entry.Operations[1].CreationHeight)
		assert.EqualValues(t, expectedCompletionTime, entry.Operations[1].CompletionTime)
		assert.EqualValues(t, redeemCoins12, entry.Operations[1].CollateralAmount)

		data := keeper.GetRedeemQueueTimeSlice(ctx, expectedCompletionTime)
		require.Len(t, data.Addresses, 2)
		assert.Equal(t, accAddr1.String(), data.Addresses[0])
		assert.Equal(t, accAddr1.String(), data.Addresses[1])
	}

	// Validate redeemEntry and queue data for account2
	{
		expectedCompletionTime := redeemStartTime2.Add(keeper.RedeemDur(ctx))

		entry, found := keeper.GetRedeemEntry(ctx, accAddr2)
		require.True(t, found)

		assert.Equal(t, accAddr2.String(), entry.Address)

		require.Len(t, entry.Operations, 1)

		assert.Equal(t, redeemStartBlock2, entry.Operations[0].CreationHeight)
		assert.EqualValues(t, expectedCompletionTime, entry.Operations[0].CompletionTime)
		assert.EqualValues(t, redeemCoins2, entry.Operations[0].CollateralAmount)

		data := keeper.GetRedeemQueueTimeSlice(ctx, expectedCompletionTime)
		require.Len(t, data.Addresses, 1)
		assert.Equal(t, accAddr2.String(), data.Addresses[0])
	}
}

func TestUSCKeeperRedeemingLimit(t *testing.T) {
	te := NewTestEnv(t)
	ctx, keeper := te.ctx, te.app.USCKeeper

	// Fixtures
	accAddr, _ := te.AddAccount(t, "")
	redeemCoins := sdk.NewCoins(sdk.NewCoin("uusdt", sdk.NewInt(1000)))

	// Fill up available slots
	for i := uint32(0); i < keeper.MaxRedeemEntries(ctx)-1; i++ {
		_, err := keeper.BeginRedeeming(ctx, accAddr, redeemCoins)
		assert.NoError(t, err)
	}

	// Limit reached
	_, err := keeper.BeginRedeeming(ctx, accAddr, redeemCoins)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrMaxRedeemEntries)
}

func TestUSCKeeperRedeemingQueue(t *testing.T) {
	const blockDur = 5 * time.Second

	te := NewTestEnv(t)
	keeper, bankKeeper := te.app.USCKeeper, te.app.BankKeeper

	// Fixtures
	accAddr1, _ := te.AddAccount(t, "")
	redeemCoins1 := sdk.NewCoins(
		sdk.NewCoin("uusdt", sdk.NewInt(1000)),
		sdk.NewCoin("musdc", sdk.NewInt(100)),
	)

	accAddr2, _ := te.AddAccount(t, "")
	redeemCoins2 := sdk.NewCoins(
		sdk.NewCoin("abusd", sdk.NewInt(100000)),
	)

	accAddr3, _ := te.AddAccount(t, "")
	redeemCoins31 := sdk.NewCoins(
		sdk.NewCoin("abusd", sdk.NewInt(1)),
		sdk.NewCoin("musdc", sdk.NewInt(10)),
	)
	redeemCoins32 := sdk.NewCoins(
		sdk.NewCoin("uusdt", sdk.NewInt(20)),
	)

	curRedeemingPoolCoins := redeemCoins1.Add(redeemCoins2...).Add(redeemCoins31...).Add(redeemCoins32...)
	te.AddRedeemingPoolBalance(t, curRedeemingPoolCoins.String())

	ctx, redeemDur := te.ctx, keeper.RedeemDur(te.ctx)
	var redeemTimestamp1, redeemTimestamp2, redeemTimestamp3 time.Time

	// Block 1: set 1st timeSlice with 2 entries (accAddr1, accAddr2)
	{
		ctx = ctx.WithBlockTime(ctx.BlockTime().Add(blockDur))
		redeemTimestamp1 = ctx.BlockTime().Add(redeemDur)

		redeemTimestamp1Rcv1, err := keeper.BeginRedeeming(ctx, accAddr1, redeemCoins1)
		require.NoError(t, err)
		assert.Equal(t, redeemTimestamp1, redeemTimestamp1Rcv1)

		redeemTimestamp1Rcv2, err := keeper.BeginRedeeming(ctx, accAddr2, redeemCoins2)
		require.NoError(t, err)
		assert.Equal(t, redeemTimestamp1, redeemTimestamp1Rcv2)
	}

	// Block 2: set 2nd timeSlice with 1 entry (accAddr3 1st part)
	{
		ctx = ctx.WithBlockTime(ctx.BlockTime().Add(blockDur))
		redeemTimestamp2 = ctx.BlockTime().Add(redeemDur)

		redeemTimestamp2Rcv, err := keeper.BeginRedeeming(ctx, accAddr3, redeemCoins31)
		require.NoError(t, err)
		assert.Equal(t, redeemTimestamp2, redeemTimestamp2Rcv)
	}

	// Block 3: set 3rd timeSlice with 1 entry (accAddr3 2nd part)
	{
		ctx = ctx.WithBlockTime(ctx.BlockTime().Add(blockDur))
		redeemTimestamp3 = ctx.BlockTime().Add(redeemDur)

		redeemTimestamp3Rcv, err := keeper.BeginRedeeming(ctx, accAddr3, redeemCoins32)
		require.NoError(t, err)
		assert.Equal(t, redeemTimestamp3, redeemTimestamp3Rcv)
	}

	// Block 4: ensure that none of the queue entries were triggered
	{
		ctx = ctx.WithBlockTime(ctx.BlockTime().Add(blockDur))
		keeper.EndRedeeming(ctx)

		assert.True(t, bankKeeper.GetAllBalances(ctx, accAddr1).IsZero())
		assert.True(t, bankKeeper.GetAllBalances(ctx, accAddr2).IsZero())
		assert.True(t, bankKeeper.GetAllBalances(ctx, accAddr3).IsZero())

		assert.EqualValues(t, curRedeemingPoolCoins, keeper.RedeemingPool(ctx))
	}

	// Block N+1 (redeemTimestamp1): check that 2 queue entries were processed, entries and queue data were removed
	{
		ctx = ctx.WithBlockTime(redeemTimestamp1)
		keeper.EndRedeeming(ctx)

		// Balances
		assert.EqualValues(t, redeemCoins1, bankKeeper.GetAllBalances(ctx, accAddr1))
		assert.EqualValues(t, redeemCoins2, bankKeeper.GetAllBalances(ctx, accAddr2))
		assert.True(t, bankKeeper.GetAllBalances(ctx, accAddr3).IsZero())

		curRedeemingPoolCoins = curRedeemingPoolCoins.Sub(redeemCoins1).Sub(redeemCoins2)
		assert.EqualValues(t, curRedeemingPoolCoins, keeper.RedeemingPool(ctx))

		// Redeem entries
		_, found := keeper.GetRedeemEntry(ctx, accAddr1)
		assert.False(t, found)

		_, found = keeper.GetRedeemEntry(ctx, accAddr2)
		assert.False(t, found)

		entry3, found := keeper.GetRedeemEntry(ctx, accAddr3)
		assert.True(t, found)
		assert.Len(t, entry3.Operations, 2)

		// Queue data
		data := keeper.GetRedeemQueueTimeSlice(ctx, redeemTimestamp1)
		assert.Len(t, data.Addresses, 0)
	}

	// Block N+2 (redeemTimestamp2 + 1 second): check that 1 queue entry was processed, entry modified and queue data was removed
	{
		ctx = ctx.WithBlockTime(redeemTimestamp2.Add(1 * time.Second))
		keeper.EndRedeeming(ctx)

		// Balances
		assert.EqualValues(t, redeemCoins1, bankKeeper.GetAllBalances(ctx, accAddr1))
		assert.EqualValues(t, redeemCoins2, bankKeeper.GetAllBalances(ctx, accAddr2))
		assert.EqualValues(t, redeemCoins31, bankKeeper.GetAllBalances(ctx, accAddr3))

		curRedeemingPoolCoins = curRedeemingPoolCoins.Sub(redeemCoins31)
		assert.EqualValues(t, curRedeemingPoolCoins, keeper.RedeemingPool(ctx))

		// Redeem entry
		entry3, found := keeper.GetRedeemEntry(ctx, accAddr3)
		assert.True(t, found)
		assert.Len(t, entry3.Operations, 1)

		// Queue data
		data := keeper.GetRedeemQueueTimeSlice(ctx, redeemTimestamp2)
		assert.Len(t, data.Addresses, 0)
	}

	// Block N+3 (redeemTimestamp3 + 2 seconds): check that 1 queue entry was processed, entry and queue data was removed
	{
		ctx = ctx.WithBlockTime(redeemTimestamp3.Add(2 * time.Second))
		keeper.EndRedeeming(ctx)

		// Balances
		assert.EqualValues(t, redeemCoins1, bankKeeper.GetAllBalances(ctx, accAddr1))
		assert.EqualValues(t, redeemCoins2, bankKeeper.GetAllBalances(ctx, accAddr2))
		assert.EqualValues(t, redeemCoins31.Add(redeemCoins32...), bankKeeper.GetAllBalances(ctx, accAddr3))

		curRedeemingPoolCoins = sdk.NewCoins()
		assert.EqualValues(t, curRedeemingPoolCoins, keeper.RedeemingPool(ctx))

		// Redeem entry
		_, found := keeper.GetRedeemEntry(ctx, accAddr3)
		assert.False(t, found)

		// Queue data
		data := keeper.GetRedeemQueueTimeSlice(ctx, redeemTimestamp3)
		assert.Len(t, data.Addresses, 0)
	}

	// Block N+4: check that the queue doesn't panic when emptied (just in case)
	{
		ctx = ctx.WithBlockTime(ctx.BlockTime().Add(blockDur))

		keeper.EndRedeeming(ctx)
	}
}
