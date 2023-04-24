package types

import "strings"

const (
	ModuleName = "ibc-switch"
)

// RouterKey is the message route. Can only contain
// alphanumeric characters.
var RouterKey = strings.ReplaceAll(ModuleName, "-", "")

var (
	KeySwitchStatus  = []byte("switchstatus")
	KeyPauserAddress = []byte("pauseraddress")
)

const (
	IbcSwitchStatusOff string = "off"
	IbcSwitchStatusOn  string = "on"
)
