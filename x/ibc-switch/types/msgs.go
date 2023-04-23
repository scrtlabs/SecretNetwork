package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	TypeMsgToggleSwitch = "toggle_switch"
)

var _ sdk.Msg = &MsgToggleSwitch{}

// NewMsgToggleSwitch creates a message to toggle switch
func NewMsgToggleSwitch(sender sdk.AccAddress) *MsgToggleSwitch {
	return &MsgToggleSwitch{sender}
}

// Route takes a toggle switch message, then returns the RouterKey used for slashing.
func (m MsgToggleSwitch) Route() string { return RouterKey }

// Type takes a toggle switch message, then returns a toggle switch message type.
func (m MsgToggleSwitch) Type() string { return TypeMsgToggleSwitch }

// ValidateBasic checks that the toggle switch message is valid.
func (m MsgToggleSwitch) ValidateBasic() error {
	return nil
}

// GetSignBytes takes a toggle switch message and turns it into a byte array.
func (m MsgToggleSwitch) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&m))
}

// GetSigners takes a toggle switch message and returns the sender in a byte array.
func (m MsgToggleSwitch) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{m.Sender}
}
