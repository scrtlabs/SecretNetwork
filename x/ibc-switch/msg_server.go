package ibc_switch

import (
	"context"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/ibc-switch/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	ics4wrapper ICS4Wrapper
}

func NewMsgServer(ics4wrapper ICS4Wrapper) types.MsgServer {
	return &msgServer{ics4wrapper}
}

func (m msgServer) ToggleSwitch(goCtx context.Context, msg *types.MsgToggleSwitch) (*types.MsgToggleSwitchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	pauser := m.ics4wrapper.GetPauserAddress(ctx)
	pauserAddress, err := sdk.AccAddressFromBech32(pauser)
	if err != nil {
		return nil, err
	}

	if !pauserAddress.Equals(msg.GetSender()) {
		return nil, sdkerrors.Wrap(types.ErrUnauthorizedToggle, "This address is not allowed to toggle ibc-switch")
	}

	status := m.ics4wrapper.GetSwitchStatus(ctx)

	// todo enum?
	if status == "off" {
		m.ics4wrapper.SetSwitchStatus(ctx, "on")
	} else {
		m.ics4wrapper.SetSwitchStatus(ctx, "off")
	}

	// todo maybe emit event here?

	return &types.MsgToggleSwitchResponse{}, nil
}
