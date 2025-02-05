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
	return &MsgToggleIbcSwitch{sender.String()}
}
