package keeper

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

type Keeper struct {
	channel    porttypes.ICS4Wrapper
	paramSpace paramtypes.Subspace
}

func (i *Keeper) GetAppVersion(ctx sdk.Context, portID, channelID string) (string, bool) {
	return i.channel.GetAppVersion(ctx, portID, channelID)
}

func NewKeeper(
	channel porttypes.ICS4Wrapper,
	paramSpace paramtypes.Subspace,
) Keeper {
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		channel:    channel,
		paramSpace: paramSpace,
	}
}

// SendPacket implements the ICS4 interface and is called when sending packets.
// This method blocks the sending of the packet if the emergencybutton is turned off.
// If the switcher param is not configured, packets are not blocked and handled by the wrapped IBC app
func (i *Keeper) SendPacket(ctx sdk.Context, chanCap *capabilitytypes.Capability, packet exported.PacketI) (uint64, error) {
	status := i.GetSwitchStatus(ctx)

	if status == types.IbcSwitchStatusOff {
		println("Returning error!")
		return 0, errors.Wrap(types.ErrIbcOff, "Ibc packets are currently paused in the network")
	}

	return i.channel.SendPacket(ctx, chanCap, packet)
}

func (i *Keeper) WriteAcknowledgement(ctx sdk.Context, chanCap *capabilitytypes.Capability, packet exported.PacketI, ack exported.Acknowledgement) error {
	return i.channel.WriteAcknowledgement(ctx, chanCap, packet, ack)
}

func (i *Keeper) IsHalted(ctx sdk.Context) bool {
	return i.GetSwitchStatus(ctx) == types.IbcSwitchStatusOff
}
