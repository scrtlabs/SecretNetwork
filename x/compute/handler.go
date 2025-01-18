package compute

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"

	errorsmod "cosmossdk.io/errors"
	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	wasmtypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

// NewHandler returns a handler for "compute" type messages.
// We still need this legacy handler to pass reply info in the data field
// as the new grpc handler truncates the data field if there's an error
// this handler is only used here: https://github.com/scrtlabs/SecretNetwork/blob/d8492253/x/compute/internal/keeper/handler_plugin.go#L574-L582
// As a reference point see the x/bank legacy msg handler which just wraps the new grpc handler https://github.com/scrtlabs/cosmos-sdk/blob/67c2d41286/x/bank/handler.go#L10-L30
func NewHandler(k Keeper) baseapp.MsgServiceHandler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {
		case *MsgStoreCode:
			return handleStoreCode(ctx, k, msg)
		case *MsgInstantiateContract:
			return handleInstantiate(ctx, k, msg)
		case *MsgExecuteContract:
			return handleExecute(ctx, k, msg)
		case *MsgMigrateContract:
			return handleMigrate(ctx, k, msg)
		case *MsgUpdateAdmin:
			return handleUpdateAdmin(ctx, k, msg)
		case *MsgClearAdmin:
			return handleClearAdmin(ctx, k, msg)
		default:
			errMsg := fmt.Sprintf("unrecognized wasm message type: %T", msg)
			return nil, sdkerrors.ErrUnknownRequest.Wrap(errMsg)
		}
	}
}

// filteredMessageEvents returns the same events with all of type == EventTypeMessage removed.
// this is so only our top-level message event comes through
func filteredMessageEvents(manager *sdk.EventManager) []abci.Event {
	events := manager.ABCIEvents()
	res := make([]abci.Event, 0, len(events)+1)
	for _, e := range events {
		if e.Type != sdk.EventTypeMessage {
			res = append(res, e)
		}
	}
	return res
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

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
			sdk.NewAttribute(types.AttributeKeySigner, msg.Sender.String()),
			sdk.NewAttribute(types.AttributeKeyCodeID, fmt.Sprintf("%d", codeID)),
		),
	})

	return &sdk.Result{
		Data:   []byte(fmt.Sprintf("%d", codeID)),
		Events: ctx.EventManager().ABCIEvents(),
	}, nil
}

func handleInstantiate(ctx sdk.Context, k Keeper, msg *MsgInstantiateContract) (*sdk.Result, error) {
	var adminAddr sdk.AccAddress
	var err error
	if msg.Admin != "" {
		if adminAddr, err = sdk.AccAddressFromBech32(msg.Admin); err != nil {
			return nil, errorsmod.Wrap(err, "admin")
		}
	}

	contractAddr, data, err := k.Instantiate(ctx, msg.CodeID, msg.Sender, adminAddr, msg.InitMsg, msg.Label, msg.InitFunds, msg.CallbackSig)
	if err != nil {
		result := sdk.Result{}
		result.Data = data
		return &result, err
	}

	events := filteredMessageEvents(ctx.EventManager().(*sdk.EventManager))
	custom := sdk.Events{sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeyCodeID, fmt.Sprintf("%d", msg.CodeID)),
		sdk.NewAttribute(types.AttributeKeyContractAddr, contractAddr.String()),
	)}
	events = append(events, custom.ToABCIEvents()...)

	// TODO Assaf:
	// also need to parse here output events and pass them to Tendermint
	// but k.Instantiate() doesn't return any output data right now, just contractAddr

	// Only for reply
	if data != nil {
		return &sdk.Result{
			Data:   data,
			Events: events,
		}, nil
	}

	return &sdk.Result{
		Data:   contractAddr,
		Events: events,
	}, nil
}

func handleExecute(ctx sdk.Context, k Keeper, msg *MsgExecuteContract) (*sdk.Result, error) {
	res, err := k.Execute(
		ctx,
		msg.Contract,
		msg.Sender,
		msg.Msg,
		msg.SentFunds,
		msg.CallbackSig,
		wasmtypes.HandleTypeExecute,
	)
	if err != nil {
		return res, err
	}

	events := filteredMessageEvents(ctx.EventManager().(*sdk.EventManager))
	custom := sdk.Events{sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeyContractAddr, msg.Contract.String()),
	)}
	events = append(events, custom.ToABCIEvents()...)

	res.Events = events

	return res, nil
}

func handleMigrate(ctx sdk.Context, k Keeper, msg *MsgMigrateContract) (*sdk.Result, error) {
	res, err := k.Migrate(
		ctx,
		sdk.MustAccAddressFromBech32(msg.Contract),
		sdk.MustAccAddressFromBech32(msg.Sender),
		msg.CodeID,
		msg.Msg,
		msg.CallbackSig,
	)
	if err != nil {
		return nil, err
	}

	events := filteredMessageEvents(ctx.EventManager().(*sdk.EventManager))
	custom := sdk.Events{sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender),
		sdk.NewAttribute(types.AttributeKeyCodeID, fmt.Sprintf("%d", msg.CodeID)),
		sdk.NewAttribute(types.AttributeKeyContractAddr, msg.Contract),
	)}
	events = append(events, custom.ToABCIEvents()...)

	return &sdk.Result{
		Data:   res,
		Events: events,
	}, nil
}

func handleUpdateAdmin(ctx sdk.Context, k Keeper, msg *MsgUpdateAdmin) (*sdk.Result, error) {
	err := k.UpdateContractAdmin(
		ctx,
		sdk.MustAccAddressFromBech32(msg.Contract),
		sdk.MustAccAddressFromBech32(msg.Sender),
		sdk.MustAccAddressFromBech32(msg.NewAdmin),
		msg.CallbackSig,
	)
	if err != nil {
		return nil, err
	}

	events := filteredMessageEvents(ctx.EventManager().(*sdk.EventManager))
	custom := sdk.Events{sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender),
		sdk.NewAttribute(types.AttributeKeyContractAddr, msg.Contract),
	)}
	events = append(events, custom.ToABCIEvents()...)

	return &sdk.Result{Events: events}, nil
}

func handleClearAdmin(ctx sdk.Context, k Keeper, msg *MsgClearAdmin) (*sdk.Result, error) {
	err := k.UpdateContractAdmin(
		ctx,
		sdk.MustAccAddressFromBech32(msg.Contract),
		sdk.MustAccAddressFromBech32(msg.Sender),
		nil,
		msg.CallbackSig,
	)
	if err != nil {
		return nil, err
	}

	events := filteredMessageEvents(ctx.EventManager().(*sdk.EventManager))
	custom := sdk.Events{sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(sdk.AttributeKeySender, msg.Sender),
		sdk.NewAttribute(types.AttributeKeyContractAddr, msg.Contract),
	)}
	events = append(events, custom.ToABCIEvents()...)

	return &sdk.Result{Events: events}, nil
}
