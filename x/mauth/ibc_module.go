package mauth

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/scrtlabs/SecretNetwork/x/mauth/keeper"

	channeltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v4/modules/core/05-port/types"
	host "github.com/cosmos/ibc-go/v4/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v4/modules/core/exported"
)

var _ porttypes.IBCModule = IBCModule{}

// IBCModule implements the ICS26 interface for interchain accounts controller chains
type IBCModule struct {
	keeper keeper.Keeper
}

// NewIBCModule creates a new IBCModule given the keeper
func NewIBCModule(k keeper.Keeper) IBCModule {
	return IBCModule{
		keeper: k,
	}
}

// OnChanOpenInit implements the IBCModule interface
func (im IBCModule) OnChanOpenInit(
	ctx sdk.Context,
	order channeltypes.Order, //nolint:all
	connectionHops []string, //nolint:all
	portID string,
	channelID string,
	chanCap *capabilitytypes.Capability,
	counterparty channeltypes.Counterparty, //nolint:all
	version string,
) (string, error) {
	err := im.keeper.ClaimCapability(ctx, chanCap, host.ChannelCapabilityPath(portID, channelID))
	if err != nil {
		return "", err
	}

	return version, nil
}

// OnChanOpenTry implements the IBCModule interface
func (im IBCModule) OnChanOpenTry(
	ctx sdk.Context, //nolint:all
	order channeltypes.Order, //nolint:all
	connectionHops []string, //nolint:all
	portID, //nolint:all
	channelID string, //nolint:all
	chanCap *capabilitytypes.Capability, //nolint:all
	counterparty channeltypes.Counterparty, //nolint:all
	counterpartyVersion string, //nolint:all
) (version string, err error) {
	return "", nil
}

// OnChanOpenAck implements the IBCModule interface
func (im IBCModule) OnChanOpenAck(
	ctx sdk.Context, //nolint:all
	portID, //nolint:all
	channelID string, //nolint:all
	counterpartychannelID string, //nolint:all
	counterpartyVersion string, //nolint:all
) error {
	return nil
}

// OnChanOpenConfirm implements the IBCModule interface
func (im IBCModule) OnChanOpenConfirm(
	ctx sdk.Context, //nolint:all
	portID, //nolint:all
	channelID string, //nolint:all
) error {
	return nil
}

// OnChanCloseInit implements the IBCModule interface
func (im IBCModule) OnChanCloseInit(
	ctx sdk.Context, //nolint:all
	portID, //nolint:all
	channelID string, //nolint:all
) error {
	return nil
}

// OnChanCloseConfirm implements the IBCModule interface
func (im IBCModule) OnChanCloseConfirm(
	ctx sdk.Context, //nolint:all
	portID, //nolint:all
	channelID string, //nolint:all
) error {
	return nil
}

// OnRecvPacket implements the IBCModule interface. A successful acknowledgement
// is returned if the packet data is successfully decoded and the receive application
// logic returns without error.
func (im IBCModule) OnRecvPacket(
	ctx sdk.Context, //nolint:all
	packet channeltypes.Packet, //nolint:all
	relayer sdk.AccAddress, //nolint:all
) ibcexported.Acknowledgement {
	return channeltypes.NewErrorAcknowledgement(sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "cannot receive packet via interchain accounts authentication module"))
}

// OnAcknowledgementPacket implements the IBCModule interface
func (im IBCModule) OnAcknowledgementPacket(
	ctx sdk.Context, //nolint:all
	packet channeltypes.Packet, //nolint:all
	acknowledgement []byte, //nolint:all
	relayer sdk.AccAddress, //nolint:all
) error {
	return nil
}

// OnTimeoutPacket implements the IBCModule interface.
func (im IBCModule) OnTimeoutPacket(
	ctx sdk.Context, //nolint:all
	packet channeltypes.Packet, //nolint:all
	relayer sdk.AccAddress, //nolint:all
) error {
	return nil
}

// NegotiateAppVersion implements the IBCModule interface
func (im IBCModule) NegotiateAppVersion(
	ctx sdk.Context, //nolint:all
	order channeltypes.Order, //nolint:all
	connectionID string, //nolint:all
	portID string, //nolint:all
	counterparty channeltypes.Counterparty, //nolint:all
	proposedVersion string, //nolint:all
) (string, error) {
	return "", nil
}
