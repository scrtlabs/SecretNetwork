package tokenswap

import (
	"fmt"

	"github.com/enigmampc/Enigmachain/x/tokenswap/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// NewHandler returns a handler for "tokenswap" type messages.
func NewHandler(keeper Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())
		switch msg := msg.(type) {
		case types.MsgTokenSwap:
			return handleMsgTokenSwap(ctx, keeper, msg)
		default:
			errMsg := fmt.Sprintf("unrecognized tokenswap message type: %v", msg.Type())
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, errMsg)
		}
	}
}

// Handle a message to create a token swap
func handleMsgTokenSwap(
	ctx sdk.Context, keeper Keeper, msg types.MsgTokenSwap,
) (*sdk.Result, error) {
	// Check if the this tokeswap request was alread processed
	_, err := keeper.GetPastTokenSwapRequest(ctx, msg.EthereumTxHash)

	if err == nil {
		// msg.EthereumTxHash already exists in db
		// So this request was already processed
		return nil, sdkerrors.Wrap(
			sdkerrors.ErrUnauthorized,
			fmt.Sprintf(
				"TokenSwap with EthereumTxHash %s was already processed",
				msg.EthereumTxHash,
			),
		)
	}

	err = keeper.ProcessTokenSwapRequest(
		ctx,
		msg.EthereumTxHash,
		msg.EthereumSender,
		msg.Receiver,
		msg.AmountENG,
	)
	return &sdk.Result{}, err

}
