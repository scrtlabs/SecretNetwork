package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	TypeMsgToggleIbcSwitch = "toggle_switch"
)

var _ sdk.Msg = &MsgToggleIbcSwitch{}

// NewMsgToggleIbcSwitch creates a message to toggle switch
func NewMsgToggleIbcSwitch(sender sdk.AccAddress) *MsgToggleIbcSwitch {
	return &MsgToggleIbcSwitch{sender}
}

// Route takes a toggle switch message, then returns the RouterKey used for slashing.
func (m MsgToggleIbcSwitch) Route() string { return RouterKey }

// Type takes a toggle switch message, then returns a toggle switch message type.
func (m MsgToggleIbcSwitch) Type() string { return TypeMsgToggleIbcSwitch }

// ValidateBasic checks that the toggle switch message is valid.
func (m MsgToggleIbcSwitch) ValidateBasic() error {
	return nil
}

// GetSignBytes takes a toggle switch message and turns it into a byte array.
func (m MsgToggleIbcSwitch) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&m))
}

// GetSigners takes a toggle switch message and returns the sender in an array.
func (m MsgToggleIbcSwitch) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{m.Sender}
}
