package ibc_switch

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	channeltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v4/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v4/modules/core/exported"
)

type IBCModule struct {
	app            porttypes.IBCModule
	ics4Middleware *ICS4Wrapper
}

func NewIBCModule(app porttypes.IBCModule, ics4 *ICS4Wrapper) IBCModule {
	return IBCModule{
		app:            app,
		ics4Middleware: ics4,
	}
}

// OnChanOpenInit implements the IBCModule interface
func (im *IBCModule) OnChanOpenInit(ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID string,
	channelID string,
	channelCap *capabilitytypes.Capability,
	counterparty channeltypes.Counterparty,
	version string,
) (string, error) {
	// Do nothing, channels can still be opened even when the switch is off
	return im.app.OnChanOpenInit(
		ctx,
		order,
		connectionHops,
		portID,
		channelID,
		channelCap,
		counterparty,
		version,
	)
}

// OnChanOpenTry implements the IBCModule interface
func (im *IBCModule) OnChanOpenTry(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID,
	channelID string,
	channelCap *capabilitytypes.Capability,
	counterparty channeltypes.Counterparty,
	counterpartyVersion string,
) (string, error) {
	// Do nothing, channels can still be opened even when the switch is off
	return im.app.OnChanOpenTry(ctx, order, connectionHops, portID, channelID, channelCap, counterparty, counterpartyVersion)
}

// OnChanOpenAck implements the IBCModule interface
func (im *IBCModule) OnChanOpenAck(
	ctx sdk.Context,
	portID,
	channelID string,
	counterpartyChannelID string,
	counterpartyVersion string,
) error {
	// Do nothing, channels can still be opened even when the switch is off
	return im.app.OnChanOpenAck(ctx, portID, channelID, counterpartyChannelID, counterpartyVersion)
}

// OnChanOpenConfirm implements the IBCModule interface
func (im *IBCModule) OnChanOpenConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// Do nothing, channels can still be opened even when the switch is off
	return im.app.OnChanOpenConfirm(ctx, portID, channelID)
}

// OnChanCloseInit implements the IBCModule interface
func (im *IBCModule) OnChanCloseInit(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// Do nothing, channels can still be closed even when the switch is off
	return im.app.OnChanCloseInit(ctx, portID, channelID)
}

// OnChanCloseConfirm implements the IBCModule interface
func (im *IBCModule) OnChanCloseConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// Do nothing, channels can still be closed even when the switch is off
	return im.app.OnChanCloseConfirm(ctx, portID, channelID)
}

// OnRecvPacket implements the IBCModule interface
func (im *IBCModule) OnRecvPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) exported.Acknowledgement {
	// todo: check if the switch is off, in which case packet is not forwarded to underlying application
	fmt.Println("RecvPacket on switch middleware: passing packet")

	return im.app.OnRecvPacket(ctx, packet, relayer)
}

// OnAcknowledgementPacket implements the IBCModule interface
func (im *IBCModule) OnAcknowledgementPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
) error {
	// todo: decide if we do want to disable acks too when the switch is off
	//  (we may want to let them in, e.g. to revert transfers):

	//var ack channeltypes.Acknowledgement
	//if err := json.Unmarshal(acknowledgement, &ack); err != nil {
	//	return sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "cannot unmarshal ICS-20 transfer packet acknowledgement: %v", err)
	//}

	//var IsAckError = func (acknowledgement []byte) bool {
	//	var ackErr channeltypes.Acknowledgement_Error
	//		if err := json.Unmarshal(acknowledgement, &ackErr); err == nil && len(ackErr.Error) > 0 {
	//		return true
	//	}
	//	return false
	//}

	//if IsAckError(acknowledgement) {
	//	err := im.RevertSentPacket(ctx, packet) // If there is an error here we should still handle the ack
	//	if err != nil {
	//		ctx.EventManager().EmitEvent(
	//			sdk.NewEvent(
	//				types.EventBadRevert,
	//				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
	//				sdk.NewAttribute(types.AttributeKeyFailureType, "acknowledgment"),
	//				sdk.NewAttribute(types.AttributeKeyPacket, string(packet.GetData())),
	//				sdk.NewAttribute(types.AttributeKeyAck, string(acknowledgement)),
	//			),
	//		)
	//	}
	//}

	return im.app.OnAcknowledgementPacket(ctx, packet, acknowledgement, relayer)
}

// OnTimeoutPacket implements the IBCModule interface
func (im *IBCModule) OnTimeoutPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) error {
	// todo: decide if we do want to disable timeouts too when the switch is off
	//  (we may want to let them in, e.g. to revert transfers):

	//err := im.RevertSentPacket(ctx, packet) // If there is an error here we should still handle the timeout
	//if err != nil {
	//	ctx.EventManager().EmitEvent(
	//		sdk.NewEvent(
	//			types.EventBadRevert,
	//			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
	//			sdk.NewAttribute(types.AttributeKeyFailureType, "timeout"),
	//			sdk.NewAttribute(types.AttributeKeyPacket, string(packet.GetData())),
	//		),
	//	)
	//}

	return im.app.OnTimeoutPacket(ctx, packet, relayer)
}

// SendPacket implements the ICS4 Wrapper interface. In case the switch is off, the SendPacket method of the
// ics4Middleware should block it
func (im *IBCModule) SendPacket(
	ctx sdk.Context,
	chanCap *capabilitytypes.Capability,
	packet exported.PacketI,
) error {
	return im.ics4Middleware.SendPacket(ctx, chanCap, packet)
}

// WriteAcknowledgement implements the ICS4 Wrapper interface
func (im *IBCModule) WriteAcknowledgement(
	ctx sdk.Context,
	chanCap *capabilitytypes.Capability,
	packet exported.PacketI,
	ack exported.Acknowledgement,
) error {
	return im.ics4Middleware.WriteAcknowledgement(ctx, chanCap, packet, ack)
}

func (im *IBCModule) GetAppVersion(ctx sdk.Context, portID, channelID string) (string, bool) {
	return im.ics4Middleware.GetAppVersion(ctx, portID, channelID)
}
