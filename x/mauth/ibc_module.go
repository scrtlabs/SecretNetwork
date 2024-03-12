package mauth

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	"github.com/scrtlabs/SecretNetwork/x/mauth/keeper"

	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	host "github.com/cosmos/ibc-go/v8/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
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
	_ channeltypes.Order,
	_ []string,
	portID string,
	channelID string,
	chanCap *capabilitytypes.Capability,
	_ channeltypes.Counterparty,
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
	_ sdk.Context,
	_ channeltypes.Order,
	_ []string,
	_,
	_ string,
	_ *capabilitytypes.Capability,
	_ channeltypes.Counterparty,
	_ string,
) (version string, err error) {
	return "", nil
}

// OnChanOpenAck implements the IBCModule interface
func (im IBCModule) OnChanOpenAck(
	_ sdk.Context,
	_,
	_ string,
	_ string,
	_ string,
) error {
	return nil
}

// OnChanOpenConfirm implements the IBCModule interface
func (im IBCModule) OnChanOpenConfirm(
	_ sdk.Context,
	_,
	_ string,
) error {
	return nil
}

// OnChanCloseInit implements the IBCModule interface
func (im IBCModule) OnChanCloseInit(
	_ sdk.Context,
	_,
	_ string,
) error {
	return nil
}

// OnChanCloseConfirm implements the IBCModule interface
func (im IBCModule) OnChanCloseConfirm(
	_ sdk.Context,
	_,
	_ string,
) error {
	return nil
}

// OnRecvPacket implements the IBCModule interface. A successful acknowledgement
// is returned if the packet data is successfully decoded and the receive application
// logic returns without error.
func (im IBCModule) OnRecvPacket(
	_ sdk.Context,
	_ channeltypes.Packet,
	_ sdk.AccAddress,
) ibcexported.Acknowledgement {
	return channeltypes.NewErrorAcknowledgement(sdkerrors.ErrInvalidRequest.Wrapf("cannot receive packet via interchain accounts authentication module"))
}

// OnAcknowledgementPacket implements the IBCModule interface
func (im IBCModule) OnAcknowledgementPacket(
	_ sdk.Context,
	_ channeltypes.Packet,
	_ []byte,
	_ sdk.AccAddress,
) error {
	return nil
}

// OnTimeoutPacket implements the IBCModule interface.
func (im IBCModule) OnTimeoutPacket(
	_ sdk.Context,
	_ channeltypes.Packet,
	_ sdk.AccAddress,
) error {
	return nil
}

// NegotiateAppVersion implements the IBCModule interface
func (im IBCModule) NegotiateAppVersion(
	_ sdk.Context,
	_ channeltypes.Order,
	_ string,
	_ string,
	_ channeltypes.Counterparty,
	_ string,
) (string, error) {
	return "", nil
}
