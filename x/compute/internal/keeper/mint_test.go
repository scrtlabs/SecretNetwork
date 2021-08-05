package keeper

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type MintInitMsg struct{}

type MintExecMsgInflation struct {
	InflationRate MintInitMsg `json:"inflation_rate"`
}

type MintExecMsgBondedRatio struct {
	BondedRatio MintInitMsg `json:"bonded_ratio"`
}

// TestMintQuerier
func TestMintQuerier(t *testing.T) {
	encoders := DefaultEncoders()
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, stakingKeeper, keeper, distKeeper := keepers.AccountKeeper, keepers.StakingKeeper, keepers.WasmKeeper, keepers.DistKeeper

	valAddr := addValidator(ctx, stakingKeeper, accKeeper, keeper.bankKeeper, sdk.NewInt64Coin(sdk.DefaultBondDenom, 100))
	ctx = nextBlock(ctx, stakingKeeper)

	v, found := stakingKeeper.GetValidator(ctx, valAddr)
	assert.True(t, found)
	assert.Equal(t, v.GetDelegatorShares(), sdk.NewDec(100))

	deposit := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 5_000_000_000))
	creator, creatorPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)

	delTokens := sdk.TokensFromConsensusPower(1000, sdk.DefaultPowerReduction)
	msg2 := stakingtypes.NewMsgDelegate(creator, valAddr,
		sdk.NewCoin(sdk.DefaultBondDenom, delTokens))

	require.Equal(t, uint64(2), distKeeper.GetValidatorHistoricalReferenceCount(ctx))

	sh := staking.NewHandler(stakingKeeper)

	res2, err := sh(ctx, msg2)
	require.NoError(t, err)
	require.NotNil(t, res2)
	require.NoError(t, err)

	distKeeper.AllocateTokensToValidator(ctx, v, sdk.NewDecCoins(sdk.NewDecCoin(sdk.DefaultBondDenom, sdk.NewInt(100))))

	// upload staking derivates code
	govCode, err := ioutil.ReadFile("./testdata/mint.wasm")
	require.NoError(t, err)
	govId, err := keeper.Create(ctx, creator, govCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), govId)

	// register to a valid address
	initMsg := MintInitMsg{}
	initBz, err := json.Marshal(&initMsg)
	require.NoError(t, err)
	initBz, err = testEncrypt(t, keeper, ctx, nil, govId, initBz)
	require.NoError(t, err)

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, creatorPrivKey, initBz, govId, nil)
	govAddr, err := keeper.Instantiate(ctx, govId, creator, initBz, "gidi gov", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, govAddr)

	queryReq := MintExecMsgInflation{}
	govQBz, err := json.Marshal(&queryReq)
	require.NoError(t, err)

	// test what happens if there are no rewards yet
	res, _, err := execHelper(t, keeper, ctx, govAddr, creator, creatorPrivKey, string(govQBz), false, defaultGasForTests, 0)
	require.Empty(t, err)
	// returns the rewards
	require.Equal(t, "0.130000000000000000", string(res))

	queryReq2 := MintExecMsgBondedRatio{}
	govQBz2, err := json.Marshal(&queryReq2)
	require.NoError(t, err)

	ctx = nextBlock(ctx, stakingKeeper)

	// test what happens if there are some rewards
	res, _, err = execHelper(t, keeper, ctx, govAddr, creator, creatorPrivKey, string(govQBz2), false, defaultGasForTests, 0)
	require.Empty(t, err)
	// returns the rewards
	require.Equal(t, "0.199920047982406077", string(res))
}
