package types

import (
	"fmt"

	"gopkg.in/yaml.v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/subspace"
)

const (
	// default paramspace for params keeper
	DefaultParamspace = ModuleName
)

// Parameter keys
var (
	ParamStoreKeyMultisigApproveAddress = []byte("MultisigApproveAddress")
	ParamStoreKeyMintingMultiplier      = []byte("MintingMultiplier")
	ParamStoreKeyMintingEnabled         = []byte("MintingEnabled")
)

// ParamKeyTable returns the parameter key table.
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// DefaultParams returns default distribution parameters
func DefaultParams() Params {
	addrString := "secret1n4pc2w3us9n4axa0ppadd3kv3c0sar8cxl30wq"
	multisigAddress, _ := sdk.AccAddressFromBech32(addrString)
	return Params{
		MultisigApproveAddress: multisigAddress,
		MintingMultiplier:      sdk.NewDec(1),
		MintingEnabled:         true, // for testing only. turn this off before deployment, duh:)
	}
}

func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

// ParamSetPairs returns the parameter set pairs.
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(ParamStoreKeyMultisigApproveAddress, &p.MultisigApproveAddress, validateMultisigApproveAddress),
		paramtypes.NewParamSetPair(ParamStoreKeyMintingMultiplier, &p.MintingMultiplier, validateMintingMultiple),
		paramtypes.NewParamSetPair(ParamStoreKeyMintingEnabled, &p.MintingEnabled, validateMintingEnabled),
	}
}

// ValidateBasic performs basic validation on distribution parameters.
func (p Params) ValidateBasic() error {
	if p.MintingMultiplier.IsNegative() || p.MintingMultiplier.IsNil() {
		return fmt.Errorf(
			"minting multiple should non-negative and greater than 0: %s", p.MintingMultiplier,
		)
	}
	return nil
}

func validateMultisigApproveAddress(i interface{}) error {
	_, ok := i.(sdk.AccAddress)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateMintingMultiple(i interface{}) error {
	v, ok := i.(sdk.Dec)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v.IsNil() {
		return fmt.Errorf("base proposer reward must be not nil")
	}
	if v.IsNegative() {
		return fmt.Errorf("base proposer reward must be positive: %s", v)
	}
	if v.GT(sdk.NewDec(10)) {
		return fmt.Errorf("base proposer reward too large: %s", v)
	}
	return nil
}

func validateMintingEnabled(i interface{}) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}
