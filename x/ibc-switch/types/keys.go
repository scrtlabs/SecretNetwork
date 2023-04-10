package types

import "strings"

const (
	// todo: can the "ibc" prefix cause conflicts?
	ModuleName = "ibc-switch"
)

var (
	// RouterKey is the message route. Can only contain
	// alphanumeric characters.
	RouterKey = strings.ReplaceAll(ModuleName, "-", "")
)
