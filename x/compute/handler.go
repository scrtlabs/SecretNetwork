package compute

import (
	"fmt"
	abci "github.com/tendermint/tendermint/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

// NewHandler returns a handler for "bank" type messages.
func NewHandler(k Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {

		case *MsgStoreCode:
			return handleStoreCode(ctx, k, msg)
		case *MsgInstantiateContract:
			return handleInstantiate(ctx, k, msg)
		case *MsgExecuteContract:
			return handleExecute(ctx, k, msg)
			/*
				case MsgMigrateContract:
					return handleMigration(ctx, k, &msg)
				case MsgUpdateAdmin:
					return handleUpdateContractAdmin(ctx, k, &msg)
				case MsgClearAdmin:
					return handleClearContractAdmin(ctx, k, &msg)
			*/
		default:
			errMsg := fmt.Sprintf("unrecognized wasm message type: %T", msg)
			return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, errMsg)
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
	contractAddr, err := k.Instantiate(ctx, msg.CodeID, msg.Sender, msg.InitMsg, msg.Label, msg.InitFunds, msg.CallbackSig)
	if err != nil {
		return nil, err
	}

	events := filteredMessageEvents(ctx.EventManager())
	custom := sdk.Events{sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(types.AttributeKeySigner, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeyCodeID, fmt.Sprintf("%d", msg.CodeID)),
		sdk.NewAttribute(types.AttributeKeyContract, contractAddr.String()),
	)}
	events = append(events, custom.ToABCIEvents()...)

	// TODO Assaf:
	// also need to parse here output events and pass them to Tendermint
	// but k.Instantiate() doesn't return any output data right now, just contractAddr

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
	)
	if err != nil {
		return nil, err
	}

	events := filteredMessageEvents(ctx.EventManager())
	custom := sdk.Events{sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(types.AttributeKeySigner, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeyContract, msg.Contract.String()),
	)}
	events = append(events, custom.ToABCIEvents()...)

	res.Events = events

	return res, nil
}

/*
func handleMigration(ctx sdk.Context, k Keeper, msg *MsgMigrateContract) (*sdk.Result, error) {
	res, err := k.Migrate(ctx, msg.Contract, msg.Sender, msg.CodeID, msg.MigrateMsg) // for MsgMigrateContract, there is only one signer which is msg.Sender (https://github.com/enigmampc/SecretNetwork/blob/d7813792fa07b93a10f0885eaa4c5e0a0a698854/x/compute/internal/types/msg.go#L228-L230)
	if err != nil {
		return nil, err
	}

	events := filteredMessageEvents(ctx.EventManager())
	ourEvent := sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(types.AttributeKeySigner, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeyContract, msg.Contract.String()),
	)
	res.Events = append(events, ourEvent)
	return res, nil
}

func handleUpdateContractAdmin(ctx sdk.Context, k Keeper, msg *MsgUpdateAdmin) (*sdk.Result, error) {
	if err := k.UpdateContractAdmin(ctx, msg.Contract, msg.Sender, msg.NewAdmin); err != nil {
		return nil, err
	}
	events := ctx.EventManager().Events()
	ourEvent := sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(types.AttributeKeySigner, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeyContract, msg.Contract.String()),
	)
	return &sdk.Result{
		Events: append(events, ourEvent),
	}, nil
}

func handleClearContractAdmin(ctx sdk.Context, k Keeper, msg *MsgClearAdmin) (*sdk.Result, error) {
	if err := k.ClearContractAdmin(ctx, msg.Contract, msg.Sender); err != nil {
		return nil, err
	}
	events := ctx.EventManager().Events()
	ourEvent := sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
		sdk.NewAttribute(types.AttributeKeySigner, msg.Sender.String()),
		sdk.NewAttribute(types.AttributeKeyContract, msg.Contract.String()),
	)
	return &sdk.Result{
		Events: append(events, ourEvent),
	}, nil
}
*/
