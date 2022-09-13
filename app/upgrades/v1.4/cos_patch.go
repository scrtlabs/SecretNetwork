package v1_4

import (
	"encoding/json"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type CosMints struct {
	Address     string `json:"address"`
	AmountUscrt string `json:"amount"`
}

var cosValidatorAddress = "secretvaloper1hscf4cjrhzsea5an5smt4z9aezhh4sf5jjrqka"
var cosConsensusAddress = "secretvalcons1hscf4cjrhzsea5an5smt4z9aezhh4sf5xpsu6u"

func mintLostTokens(
	ctx sdk.Context,
	bankKeeper *bankkeeper.BaseKeeper,
	stakingKeeper *stakingkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper,
) {
	var cosMints []CosMints
	err := json.Unmarshal([]byte(recordsJsonString), &cosMints)
	if err != nil {
		panic(fmt.Sprintf("error reading COS JSON: %+v", err))
	}

	cosValAddress, err := sdk.ValAddressFromBech32(cosValidatorAddress)
	if err != nil {
		panic(fmt.Sprintf("validator address is not valid bech32: %s", cosValAddress))
	}

	cosValidator, found := stakingKeeper.GetValidator(ctx, cosValAddress)
	if !found {
		panic(fmt.Sprintf("cos validator not found"))
	}

	for _, mintRecord := range cosMints {
		coinAmount, mintOk := sdk.NewIntFromString(mintRecord.AmountUscrt)
		if !mintOk {
			panic(fmt.Sprintf("error parsing mint of %suscrt to %s", mintRecord.AmountUscrt, mintRecord.Address))
		}

		coin := sdk.NewCoin("uscrt", coinAmount)
		coins := sdk.NewCoins(coin)

		err = mintKeeper.MintCoins(ctx, coins)
		if err != nil {
			panic(fmt.Sprintf("error minting %suscrt to %s: %+v", mintRecord.AmountUscrt, mintRecord.Address, err))
		}

		delegatorAccount, err := sdk.AccAddressFromBech32(mintRecord.Address)
		if err != nil {
			panic(fmt.Sprintf("error converting human address %s to sdk.AccAddress: %+v", mintRecord.Address, err))
		}

		err = bankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, delegatorAccount, coins)
		if err != nil {
			panic(fmt.Sprintf("error sending minted %suscrt to %s: %+v", mintRecord.AmountUscrt, mintRecord.Address, err))
		}

		sdkAddress, err := sdk.AccAddressFromBech32(mintRecord.Address)
		if err != nil {
			panic(fmt.Sprintf("account address is not valid bech32: %s", mintRecord.Address))
		}

		_, err = stakingKeeper.Delegate(ctx, sdkAddress, coin.Amount, stakingtypes.Unbonded, cosValidator, true)
		if err != nil {
			panic(fmt.Sprintf("error delegating minted %suscrt from %s to %s: %+v", mintRecord.AmountUscrt, mintRecord.Address, cosValidatorAddress, err))
		}
	}
}

func revertTombstone(ctx sdk.Context, slashingKeeper *slashingkeeper.Keeper) error {

	cosValAddress, err := sdk.ValAddressFromBech32(cosValidatorAddress)
	if err != nil {
		panic(fmt.Sprintf("validator address is not valid bech32: %s", cosValAddress))
	}

	cosConsAddress, err := sdk.ConsAddressFromBech32(cosConsensusAddress)
	if err != nil {
		panic(fmt.Sprintf("consensus address is not valid bech32: %s", cosValAddress))
	}

	slashingKeeper.RevertTombstone(ctx, cosConsAddress)
	err = slashingKeeper.Unjail(ctx, cosValAddress, true)
	if err != nil {
		return err
	}

	return nil
}

func RevertCosTombstoning(
	ctx sdk.Context,
	slashingKeeper *slashingkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper,
	bankKeeper *bankkeeper.BaseKeeper,
	stakingKeeper *stakingkeeper.Keeper,
) error {
	err := revertTombstone(ctx, slashingKeeper)
	if err != nil {
		return err
	}

	mintLostTokens(ctx, bankKeeper, stakingKeeper, mintKeeper)

	return nil
}
