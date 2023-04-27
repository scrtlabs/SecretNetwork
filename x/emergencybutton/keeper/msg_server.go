package keeper

import (
	"context"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	keeper Keeper
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{keeper}
}

func (m msgServer) ToggleIbcSwitch(goCtx context.Context, msg *types.MsgToggleIbcSwitch) (*types.MsgToggleIbcSwitchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	pauser := m.keeper.GetPauserAddress(ctx)
	if pauser == "" {
		return nil, sdkerrors.Wrap(types.ErrPauserUnset, "no address is currently approved to toggle emergencybutton")
	}

	if pauser != msg.GetSender() {
		return nil, sdkerrors.Wrap(types.ErrUnauthorizedToggle, "this address is not allowed to toggle emergencybutton")
	}

	status := m.keeper.GetSwitchStatus(ctx)

	// todo enum?
	if status == "off" {
		m.keeper.SetSwitchStatus(ctx, "on")
	} else {
		m.keeper.SetSwitchStatus(ctx, "off")
	}

	// todo maybe emit event here?

	return &types.MsgToggleIbcSwitchResponse{}, nil
}
