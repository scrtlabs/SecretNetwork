package registration

import (
	"encoding/hex"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
	ra "github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
)

const (
	AttributeSigner        = "signer"
	AttributeEncryptedSeed = "encrypted_seed"
	AttributeNodeID        = "node_id"
)

// NewHandler returns a handler for "bank" type messages.
func NewHandler(k Keeper) baseapp.MsgServiceHandler {
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())

		switch msg := msg.(type) {

		case *MsgRaAuthenticate:
			return handleRaAuthenticate(ctx, k, msg)

		default:
			errMsg := fmt.Sprintf("unrecognized wasm message type: %T", msg)
			return nil, sdkerrors.ErrUnknownRequest.Wrap(errMsg)
		}
	}
}

// filterMessageEvents returns the same events with all of type == EventTypeMessage removed.
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

func handleRaAuthenticate(ctx sdk.Context, k Keeper, msg *types.RaAuthenticate) (*sdk.Result, error) {
	err := msg.ValidateBasic()
	if err != nil {
		return nil, err
	}

	pubkey, err := ra.VerifyCombinedCert(msg.Certificate)
	if err != nil {
		return nil, err
	}

	encSeed, err := k.RegisterNode(ctx, msg.Certificate)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, ModuleName),
			sdk.NewAttribute(AttributeSigner, msg.Sender.String()),
			sdk.NewAttribute(AttributeEncryptedSeed, fmt.Sprintf("0x%02x", encSeed)),
			sdk.NewAttribute(AttributeNodeID, fmt.Sprintf("0x%s", hex.EncodeToString(pubkey))),
		),
	})

	return &sdk.Result{
		Data:   []byte(fmt.Sprintf("S: %02x", encSeed)),
		Events: ctx.EventManager().ABCIEvents(),
	}, nil
}
