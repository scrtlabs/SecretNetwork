package ibc_switch

import (
	"context"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/ibc-switch/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	channelWrapper ChannelWrapper
}

func NewMsgServer(channelWrapper ChannelWrapper) types.MsgServer {
	return &msgServer{channelWrapper}
}

func (m msgServer) ToggleIbcSwitch(goCtx context.Context, msg *types.MsgToggleIbcSwitch) (*types.MsgToggleIbcSwitchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	pauser := m.channelWrapper.GetPauserAddress(ctx)
	if pauser == "" {
		return nil, sdkerrors.Wrap(types.ErrPauserUnset, "no address is currently approved to toggle ibc-switch")
	}

	if pauser != msg.GetSender() {
		return nil, sdkerrors.Wrap(types.ErrUnauthorizedToggle, "this address is not allowed to toggle ibc-switch")
	}

	status := m.channelWrapper.GetSwitchStatus(ctx)

	// todo enum?
	if status == "off" {
		m.channelWrapper.SetSwitchStatus(ctx, "on")
	} else {
		m.channelWrapper.SetSwitchStatus(ctx, "off")
	}

	// todo maybe emit event here?

	return &types.MsgToggleIbcSwitchResponse{}, nil
}
