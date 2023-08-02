package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	channeltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v4/modules/core/exported"
)

// ICS20TransferPortSource is a subset of the ibc transfer keeper.
type ICS20TransferPortSource interface {
	GetPort(ctx sdk.Context) string
}

// ChannelKeeper defines the expected IBC channel keeper
type ChannelKeeper interface {
	GetChannel(ctx sdk.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool)
	GetNextSequenceSend(ctx sdk.Context, portID, channelID string) (uint64, bool)
	SendPacket(ctx sdk.Context, channelCap *capabilitytypes.Capability, packet ibcexported.PacketI) error
	ChanCloseInit(ctx sdk.Context, portID, channelID string, chanCap *capabilitytypes.Capability) error
	GetAllChannels(ctx sdk.Context) (channels []channeltypes.IdentifiedChannel)
	IterateChannels(ctx sdk.Context, cb func(channeltypes.IdentifiedChannel) bool)
	SetChannel(ctx sdk.Context, portID, channelID string, channel channeltypes.Channel)
}
