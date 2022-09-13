package chainofsecretsreverttombstone

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type CosMints struct {
	Address     string `json:"address"`
	AmountUscrt string `json:"amount"`
}

var cosValidatorAddress = "secretvaloper1hscf4cjrhzsea5an5smt4z9aezhh4sf5jjrqka"

func LetsGo(
	ctx sdk.Context,
	bankKeeper bankkeeper.Keeper,
	stakingKeeper stakingkeeper.Keeper,
	mintKeeper mintkeeper.Keeper,
) {
	var cosMints []CosMints
	err := json.Unmarshal([]byte(recordsJsonString), &cosMints)
	if err != nil {
		panic(fmt.Sprintf("error reading COS JSON: %+v", err))
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

		_, err = stakingkeeper.NewMsgServerImpl(stakingKeeper).Delegate(sdk.WrapSDKContext(ctx), &stakingtypes.MsgDelegate{
			DelegatorAddress: mintRecord.Address,
			ValidatorAddress: cosValidatorAddress,
			Amount:           coin,
		})
		if err != nil {
			panic(fmt.Sprintf("error delegating minted %suscrt from %s to %s: %+v", mintRecord.AmountUscrt, mintRecord.Address, cosValidatorAddress, err))
		}
	}
}
