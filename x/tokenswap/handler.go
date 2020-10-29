package tokenswap

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// NewHandler returns a handler for "tokenswap" type messages.
func NewHandler(keeper SwapKeeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())
		switch msg := msg.(type) {
		case MsgSwapRequest:
			return handleMsgTokenSwap(ctx, keeper, msg)
		default:
			errMsg := fmt.Sprintf("unrecognized tokenswap message type: %v", msg.Type())
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, errMsg)
		}
	}
}

// Handle a message to create a token swap
func handleMsgTokenSwap(
	ctx sdk.Context, keeper SwapKeeper, msg MsgSwapRequest,
) (*sdk.Result, error) {

	err := keeper.SwapIsEnabled(ctx)
	if err != nil {
		return nil, sdkerrors.Wrap(
			sdkerrors.ErrUnauthorized,
			err.Error(),
		)
	}

	// validate signer
	err = keeper.ValidateTokenSwapSigner(ctx, msg.SignerAddr)
	if err != nil {
		return nil, sdkerrors.Wrap(
			sdkerrors.ErrUnauthorized,
			err.Error(),
		)
	}

	// Check if the this tokeswap request was alread processed
	swapRecord, err := keeper.GetPastTokenSwapRequest(ctx, msg.BurnTxHash)
	if err == nil {
		// msg.EthereumTxHash already exists in db
		// So this request was already processed
		// Check if we might have failed processing the transaction
		if swapRecord.Done {
			return nil, sdkerrors.Wrap(
				sdkerrors.ErrUnauthorized,
				fmt.Sprintf(
					"TokenSwap with EthereumTxHash %s was already processed",
					msg.BurnTxHash,
				),
			)
		}
	}

	err = keeper.ProcessTokenSwapRequest(
		ctx,
		msg,
	)
	return &sdk.Result{}, err

}
