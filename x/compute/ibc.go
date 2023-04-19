package compute

import (
	"math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	channeltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v4/modules/core/05-port/types"
	host "github.com/cosmos/ibc-go/v4/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v4/modules/core/exported"
	v1types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

var _ porttypes.IBCModule = IBCHandler{}

// internal interface that is implemented by ibc middleware
type appVersionGetter interface {
	// GetAppVersion returns the application level version with all middleware data stripped out
	GetAppVersion(ctx sdk.Context, portID, channelID string) (string, bool)
}

type IBCHandler struct {
	keeper           types.IBCContractKeeper
	channelKeeper    types.ChannelKeeper
	appVersionGetter appVersionGetter
}

func NewIBCHandler(k types.IBCContractKeeper, ck types.ChannelKeeper, vg appVersionGetter) IBCHandler {
	return IBCHandler{keeper: k, channelKeeper: ck, appVersionGetter: vg}
}

// OnChanOpenInit implements the IBCModule interface
func (i IBCHandler) OnChanOpenInit(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID string,
	channelID string,
	chanCap *capabilitytypes.Capability,
	counterParty channeltypes.Counterparty,
	version string,
) (string, error) {
	// ensure port, version, capability
	if err := ValidateChannelParams(channelID); err != nil {
		return "", err
	}
	contractAddr, err := ContractFromPortID(portID)
	if err != nil {
		return "", sdkerrors.Wrapf(err, "contract port id")
	}

	msg := v1types.IBCChannelOpenMsg{
		OpenInit: &v1types.IBCOpenInit{
			Channel: v1types.IBCChannel{
				Endpoint:             v1types.IBCEndpoint{PortID: portID, ChannelID: channelID},
				CounterpartyEndpoint: v1types.IBCEndpoint{PortID: counterParty.PortId, ChannelID: counterParty.ChannelId},
				Order:                order.String(),
				// DESIGN V3: this may be "" ??
				Version:      version,
				ConnectionID: connectionHops[0], // At the moment this list must be of length 1. In the future multi-hop channels may be supported.
			},
		},
	}
	_, err = i.keeper.OnOpenChannel(ctx, contractAddr, msg)
	if err != nil {
		return "", err
	}
	// Claim channel capability passed back by IBC module
	if err := i.keeper.ClaimCapability(ctx, chanCap, host.ChannelCapabilityPath(portID, channelID)); err != nil {
		return "", sdkerrors.Wrap(err, "claim capability")
	}
	return version, nil
}

// OnChanOpenTry implements the IBCModule interface
func (i IBCHandler) OnChanOpenTry(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID, channelID string,
	chanCap *capabilitytypes.Capability,
	counterParty channeltypes.Counterparty,
	counterpartyVersion string,
) (string, error) {
	// ensure port, version, capability
	if err := ValidateChannelParams(channelID); err != nil {
		return "", err
	}

	contractAddr, err := ContractFromPortID(portID)
	if err != nil {
		return "", sdkerrors.Wrapf(err, "contract port id")
	}

	msg := v1types.IBCChannelOpenMsg{
		OpenTry: &v1types.IBCOpenTry{
			Channel: v1types.IBCChannel{
				Endpoint:             v1types.IBCEndpoint{PortID: portID, ChannelID: channelID},
				CounterpartyEndpoint: v1types.IBCEndpoint{PortID: counterParty.PortId, ChannelID: counterParty.ChannelId},
				Order:                order.String(),
				Version:              counterpartyVersion,
				ConnectionID:         connectionHops[0], // At the moment this list must be of length 1. In the future multi-hop channels may be supported.
			},
			CounterpartyVersion: counterpartyVersion,
		},
	}

	// Allow contracts to return a version (or default to counterpartyVersion if unset)
	version, err := i.keeper.OnOpenChannel(ctx, contractAddr, msg)
	if err != nil {
		return "", err
	}
	if version == "" {
		version = counterpartyVersion
	}

	// Module may have already claimed capability in OnChanOpenInit in the case of crossing hellos
	// (ie chainA and chainB both call ChanOpenInit before one of them calls ChanOpenTry)
	// If module can already authenticate the capability then module already owns it so we don't need to claim
	// Otherwise, module does not have channel capability and we must claim it from IBC
	if !i.keeper.AuthenticateCapability(ctx, chanCap, host.ChannelCapabilityPath(portID, channelID)) {
		// Only claim channel capability passed back by IBC module if we do not already own it
		if err := i.keeper.ClaimCapability(ctx, chanCap, host.ChannelCapabilityPath(portID, channelID)); err != nil {
			return "", sdkerrors.Wrap(err, "claim capability")
		}
	}

	return version, nil
}

// OnChanOpenAck implements the IBCModule interface
func (i IBCHandler) OnChanOpenAck(
	ctx sdk.Context,
	portID, channelID string,
	counterpartyChannelID string,
	counterpartyVersion string,
) error {
	contractAddr, err := ContractFromPortID(portID)
	if err != nil {
		return sdkerrors.Wrapf(err, "contract port id")
	}
	channelInfo, ok := i.channelKeeper.GetChannel(ctx, portID, channelID)
	if !ok {
		return sdkerrors.Wrapf(channeltypes.ErrChannelNotFound, "port ID (%s) channel ID (%s)", portID, channelID)
	}
	channelInfo.Counterparty.ChannelId = counterpartyChannelID

	appVersion, ok := i.appVersionGetter.GetAppVersion(ctx, portID, channelID)
	if !ok {
		return sdkerrors.Wrapf(channeltypes.ErrInvalidChannelVersion, "port ID (%s) channel ID (%s)", portID, channelID)
	}

	msg := v1types.IBCChannelConnectMsg{
		OpenAck: &v1types.IBCOpenAck{
			Channel:             toWasmVMChannel(portID, channelID, channelInfo, appVersion),
			CounterpartyVersion: counterpartyVersion,
		},
	}
	return i.keeper.OnConnectChannel(ctx, contractAddr, msg)
}

// OnChanOpenConfirm implements the IBCModule interface
func (i IBCHandler) OnChanOpenConfirm(ctx sdk.Context, portID, channelID string) error {
	contractAddr, err := ContractFromPortID(portID)
	if err != nil {
		return sdkerrors.Wrapf(err, "contract port id")
	}
	channelInfo, ok := i.channelKeeper.GetChannel(ctx, portID, channelID)
	if !ok {
		return sdkerrors.Wrapf(channeltypes.ErrChannelNotFound, "port ID (%s) channel ID (%s)", portID, channelID)
	}
	appVersion, ok := i.appVersionGetter.GetAppVersion(ctx, portID, channelID)
	if !ok {
		return sdkerrors.Wrapf(channeltypes.ErrInvalidChannelVersion, "port ID (%s) channel ID (%s)", portID, channelID)
	}
	msg := v1types.IBCChannelConnectMsg{
		OpenConfirm: &v1types.IBCOpenConfirm{
			Channel: toWasmVMChannel(portID, channelID, channelInfo, appVersion)},
	}
	return i.keeper.OnConnectChannel(ctx, contractAddr, msg)
}

// OnChanCloseInit implements the IBCModule interface
func (i IBCHandler) OnChanCloseInit(ctx sdk.Context, portID, channelID string) error {
	contractAddr, err := ContractFromPortID(portID)
	if err != nil {
		return sdkerrors.Wrapf(err, "contract port id")
	}
	channelInfo, ok := i.channelKeeper.GetChannel(ctx, portID, channelID)
	if !ok {
		return sdkerrors.Wrapf(channeltypes.ErrChannelNotFound, "port ID (%s) channel ID (%s)", portID, channelID)
	}
	appVersion, ok := i.appVersionGetter.GetAppVersion(ctx, portID, channelID)
	if !ok {
		return sdkerrors.Wrapf(channeltypes.ErrInvalidChannelVersion, "port ID (%s) channel ID (%s)", portID, channelID)
	}
	msg := v1types.IBCChannelCloseMsg{
		CloseInit: &v1types.IBCCloseInit{Channel: toWasmVMChannel(portID, channelID, channelInfo, appVersion)},
	}
	err = i.keeper.OnCloseChannel(ctx, contractAddr, msg)
	if err != nil {
		return err
	}
	// emit events?

	return err
}

// OnChanCloseConfirm implements the IBCModule interface
func (i IBCHandler) OnChanCloseConfirm(ctx sdk.Context, portID, channelID string) error {
	// counterparty has closed the channel
	contractAddr, err := ContractFromPortID(portID)
	if err != nil {
		return sdkerrors.Wrapf(err, "contract port id")
	}
	channelInfo, ok := i.channelKeeper.GetChannel(ctx, portID, channelID)
	if !ok {
		return sdkerrors.Wrapf(channeltypes.ErrChannelNotFound, "port ID (%s) channel ID (%s)", portID, channelID)
	}
	appVersion, ok := i.appVersionGetter.GetAppVersion(ctx, portID, channelID)
	if !ok {
		return sdkerrors.Wrapf(channeltypes.ErrInvalidChannelVersion, "port ID (%s) channel ID (%s)", portID, channelID)
	}
	msg := v1types.IBCChannelCloseMsg{
		CloseConfirm: &v1types.IBCCloseConfirm{Channel: toWasmVMChannel(portID, channelID, channelInfo, appVersion)},
	}
	err = i.keeper.OnCloseChannel(ctx, contractAddr, msg)
	if err != nil {
		return err
	}
	// emit events?

	return err
}

func toWasmVMChannel(portID, channelID string, channelInfo channeltypes.Channel, appVersion string) v1types.IBCChannel {
	return v1types.IBCChannel{
		Endpoint:             v1types.IBCEndpoint{PortID: portID, ChannelID: channelID},
		CounterpartyEndpoint: v1types.IBCEndpoint{PortID: channelInfo.Counterparty.PortId, ChannelID: channelInfo.Counterparty.ChannelId},
		Order:                channelInfo.Ordering.String(),
		Version:              appVersion,
		ConnectionID:         channelInfo.ConnectionHops[0], // At the moment this list must be of length 1. In the future multi-hop channels may be supported.
	}
}

// OnRecvPacket implements the IBCModule interface
func (i IBCHandler) OnRecvPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) ibcexported.Acknowledgement {
	contractAddr, err := ContractFromPortID(packet.DestinationPort)
	if err != nil {
		return channeltypes.NewErrorAcknowledgement(sdkerrors.Wrapf(err, "contract port id"))
	}
	msg := v1types.IBCPacketReceiveMsg{Packet: newIBCPacket(packet), Relayer: relayer.String()}
	ack, err := i.keeper.OnRecvPacket(ctx, contractAddr, msg)
	if err != nil {
		return channeltypes.NewErrorAcknowledgement(sdkerrors.Wrapf(err, "on recv packet"))
	}
	return ContractConfirmStateAck(ack)
}

var _ ibcexported.Acknowledgement = ContractConfirmStateAck{}

type ContractConfirmStateAck []byte

func (w ContractConfirmStateAck) Success() bool {
	return true // always commit state
}

func (w ContractConfirmStateAck) Acknowledgement() []byte {
	return w
}

// OnAcknowledgementPacket implements the IBCModule interface
func (i IBCHandler) OnAcknowledgementPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
) error {
	contractAddr, err := ContractFromPortID(packet.SourcePort)
	if err != nil {
		return sdkerrors.Wrapf(err, "contract port id")
	}

	err = i.keeper.OnAckPacket(ctx, contractAddr, v1types.IBCPacketAckMsg{
		Acknowledgement: v1types.IBCAcknowledgement{Data: acknowledgement},
		OriginalPacket:  newIBCPacket(packet),
		Relayer:         relayer.String(),
	})
	if err != nil {
		return sdkerrors.Wrap(err, "on ack")
	}
	return nil
}

// OnTimeoutPacket implements the IBCModule interface
func (i IBCHandler) OnTimeoutPacket(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) error {
	contractAddr, err := ContractFromPortID(packet.SourcePort)
	if err != nil {
		return sdkerrors.Wrapf(err, "contract port id")
	}
	msg := v1types.IBCPacketTimeoutMsg{Packet: newIBCPacket(packet), Relayer: relayer.String()}
	err = i.keeper.OnTimeoutPacket(ctx, contractAddr, msg)
	if err != nil {
		return sdkerrors.Wrap(err, "on timeout")
	}
	return nil
}

func newIBCPacket(packet channeltypes.Packet) v1types.IBCPacket {
	timeout := v1types.IBCTimeout{
		Timestamp: packet.TimeoutTimestamp,
	}
	if !packet.TimeoutHeight.IsZero() {
		timeout.Block = &v1types.IBCTimeoutBlock{
			Height:   packet.TimeoutHeight.RevisionHeight,
			Revision: packet.TimeoutHeight.RevisionNumber,
		}
	}

	return v1types.IBCPacket{
		Data:     packet.Data,
		Src:      v1types.IBCEndpoint{ChannelID: packet.SourceChannel, PortID: packet.SourcePort},
		Dest:     v1types.IBCEndpoint{ChannelID: packet.DestinationChannel, PortID: packet.DestinationPort},
		Sequence: packet.Sequence,
		Timeout:  timeout,
	}
}

func ValidateChannelParams(channelID string) error {
	// NOTE: for escrow address security only 2^32 channels are allowed to be created
	// Issue: https://github.com/cosmos/cosmos-sdk/issues/7737
	channelSequence, err := channeltypes.ParseChannelSequence(channelID)
	if err != nil {
		return err
	}
	if channelSequence > math.MaxUint32 {
		return sdkerrors.Wrapf(types.ErrMaxIBCChannels, "channel sequence %d is greater than max allowed transfer channels %d", channelSequence, math.MaxUint32)
	}
	return nil
}
