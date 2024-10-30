package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
	ra "github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
)

const (
	AttributeSigner        = "signer"
	AttributeEncryptedSeed = "encrypted_seed"
	AttributeNodeID        = "node_id"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	keeper Keeper
	module string
}

func NewMsgServerImpl(k Keeper, m string) types.MsgServer {
	return &msgServer{keeper: k, module: m}
}

func (m msgServer) RegisterAuth(goCtx context.Context, msg *types.RaAuthenticate) (*types.RaAuthenticateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	err := msg.ValidateBasic()
	if err != nil {
		return nil, err
	}

	pubkey, err := ra.VerifyCombinedCert(msg.Certificate)
	if err != nil {
		return nil, err
	}

	encSeed, err := m.keeper.RegisterNode(ctx, msg.Certificate)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, m.module),
			sdk.NewAttribute(AttributeSigner, msg.Sender),
			sdk.NewAttribute(AttributeEncryptedSeed, fmt.Sprintf("0x%02x", encSeed)),
			sdk.NewAttribute(AttributeNodeID, fmt.Sprintf("0x%s", hex.EncodeToString(pubkey))),
		),
	})

	events, err := json.Marshal(ctx.EventManager().ABCIEvents())
	if err != nil {
		ctx.Logger().Error("Marshal ABCIEvents", "error", err.Error())
		return nil, err
	}

	return &types.RaAuthenticateResponse{
		Data:   fmt.Sprintf("S: %02x", encSeed),
		Events: string(events),
	}, nil
}
