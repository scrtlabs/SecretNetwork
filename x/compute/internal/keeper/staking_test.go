package keeper

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/cosmos/cosmos-sdk/codec/types"
	wasmtypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	wasmTypes "github.com/scrtlabs/SecretNetwork/x/compute/internal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type StakingInitMsg struct {
	Name      string         `json:"name"`
	Symbol    string         `json:"symbol"`
	Decimals  uint8          `json:"decimals"`
	Validator sdk.ValAddress `json:"validator"`
	ExitTax   math.LegacyDec `json:"exit_tax"`
	// MinWithdrawal is uint128 encoded as a string (use sdk.Int?)
	MinWithdrawl string `json:"min_withdrawal"`
}

// StakingHandleMsg is used to encode handle messages
type StakingHandleMsg struct {
	Transfer *transferPayload `json:"transfer,omitempty"`
	Bond     *struct{}        `json:"bond,omitempty"`
	Unbond   *unbondPayload   `json:"unbond,omitempty"`
	Claim    *struct{}        `json:"claim,omitempty"`
	Reinvest *struct{}        `json:"reinvest,omitempty"`
	Change   *ownerPayload    `json:"change_owner,omitempty"`
}

type transferPayload struct {
	Recipient sdk.Address `json:"recipient"`
	// uint128 encoded as string
	Amount string `json:"amount"`
}

type ownerPayload struct {
	Owner sdk.Address `json:"owner"`
}

type unbondPayload struct {
	// uint128 encoded as string
	Amount string `json:"amount"`
}

// StakingQueryMsg is used to encode query messages
type StakingQueryMsg struct {
	Balance    *addressQuery `json:"balance,omitempty"`
	Claims     *addressQuery `json:"claims,omitempty"`
	TokenInfo  *struct{}     `json:"token_info,omitempty"`
	Investment *struct{}     `json:"investment,omitempty"`
}

type addressQuery struct {
	Address sdk.AccAddress `json:"address"`
}

type BalanceResponse struct {
	Balance string `json:"balance,omitempty"`
}

type ClaimsResponse struct {
	Claims string `json:"claims,omitempty"`
}

type TokenInfoResponse struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals uint8  `json:"decimals"`
}

type InvestmentResponse struct {
	TokenSupply  string         `json:"token_supply"`
	StakedTokens sdk.Coin       `json:"staked_tokens"`
	NominalValue math.LegacyDec `json:"nominal_value"`
	Owner        sdk.AccAddress `json:"owner"`
	Validator    sdk.ValAddress `json:"validator"`
	ExitTax      math.LegacyDec `json:"exit_tax"`
	// MinWithdrawl is uint128 encoded as a string (use sdk.Int?)
	MinWithdrawl string `json:"min_withdrawl"`
}

func TestInitializeStaking(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource wasmTypes.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Codec)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, stakingKeeper, keeper := keepers.AccountKeeper, keepers.StakingKeeper, keepers.WasmKeeper

	valAddr := addValidator(ctx, stakingKeeper, accKeeper, keeper.bankKeeper, sdk.NewInt64Coin("stake", 1234567))
	ctx = nextBlock(ctx, stakingKeeper, keeper)
	v, err := stakingKeeper.GetValidator(ctx, valAddr)
	assert.True(t, err == nil)
	assert.Equal(t, v.GetDelegatorShares(), math.LegacyNewDec(1234567))

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000), sdk.NewInt64Coin("stake", 500000))
	creator, creatorPrivKey, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit, 5000)

	// upload staking derivates code
	stakingCode, err := os.ReadFile("./testdata/staking.wasm")
	require.NoError(t, err)
	stakingID, err := keeper.Create(ctx, creator, stakingCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), stakingID)

	// register to a valid address
	initMsg := StakingInitMsg{
		Name:         "Staking Derivatives",
		Symbol:       "DRV",
		Decimals:     0,
		Validator:    valAddr,
		ExitTax:      math.LegacyMustNewDecFromStr("0.10"),
		MinWithdrawl: "100",
	}
	initBz, err := json.Marshal(&initMsg)
	require.NoError(t, err)
	initBz, err = testEncrypt(t, keeper, ctx, nil, stakingID, initBz)
	require.NoError(t, err)

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, nil, creatorPrivKey, initBz, stakingID, nil)
	stakingAddr, _, err := keeper.Instantiate(ctx, stakingID, creator, nil, initBz, "staking derivates - DRV", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, stakingAddr)

	// nothing spent here
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, creator, deposit)

	// try to register with a validator not on the list and it fails
	_, _, bob := keyPubAddr()
	badInitMsg := StakingInitMsg{
		Name:         "Missing Validator",
		Symbol:       "MISS",
		Decimals:     0,
		Validator:    sdk.ValAddress(bob),
		ExitTax:      math.LegacyMustNewDecFromStr("0.10"),
		MinWithdrawl: "100",
	}
	badBz, err := json.Marshal(&badInitMsg)
	require.NoError(t, err)

	_, _, _, _, initErr := initHelper(t, keeper, ctx, stakingID, creator, nil, creatorPrivKey, string(badBz), true, false, defaultGasForTests)
	require.Error(t, initErr)
	require.Error(t, initErr.GenericErr)
	require.Equal(t, fmt.Sprintf("%s is not in the current validator set", sdk.ValAddress(bob).String()), initErr.GenericErr.Msg)

	// no changes to bonding shares
	val, _ := stakingKeeper.GetValidator(ctx, valAddr)
	assert.Equal(t, val.GetDelegatorShares(), math.LegacyNewDec(1234567))
}

type initInfo struct {
	valAddr      sdk.ValAddress
	creator      sdk.AccAddress
	contractAddr sdk.AccAddress

	ctx           sdk.Context
	accKeeper     authkeeper.AccountKeeper
	stakingKeeper stakingkeeper.Keeper
	distKeeper    distributionkeeper.Keeper
	bankKeeper    bankkeeper.Keeper
	wasmKeeper    Keeper

	cleanup func()
}

func initializeStaking(t *testing.T) initInfo {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource wasmTypes.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Codec)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, stakingKeeper, keeper := keepers.AccountKeeper, keepers.StakingKeeper, keepers.WasmKeeper

	valAddr := addValidator(ctx, stakingKeeper, accKeeper, keeper.bankKeeper, sdk.NewInt64Coin(sdk.DefaultBondDenom, 1000000))
	ctx = nextBlock(ctx, stakingKeeper, keeper)

	// set some baseline - this seems to be needed
	keepers.DistKeeper.SetValidatorHistoricalRewards(ctx, valAddr, 0, distributiontypes.ValidatorHistoricalRewards{
		CumulativeRewardRatio: sdk.DecCoins{},
		ReferenceCount:        1,
	})

	v, err := stakingKeeper.GetValidator(ctx, valAddr)
	assert.True(t, err == nil)
	assert.Equal(t, v.GetDelegatorShares(), math.LegacyNewDec(1000000))
	assert.Equal(t, v.Status, stakingtypes.Bonded)

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000), sdk.NewInt64Coin(sdk.DefaultBondDenom, 500000))
	creator, creatorPrivKey, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit, 5001)

	// upload staking derivates code
	stakingCode, err := os.ReadFile("./testdata/staking.wasm")
	require.NoError(t, err)
	stakingID, err := keeper.Create(ctx, creator, stakingCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), stakingID)

	// register to a valid address
	initMsg := StakingInitMsg{
		Name:         "Staking Derivatives",
		Symbol:       "DRV",
		Decimals:     0,
		Validator:    valAddr,
		ExitTax:      math.LegacyMustNewDecFromStr("0.10"),
		MinWithdrawl: "100",
	}
	initBz, err := json.Marshal(&initMsg)
	require.NoError(t, err)
	initBz, err = testEncrypt(t, keeper, ctx, nil, stakingID, initBz)
	require.NoError(t, err)

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, nil, creatorPrivKey, initBz, stakingID, nil)
	stakingAddr, _, err := keeper.Instantiate(ctx, stakingID, creator, nil, initBz, "staking derivates - DRV", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, stakingAddr)

	return initInfo{
		valAddr:       valAddr,
		creator:       creator,
		contractAddr:  stakingAddr,
		ctx:           ctx,
		accKeeper:     accKeeper,
		stakingKeeper: stakingKeeper,
		wasmKeeper:    keeper,
		distKeeper:    keepers.DistKeeper,
		bankKeeper:    keeper.bankKeeper,
		cleanup:       func() {},
	}
}

func checkAccount(t *testing.T, ctx sdk.Context, accKeeper authkeeper.AccountKeeper, bankKeeper bankkeeper.Keeper, addr sdk.AccAddress, expected sdk.Coins) {
	acct := accKeeper.GetAccount(ctx, addr)
	if expected == nil {
		assert.Nil(t, acct)
	} else {
		assert.NotNil(t, acct)
		coins := bankKeeper.GetAllBalances(ctx, acct.GetAddress())
		if expected.Empty() {
			// there is confusion between nil and empty slice... let's just treat them the same
			assert.True(t, coins.Empty())
		} else {
			assert.Equal(t, coins, expected)
		}
	}
}

func TestBonding(t *testing.T) {
	initInfo := initializeStaking(t)
	defer initInfo.cleanup()
	ctx, valAddr, contractAddr := initInfo.ctx, initInfo.valAddr, initInfo.contractAddr
	keeper, stakingKeeper, accKeeper := initInfo.wasmKeeper, initInfo.stakingKeeper, initInfo.accKeeper

	// initial checks of bonding state
	val, err := stakingKeeper.GetValidator(ctx, valAddr)
	require.True(t, err == nil)
	initPower := val.GetDelegatorShares()

	// bob has 160k, putting 80k into the contract
	full := sdk.NewCoins(sdk.NewInt64Coin("stake", 160000))
	funds := sdk.NewCoins(sdk.NewInt64Coin("stake", 80000))
	bob, privBob, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, full, 5002)

	// check contract state before
	assertBalance(t, ctx, keeper, contractAddr, bob, "0")
	assertClaims(t, ctx, keeper, contractAddr, bob, "0")
	assertSupply(t, ctx, keeper, contractAddr, "0", sdk.NewInt64Coin("stake", 0))

	bond := StakingHandleMsg{
		Bond: &struct{}{},
	}
	bondBz, err := json.Marshal(bond)
	require.NoError(t, err)
	bondBz, err = testEncrypt(t, keeper, ctx, contractAddr, 0, bondBz)
	require.NoError(t, err)
	ctx = PrepareExecSignedTx(t, keeper, ctx, bob, privBob, bondBz, contractAddr, funds)
	_, err = keeper.Execute(ctx, contractAddr, bob, bondBz, funds, nil, wasmtypes.HandleTypeExecute)
	require.NoError(t, err)

	// check some account values - the money is on neither account (cuz it is bonded)
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, contractAddr, sdk.Coins{})
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, bob, funds)

	// make sure the proper number of tokens have been bonded
	val, _ = stakingKeeper.GetValidator(ctx, valAddr)
	finalPower := val.GetDelegatorShares()
	assert.Equal(t, math.NewInt(80000), finalPower.Sub(initPower).TruncateInt())

	// check the delegation itself
	d, err := stakingKeeper.GetDelegation(ctx, contractAddr, valAddr)
	require.True(t, err == nil)
	assert.Equal(t, d.Shares, math.LegacyMustNewDecFromStr("80000"))

	// check we have the desired balance
	assertBalance(t, ctx, keeper, contractAddr, bob, "80000")
	assertClaims(t, ctx, keeper, contractAddr, bob, "0")
	assertSupply(t, ctx, keeper, contractAddr, "80000", sdk.NewInt64Coin("stake", 80000))
}

func TestUnbonding(t *testing.T) {
	initInfo := initializeStaking(t)
	defer initInfo.cleanup()
	ctx, valAddr, contractAddr := initInfo.ctx, initInfo.valAddr, initInfo.contractAddr
	keeper, stakingKeeper, accKeeper := initInfo.wasmKeeper, initInfo.stakingKeeper, initInfo.accKeeper

	// initial checks of bonding state
	val, err := stakingKeeper.GetValidator(ctx, valAddr)
	require.True(t, err == nil)
	initPower := val.GetDelegatorShares()

	// bob has 160k, putting 80k into the contract
	full := sdk.NewCoins(sdk.NewInt64Coin("stake", 160000))
	funds := sdk.NewCoins(sdk.NewInt64Coin("stake", 80000))
	bob, privBob, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, full, 5003)

	bond := StakingHandleMsg{
		Bond: &struct{}{},
	}
	bondBz, err := json.Marshal(bond)
	require.NoError(t, err)
	bondBz, err = testEncrypt(t, keeper, ctx, contractAddr, 0, bondBz)
	require.NoError(t, err)
	ctx = PrepareExecSignedTx(t, keeper, ctx, bob, privBob, bondBz, contractAddr, funds)
	_, err = keeper.Execute(ctx, contractAddr, bob, bondBz, funds, nil, wasmtypes.HandleTypeExecute)
	require.NoError(t, err)

	// update height a bit
	ctx = nextBlock(ctx, stakingKeeper, keeper)

	// now unbond 30k - note that 3k (10%) goes to the owner as a tax, 27k unbonded and available as claims
	unbond := StakingHandleMsg{
		Unbond: &unbondPayload{
			Amount: "30000",
		},
	}
	unbondBz, err := json.Marshal(unbond)
	require.NoError(t, err)
	unbondBz, err = testEncrypt(t, keeper, ctx, contractAddr, 0, unbondBz)
	require.NoError(t, err)
	ctx = PrepareExecSignedTx(t, keeper, ctx, bob, privBob, unbondBz, contractAddr, nil)
	_, err = keeper.Execute(ctx, contractAddr, bob, unbondBz, nil, nil, wasmtypes.HandleTypeExecute)
	require.NoError(t, err)

	// check some account values - the money is on neither account (cuz it is bonded)
	// Note: why is this immediate? just test setup?
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, contractAddr, sdk.Coins{})
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, bob, funds)

	// make sure the proper number of tokens have been bonded (80k - 27k = 53k)
	val, _ = stakingKeeper.GetValidator(ctx, valAddr)
	finalPower := val.GetDelegatorShares()
	assert.Equal(t, math.NewInt(53000), finalPower.Sub(initPower).TruncateInt(), finalPower.String())

	// check the delegation itself
	d, err := stakingKeeper.GetDelegation(ctx, contractAddr, valAddr)
	require.True(t, err == nil)
	assert.Equal(t, d.Shares, math.LegacyMustNewDecFromStr("53000"))

	// check there is unbonding in progress
	un, err := stakingKeeper.GetUnbondingDelegation(ctx, contractAddr, valAddr)
	require.True(t, err == nil)
	require.Equal(t, 1, len(un.Entries))
	assert.Equal(t, "27000", un.Entries[0].Balance.String())

	// check we have the desired balance
	assertBalance(t, ctx, keeper, contractAddr, bob, "50000")
	assertBalance(t, ctx, keeper, contractAddr, initInfo.creator, "3000")
	assertClaims(t, ctx, keeper, contractAddr, bob, "27000")
	assertSupply(t, ctx, keeper, contractAddr, "53000", sdk.NewInt64Coin("stake", 53000))
}

func TestReinvest(t *testing.T) {
	initInfo := initializeStaking(t)
	defer initInfo.cleanup()
	ctx, valAddr, contractAddr := initInfo.ctx, initInfo.valAddr, initInfo.contractAddr
	keeper, stakingKeeper, accKeeper := initInfo.wasmKeeper, initInfo.stakingKeeper, initInfo.accKeeper
	distKeeper := initInfo.distKeeper
	bankKeeper := initInfo.bankKeeper
	// initial checks of bonding state
	val, err := stakingKeeper.GetValidator(ctx, valAddr)
	require.True(t, err == nil)
	initPower := val.GetDelegatorShares()
	assert.Equal(t, val.Tokens, math.NewInt(1000000), "%s", val.Tokens)

	// full is 2x funds, 1x goes to the contract, other stays on his wallet
	full := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 400000))
	funds := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 200000))
	bob, privBob, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, full, 5004)

	// we will stake 200k to a validator with 1M self-bond
	// this means we should get 1/6 of the rewards
	bond := StakingHandleMsg{
		Bond: &struct{}{},
	}
	bondBz, err := json.Marshal(bond)
	require.NoError(t, err)
	bondBz, err = testEncrypt(t, keeper, ctx, contractAddr, 0, bondBz)
	require.NoError(t, err)
	ctx = PrepareExecSignedTx(t, keeper, ctx, bob, privBob, bondBz, contractAddr, funds)
	_, err = keeper.Execute(ctx, contractAddr, bob, bondBz, funds, nil, wasmtypes.HandleTypeExecute)
	require.NoError(t, err)

	// update height a bit to solidify the delegation
	ctx = nextBlock(ctx, stakingKeeper, keeper)
	// we get 1/6, our share should be 40k minus 10% commission = 36k
	setValidatorRewards(ctx, bankKeeper, stakingKeeper, distKeeper, valAddr, sdk.NewInt64Coin(sdk.DefaultBondDenom, 240000))

	// this should withdraw our outstanding 40k of rewards and reinvest them in the same delegation
	reinvest := StakingHandleMsg{
		Reinvest: &struct{}{},
	}
	reinvestBz, err := json.Marshal(reinvest)
	require.NoError(t, err)
	reinvestBz, err = testEncrypt(t, keeper, ctx, contractAddr, 0, reinvestBz)
	require.NoError(t, err)
	ctx = PrepareExecSignedTx(t, keeper, ctx, bob, privBob, reinvestBz, contractAddr, nil)
	_, err = keeper.Execute(ctx, contractAddr, bob, reinvestBz, nil, nil, wasmtypes.HandleTypeExecute)
	require.NoError(t, err)

	// check some account values - the money is on neither account (cuz it is bonded)
	// Note: why is this immediate? just test setup?
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, contractAddr, sdk.Coins{})
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, bob, funds)

	// check the delegation itself
	d, err := stakingKeeper.GetDelegation(ctx, contractAddr, valAddr)
	require.True(t, err == nil)
	// we started with 200k and added 36k
	assert.Equal(t, d.Shares, math.LegacyMustNewDecFromStr("236000"))

	// make sure the proper number of tokens have been bonded (80k + 40k = 120k)
	val, _ = stakingKeeper.GetValidator(ctx, valAddr)
	finalPower := val.GetDelegatorShares()
	assert.Equal(t, math.NewInt(236000), finalPower.Sub(initPower).TruncateInt(), finalPower.String())

	// check there is no unbonding in progress
	un, err := stakingKeeper.GetUnbondingDelegation(ctx, contractAddr, valAddr)
	assert.False(t, err == nil, "%#v", un)

	// check we have the desired balance
	assertBalance(t, ctx, keeper, contractAddr, bob, "200000")
	assertBalance(t, ctx, keeper, contractAddr, initInfo.creator, "0")
	assertClaims(t, ctx, keeper, contractAddr, bob, "0")
	assertSupply(t, ctx, keeper, contractAddr, "200000", sdk.NewInt64Coin("stake", 236000))
}

// adds a few validators and returns a list of validators that are registered
func addValidator(ctx sdk.Context, stakingKeeper stakingkeeper.Keeper, accountKeeper authkeeper.AccountKeeper, bankKeeper bankkeeper.Keeper, value sdk.Coin) sdk.ValAddress {
	accAddr, _, pub := CreateFakeFundedAccount(ctx, accountKeeper, bankKeeper, sdk.Coins{value}, 6000)

	addr := sdk.ValAddress(accAddr)

	anypub, _ := types.NewAnyWithValue(pub)
	msg := stakingtypes.MsgCreateValidator{
		Description: stakingtypes.Description{
			Moniker: "Validator power",
		},
		Commission: stakingtypes.CommissionRates{
			Rate:          math.LegacyMustNewDecFromStr("0.1"),
			MaxRate:       math.LegacyMustNewDecFromStr("0.2"),
			MaxChangeRate: math.LegacyMustNewDecFromStr("0.01"),
		},
		MinSelfDelegation: math.OneInt(),
		DelegatorAddress:  addr.String(),
		ValidatorAddress:  addr.String(),
		Pubkey:            anypub,
		Value:             value,
	}

	h := stakingkeeper.NewMsgServerImpl(&stakingKeeper)
	_, err := h.CreateValidator(ctx, &msg)
	if err != nil {
		panic(err)
	}
	return addr
}

// this will commit the current set, update the block height and set historic info
// basically, letting two blocks pass
func nextBlock(ctx sdk.Context, stakingKeeper stakingkeeper.Keeper, _ Keeper) sdk.Context {
	// unusded param wasmKeeper
	stakingKeeper.EndBlocker(ctx)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	stakingKeeper.BeginBlocker(ctx)
	// StoreRandomOnNewBlock(ctx, wasmKeeper)

	return ctx
}

func setValidatorRewards(ctx sdk.Context, bankKeeper bankkeeper.Keeper, stakingKeeper stakingkeeper.Keeper, distKeeper distributionkeeper.Keeper, valAddr sdk.ValAddress, rewards ...sdk.Coin) {
	// allocate some rewards
	validator, _ := stakingKeeper.Validator(ctx, valAddr)
	payout := sdk.NewDecCoinsFromCoins(rewards...)
	distKeeper.AllocateTokensToValidator(ctx, validator, payout)

	// allocate rewards to validator by minting tokens to distr module balance
	err := bankKeeper.MintCoins(ctx, faucetAccountName, rewards)
	if err != nil {
		panic(err)
	}

	err = bankKeeper.SendCoinsFromModuleToModule(ctx, faucetAccountName, distributiontypes.ModuleName, rewards)
	if err != nil {
		panic(err)
	}
}

func assertBalance(t *testing.T, ctx sdk.Context, keeper Keeper, contract sdk.AccAddress, addr sdk.AccAddress, expected string) {
	query := StakingQueryMsg{
		Balance: &addressQuery{
			Address: addr,
		},
	}
	queryBz, err := json.Marshal(query)
	require.NoError(t, err)

	res, qErr := queryHelper(t, keeper, ctx, contract, string(queryBz), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	var balance BalanceResponse
	err = json.Unmarshal([]byte(res), &balance)
	require.NoError(t, err)
	assert.Equal(t, expected, balance.Balance)
}

func assertClaims(t *testing.T, ctx sdk.Context, keeper Keeper, contract sdk.AccAddress, addr sdk.AccAddress, expected string) {
	query := StakingQueryMsg{
		Claims: &addressQuery{
			Address: addr,
		},
	}
	queryBz, err := json.Marshal(query)
	require.NoError(t, err)

	res, qErr := queryHelper(t, keeper, ctx, contract, string(queryBz), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	var claims ClaimsResponse
	err = json.Unmarshal([]byte(res), &claims)
	require.NoError(t, err)
	assert.Equal(t, expected, claims.Claims)
}

func assertSupply(t *testing.T, ctx sdk.Context, keeper Keeper, contract sdk.AccAddress, expectedIssued string, expectedBonded sdk.Coin) {
	query := StakingQueryMsg{Investment: &struct{}{}}
	queryBz, err := json.Marshal(query)
	require.NoError(t, err)
	res, qErr := queryHelper(t, keeper, ctx, contract, string(queryBz), true, false, defaultGasForTests)
	require.Empty(t, qErr)

	var invest InvestmentResponse
	err = json.Unmarshal([]byte(res), &invest)
	require.NoError(t, err)
	assert.Equal(t, expectedIssued, invest.TokenSupply)
	assert.Equal(t, expectedBonded.Amount, invest.StakedTokens.Amount)
}
