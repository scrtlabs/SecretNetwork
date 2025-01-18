package keeper

import (
	"cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	codec "github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	"github.com/scrtlabs/SecretNetwork/x/emergencybutton/types"
)

type Keeper struct {
	cdc       codec.BinaryCodec
	channel   porttypes.ICS4Wrapper
	storeKey  storetypes.StoreKey
	authority string
}

func (i *Keeper) GetAppVersion(ctx sdk.Context, portID, channelID string) (string, bool) {
	return i.channel.GetAppVersion(ctx, portID, channelID)
}

func NewKeeper(
	channel porttypes.ICS4Wrapper,
	cdc codec.BinaryCodec,
	key storetypes.StoreKey,
	authority string,
) Keeper {
	return Keeper{
		channel:   channel,
		authority: authority,
		cdc:       cdc,
		storeKey:  key,
	}
}

// SendPacket implements the ICS4 interface and is called when sending packets.
// This method blocks the sending of the packet if the emergencybutton is turned off.
// If the switcher param is not configured, packets are not blocked and handled by the wrapped IBC app
func (i *Keeper) SendPacket(ctx sdk.Context, chanCap *capabilitytypes.Capability, sourcePort string, sourceChannel string, timeoutHeight ibcclienttypes.Height, timeoutTimestamp uint64, data []byte) (uint64, error) {
	status := i.GetSwitchStatus(ctx)

	if status == types.IbcSwitchStatusOff {
		println("Returning error!")
		return 0, errors.Wrap(types.ErrIbcOff, "Ibc packets are currently paused in the network")
	}

	return i.channel.SendPacket(ctx, chanCap, sourcePort, sourceChannel, timeoutHeight, timeoutTimestamp, data)
}

func (i *Keeper) WriteAcknowledgement(ctx sdk.Context, chanCap *capabilitytypes.Capability, packet exported.PacketI, ack exported.Acknowledgement) error {
	return i.channel.WriteAcknowledgement(ctx, chanCap, packet, ack)
}

func (i *Keeper) IsHalted(ctx sdk.Context) bool {
	return i.GetSwitchStatus(ctx) == types.IbcSwitchStatusOff
}

// GetAuthority returns the x/emergencybutton module's authority.
func (i Keeper) GetAuthority() string {
	return i.authority
}
