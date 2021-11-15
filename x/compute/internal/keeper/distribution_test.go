package keeper

import (
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type DistInitMsg struct{}

type DistExecMsgRewards struct {
	Rewards Delegator `json:"rewards"`
}

type Delegator struct {
	Address string `json:"address"`
}

// TestDistributionRewards tests querying staking rewards from inside a contract - first testing no rewards, then advancing
// 1 block and checking the rewards again
func TestDistributionRewards(t *testing.T) {
	encoders := DefaultEncoders()
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, stakingKeeper, keeper, distKeeper := keepers.AccountKeeper, keepers.StakingKeeper, keepers.WasmKeeper, keepers.DistKeeper

	valAddr := addValidator(ctx, stakingKeeper, accKeeper, keeper.bankKeeper, sdk.NewInt64Coin("stake", 100))
	ctx = nextBlock(ctx, stakingKeeper)

	v, found := stakingKeeper.GetValidator(ctx, valAddr)
	assert.True(t, found)
	assert.Equal(t, v.GetDelegatorShares(), sdk.NewDec(100))

	depositCoin := sdk.NewInt64Coin(sdk.DefaultBondDenom, 5_000_000_000)
	deposit := sdk.NewCoins(depositCoin)
	creator, creatorPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)
	require.Equal(t, keeper.bankKeeper.GetBalance(ctx, creator, sdk.DefaultBondDenom), depositCoin)

	delTokens := sdk.TokensFromConsensusPower(1000, sdk.DefaultPowerReduction)
	msg2 := stakingtypes.NewMsgDelegate(creator, valAddr,
		sdk.NewCoin(sdk.DefaultBondDenom, delTokens))

	require.Equal(t, uint64(2), distKeeper.GetValidatorHistoricalReferenceCount(ctx))

	sh := staking.NewHandler(stakingKeeper)

	res2, err := sh(ctx, msg2)
	require.NoError(t, err)
	require.NotNil(t, res2)
	require.NoError(t, err)

	distKeeper.AllocateTokensToValidator(ctx, v, sdk.NewDecCoins(sdk.NewDecCoin("stake", sdk.NewInt(100))))

	// upload staking derivates code
	govCode, err := ioutil.ReadFile("./testdata/dist.wasm")
	require.NoError(t, err)
	govId, err := keeper.Create(ctx, creator, govCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), govId)

	// register to a valid address
	initMsg := DistInitMsg{}
	initBz, err := json.Marshal(&initMsg)
	require.NoError(t, err)
	initBz, err = testEncrypt(t, keeper, ctx, nil, govId, initBz)
	require.NoError(t, err)

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, creatorPrivKey, initBz, govId, nil)
	govAddr, err := keeper.Instantiate(ctx, govId, creator, initBz, "gidi gov", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, govAddr)

	queryReq := DistExecMsgRewards{
		Rewards: Delegator{
			Address: creator.String(),
		},
	}
	govQBz, err := json.Marshal(&queryReq)
	require.NoError(t, err)

	// test what happens if there are no rewards yet
	res, _, err := execHelper(t, keeper, ctx, govAddr, creator, creatorPrivKey, string(govQBz), false, defaultGasForTests, 0)
	require.Empty(t, err)
	// returns the rewards
	require.Equal(t, uint64(0), binary.BigEndian.Uint64(res))
	ctx = nextBlock(ctx, stakingKeeper)

	// test what happens if there are some rewards
	res, _, err = execHelper(t, keeper, ctx, govAddr, creator, creatorPrivKey, string(govQBz), false, defaultGasForTests, 0)
	require.Empty(t, err)
	// returns the rewards
	require.Equal(t, uint64(0x59), binary.BigEndian.Uint64(res))
}
