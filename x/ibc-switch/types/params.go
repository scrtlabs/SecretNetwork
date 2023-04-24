package types

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Parameter store keys.
var _ paramtypes.ParamSet = &Params{}

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams(switchStatus string, pauserAddress string) (Params, error) {
	return Params{
		SwitchStatus:  switchStatus,
		PauserAddress: pauserAddress,
	}, nil
}

// default gamm module parameters.
func DefaultParams() Params {
	return Params{
		SwitchStatus: IbcSwitchStatusOn,
	}
}

// validate params.
func (p Params) Validate() error {
	if err := validatePauserAddress(p.PauserAddress); err != nil {
		return err
	}

	return nil
}

// Implements params.ParamSet.
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeySwitchStatus, &p.SwitchStatus, validateSwitchStatus),
		paramtypes.NewParamSetPair(KeyPauserAddress, &p.PauserAddress, validatePauserAddress),
	}
}

func validatePauserAddress(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type for pauser address: %T", i)
	}

	// Empty strings are valid for unsetting the param
	if v == "" {
		return nil
	}

	// Checks that the contract address is valid
	bech32, err := sdk.AccAddressFromBech32(v)
	if err != nil {
		return err
	}

	err = sdk.VerifyAddressFormat(bech32)
	if err != nil {
		return err
	}

	return nil
}

func validateSwitchStatus(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type for switch status: %T", i)
	}

	if v != IbcSwitchStatusOn && v != IbcSwitchStatusOff {
		return fmt.Errorf("invalid value for switch status: %s", v)
	}

	return nil
}
