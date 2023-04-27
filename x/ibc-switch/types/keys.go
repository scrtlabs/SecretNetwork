package types

const (
	ModuleName = "ibc-switch"
	StoreKey
	TStoreKey
	QuerierRoute = "ibcswitch"
)

// RouterKey is the message route. Can only contain
// alphanumeric characters.
var RouterKey = QuerierRoute

var (
	KeySwitchStatus  = []byte("switchstatus")
	KeyPauserAddress = []byte("pauseraddress")
)

const (
	IbcSwitchStatusOff string = "off"
	IbcSwitchStatusOn  string = "on"
)
