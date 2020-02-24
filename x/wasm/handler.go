package wasm

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	AttributeKeyContract = "contract_address"
	AttributeKeyCodeID   = "code_id"
	AttributeSigner      = "signer"
)

// NewHandler returns a handler for "bank" type messages.
func NewHandler(k Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		case MsgStoreCode:
			return handleStoreCode(ctx, k, &msg)
		case *MsgStoreCode:
			return handleStoreCode(ctx, k, msg)

		case MsgInstantiateContract:
			return handleInstantiate(ctx, k, &msg)
		case *MsgInstantiateContract:
			return handleInstantiate(ctx, k, msg)

		case MsgExecuteContract:
			return handleExecute(ctx, k, &msg)
		case *MsgExecuteContract:
			return handleExecute(ctx, k, msg)

		default:
			errMsg := fmt.Sprintf("unrecognized wasm message type: %T", msg)
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, errMsg)
		}
	}
}

func handleStoreCode(ctx sdk.Context, k Keeper, msg *MsgStoreCode) (*sdk.Result, error) {
	err := msg.ValidateBasic()
	if err != nil {
		return nil, err
	}

	codeID, err := k.Create(ctx, msg.Sender, msg.WASMByteCode, msg.Source, msg.Builder)
	if err != nil {
		return nil, err
	}

	// de-duplicate events sent in other parts of the sdk
	// https://github.com/confio/cosm-js/pull/40#discussion_r374811810
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
			sdk.NewAttribute(AttributeSigner, msg.Sender.String()),
			sdk.NewAttribute(AttributeKeyCodeID, fmt.Sprintf("%d", codeID)),
		),
	)

	return &sdk.Result{
		Data:   []byte(fmt.Sprintf("%d", codeID)),
		Events: ctx.EventManager().Events(),
	}, nil
}

func handleInstantiate(ctx sdk.Context, k Keeper, msg *MsgInstantiateContract) (*sdk.Result, error) {
	contractAddr, err := k.Instantiate(ctx, msg.Code, msg.Sender, msg.InitMsg, msg.InitFunds)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
			sdk.NewAttribute(AttributeSigner, msg.Sender.String()),
			sdk.NewAttribute(AttributeKeyCodeID, fmt.Sprintf("%d", msg.Code)),
			sdk.NewAttribute(AttributeKeyContract, contractAddr.String()),
		),
	)

	return &sdk.Result{
		Data:   contractAddr,
		Events: ctx.EventManager().Events(),
	}, nil
}

func handleExecute(ctx sdk.Context, k Keeper, msg *MsgExecuteContract) (*sdk.Result, error) {
	res, err := k.Execute(ctx, msg.Contract, msg.Sender, msg.Msg, msg.SentFunds)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
			sdk.NewAttribute(AttributeSigner, msg.Sender.String()),
			sdk.NewAttribute(AttributeKeyContract, msg.Contract.String()),
		),
	)

	res.Events = append(res.Events, ctx.EventManager().Events()...)
	return &res, nil
}
