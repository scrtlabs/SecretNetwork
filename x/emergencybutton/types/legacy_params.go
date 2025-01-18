package types

import (
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Parameter store keys.
var _ paramtypes.ParamSet = &Params{}

var (
	KeySwitchStatus  = []byte("switchstatus")
	KeyPauserAddress = []byte("pauseraddress")
)

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// Implements params.ParamSet.
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeySwitchStatus, &p.SwitchStatus, validateSwitchStatus),
		paramtypes.NewParamSetPair(KeyPauserAddress, &p.PauserAddress, validatePauserAddress),
	}
}
