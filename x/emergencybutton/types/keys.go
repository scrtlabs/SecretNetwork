package types

const (
	ModuleName   = "emergencybutton"
	StoreKey     = "emergencybutton"
	TStoreKey    = "emergencybutton"
	QuerierRoute = "emergencybutton"
)

// RouterKey is the message route. Can only contain
// alphanumeric characters.
var RouterKey = QuerierRoute

const (
	// IbcSwitchStatusOff - IBC messages halted
	IbcSwitchStatusOff string = "off"
	// IbcSwitchStatusOn - IBC messages enabled
	IbcSwitchStatusOn string = "on"
)
