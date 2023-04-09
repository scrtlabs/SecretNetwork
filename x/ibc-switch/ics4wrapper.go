package ibc_switch

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	porttypes "github.com/cosmos/ibc-go/v4/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v4/modules/core/exported"

	// todo: change with custom types, from osmosis
	"github.com/osmosis-labs/osmosis/v15/x/ibc-rate-limit/types"
)

var (
	_ porttypes.Middleware  = &IBCModule{}
	_ porttypes.ICS4Wrapper = &ICS4Wrapper{}
)

type ICS4Wrapper struct {
	channel       porttypes.ICS4Wrapper
	accountKeeper *authkeeper.AccountKeeper
	paramSpace    paramtypes.Subspace
}

func (i *ICS4Wrapper) GetAppVersion(ctx sdk.Context, portID, channelID string) (string, bool) {
	return i.channel.GetAppVersion(ctx, portID, channelID)
}

func NewICS4Middleware(
	channel porttypes.ICS4Wrapper,
	accountKeeper *authkeeper.AccountKeeper,
	paramSpace paramtypes.Subspace,
) ICS4Wrapper {
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}
	return ICS4Wrapper{
		channel:       channel,
		accountKeeper: accountKeeper,
		paramSpace:    paramSpace,
	}
}

// SendPacket implements the ICS4 interface and is called when sending packets.
// This method blocks the sending of the packet if the ibc-switch is turned off.
// If the switcher param is not configured, packets are not blocked and handled by the wrapped IBC app
func (i *ICS4Wrapper) SendPacket(ctx sdk.Context, chanCap *capabilitytypes.Capability, packet exported.PacketI) error {
	status := i.GetSwitchStatus(ctx)

	// todo: make this an enum
	if status != "off" {
		// The status has not been configured. Continue as usual
		return i.channel.SendPacket(ctx, chanCap, packet)
	}

	// todo: define error
	return sdkerrors.Wrap(types.ErrIbcOff, "Ibc packets are currently paused in the network")
}

func (i *ICS4Wrapper) WriteAcknowledgement(ctx sdk.Context, chanCap *capabilitytypes.Capability, packet exported.PacketI, ack exported.Acknowledgement) error {
	return i.channel.WriteAcknowledgement(ctx, chanCap, packet, ack)
}

func (i *ICS4Wrapper) GetSwitchStatus(ctx sdk.Context) (contract string) {
	// todo: create param
	return i.GetParams(ctx).SwitchStatus
}

func (i *ICS4Wrapper) GetParams(ctx sdk.Context) (params types.Params) {
	// This was previously done via i.paramSpace.GetParamSet(ctx, &params). That will
	// panic if the params don't exist. This is a workaround to avoid that panic.
	// Params should be refactored to just use a raw kvstore.
	empty := types.Params{}
	for _, pair := range params.ParamSetPairs() {
		i.paramSpace.GetIfExists(ctx, pair.Key, pair.Value)
	}
	if params == empty {
		return types.DefaultParams()
	}
	return params
}

func (i *ICS4Wrapper) SetParams(ctx sdk.Context, params types.Params) {
	i.paramSpace.SetParamSet(ctx, &params)
}
