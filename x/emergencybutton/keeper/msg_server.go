package keeper

import (
	"context"

	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
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
		return nil, errors.Wrap(types.ErrPauserUnset, "no address is currently approved to toggle emergency button")
	}

	if pauser != msg.GetSender() {
		return nil, errors.Wrap(types.ErrUnauthorizedToggle, "this address is not allowed to toggle emergency button")
	}

	status := m.keeper.GetSwitchStatus(ctx)

	if status == types.IbcSwitchStatusOff {
		m.keeper.SetSwitchStatus(ctx, types.IbcSwitchStatusOn)
	} else {
		m.keeper.SetSwitchStatus(ctx, types.IbcSwitchStatusOff)
	}

	// todo maybe emit event here?

	return &types.MsgToggleIbcSwitchResponse{}, nil
}

func (m msgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if m.keeper.authority != req.Authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", m.keeper.authority, req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := m.keeper.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
