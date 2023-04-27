package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	porttypes "github.com/cosmos/ibc-go/v4/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v4/modules/core/exported"
	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

type Keeper struct {
	channel       porttypes.ICS4Wrapper
	accountKeeper *authkeeper.AccountKeeper
	paramSpace    paramtypes.Subspace
}

func (i *Keeper) GetAppVersion(ctx sdk.Context, portID, channelID string) (string, bool) {
	return i.channel.GetAppVersion(ctx, portID, channelID)
}

func NewKeeper(
	channel porttypes.ICS4Wrapper,
	accountKeeper *authkeeper.AccountKeeper,
	paramSpace paramtypes.Subspace,
) Keeper {

	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		channel:       channel,
		accountKeeper: accountKeeper,
		paramSpace:    paramSpace,
	}
}

// SendPacket implements the ICS4 interface and is called when sending packets.
// This method blocks the sending of the packet if the emergencybutton is turned off.
// If the switcher param is not configured, packets are not blocked and handled by the wrapped IBC app
func (i *Keeper) SendPacket(ctx sdk.Context, chanCap *capabilitytypes.Capability, packet exported.PacketI) error {
	status := i.GetSwitchStatus(ctx)

	if status == types.IbcSwitchStatusOff {
		return sdkerrors.Wrap(types.ErrIbcOff, "Ibc packets are currently paused in the network")
	}

	return i.channel.SendPacket(ctx, chanCap, packet)
}

func (i *Keeper) WriteAcknowledgement(ctx sdk.Context, chanCap *capabilitytypes.Capability, packet exported.PacketI, ack exported.Acknowledgement) error {
	return i.channel.WriteAcknowledgement(ctx, chanCap, packet, ack)
}
