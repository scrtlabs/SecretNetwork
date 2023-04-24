package types

import "strings"

const (
	ModuleName = "ibc-switch"
)

var (
	// RouterKey is the message route. Can only contain
	// alphanumeric characters.
	RouterKey = strings.ReplaceAll(ModuleName, "-", "")
)

var (
	KeySwitchStatus  = []byte("switchstatus")
	KeyPauserAddress = []byte("pauseraddress")
)

const (
	IbcSwitchStatusOff string = "off"
	IbcSwitchStatusOn         = "on"
)
