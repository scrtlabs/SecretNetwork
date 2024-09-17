package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewParams(switchStatus string, pauserAddress string) Params {
	return Params{
		SwitchStatus:  switchStatus,
		PauserAddress: pauserAddress,
	}
}

// default module parameters.
func DefaultParams() Params {
	return NewParams(IbcSwitchStatusOn, "")
}

// validate params.
func (p Params) Validate() error {
	return validatePauserAddress(p.PauserAddress)
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
