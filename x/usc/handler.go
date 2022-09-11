package usc

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/SecretNetwork/x/usc/keeper"
	"github.com/enigmampc/SecretNetwork/x/usc/types"
)

func NewHandler(k keeper.Keeper) sdk.Handler {
	msgServer := keeper.NewMsgServerImpl(k)

	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		if !k.Enabled(ctx) {
			return nil, sdkErrors.Wrapf(sdkErrors.ErrUnauthorized, "Module %s is currently disabled", types.ModuleName)
		}

		switch msg := msg.(type) {
		case *types.MsgMintUSC:
			res, err := msgServer.MintUSC(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgRedeemCollateral:
			res, err := msgServer.RedeemCollateral(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		default:
			return nil, sdkErrors.Wrapf(sdkErrors.ErrUnknownRequest, "unrecognized %s message type: %T", types.ModuleName, msg)
		}
	}
}
