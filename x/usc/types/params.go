package types

import (
	"fmt"
	"time"

	paramsTypes "github.com/cosmos/cosmos-sdk/x/params/types"
	yaml "gopkg.in/yaml.v2"
)

// Params default values.
const (
	DefaultRedeemPeriod     = 2 * 7 * 24 * time.Hour // 2 weeks
	DefaultMaxRedeemEntries = 7

	DefaultUSCDenom    = "uusc"
	DefaultUSCDesc     = "USC native token (micro USC)"
	DefaultUSCDecimals = 6
)

// Params storage keys.
var (
	ParamsKeyRedeemDur        = []byte("RedeemDur")
	ParamsKeyMaxRedeemEntries = []byte("MaxRedeemEntries")
	ParamsKeyCollateralMetas  = []byte("CollateralMetas")
	ParamsKeyUSCMeta          = []byte("USCMeta")
	ParamsKeyEnabled          = []byte("Enabled")
)

var _ paramsTypes.ParamSet = &Params{}

// NewParams creates a new Params object.
func NewParams(redeemDur time.Duration, maxRedeemEntries uint32, collateralMetas []TokenMeta, uscMeta TokenMeta, enabled bool) Params {
	return Params{
		RedeemDur:        redeemDur,
		MaxRedeemEntries: maxRedeemEntries,
		CollateralMetas:  collateralMetas,
		UscMeta:          uscMeta,
		Enabled:          enabled,
	}
}

// DefaultParams returns Params with defaults.
func DefaultParams() Params {
	return Params{
		RedeemDur:        DefaultRedeemPeriod,
		MaxRedeemEntries: DefaultMaxRedeemEntries,
		CollateralMetas:  []TokenMeta{},
		UscMeta: TokenMeta{
			Denom:       DefaultUSCDenom,
			Decimals:    DefaultUSCDecimals,
			Description: DefaultUSCDesc,
		},
		Enabled: false,
	}
}

// ParamKeyTable returns module params table.
func ParamKeyTable() paramsTypes.KeyTable {
	return paramsTypes.NewKeyTable().RegisterParamSet(&Params{})
}

// String implements the fmt.Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)

	return string(out)
}

// ParamSetPairs implements the paramsTypes.ParamSet interface.
func (p *Params) ParamSetPairs() paramsTypes.ParamSetPairs {
	return paramsTypes.ParamSetPairs{
		paramsTypes.NewParamSetPair(ParamsKeyRedeemDur, &p.RedeemDur, validateRedeemDurParam),
		paramsTypes.NewParamSetPair(ParamsKeyMaxRedeemEntries, &p.MaxRedeemEntries, validateMaxRedeemEntriesParam),
		paramsTypes.NewParamSetPair(ParamsKeyCollateralMetas, &p.CollateralMetas, validateCollateralMetasParam),
		paramsTypes.NewParamSetPair(ParamsKeyUSCMeta, &p.UscMeta, validateUscMeta),
		paramsTypes.NewParamSetPair(ParamsKeyEnabled, &p.Enabled, validateEnabledParam),
	}
}

// Validate perform a Params object validation.
func (p Params) Validate() error {
	// Basic
	if err := validateRedeemDurParam(p.RedeemDur); err != nil {
		return err
	}

	if err := validateMaxRedeemEntriesParam(p.MaxRedeemEntries); err != nil {
		return err
	}

	if err := validateCollateralMetasParam(p.CollateralMetas); err != nil {
		return err
	}

	if err := validateUscMeta(p.UscMeta); err != nil {
		return err
	}

	if err := validateEnabledParam(p.Enabled); err != nil {
		return err
	}

	// USC is not a part of Collaterals
	for _, colMeta := range p.CollateralMetas {
		if colMeta.Denom == p.UscMeta.Denom {
			return fmt.Errorf("usc_meta denom (%s) is used within collateral_metas", p.UscMeta.Denom)
		}
	}

	return nil
}

// validateRedeemDurParam validates the RedeemDur param.
func validateEnabledParam(i interface{}) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("redeem_dur param: %w", retErr)
		}
	}()

	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type (%T, bool is expected)", i)
	}

	return
}

// validateRedeemDurParam validates the RedeemDur param.
func validateRedeemDurParam(i interface{}) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("redeem_dur param: %w", retErr)
		}
	}()

	v, ok := i.(time.Duration)
	if !ok {
		return fmt.Errorf("invalid parameter type (%T, time.Duration is expected)", i)
	}

	if v < 0 {
		return fmt.Errorf("must be GTE 0 (%d)", v)
	}

	return
}

// validateMaxRedeemEntriesParam validates the MaxRedeemEntries param.
func validateMaxRedeemEntriesParam(i interface{}) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("max_redeem_entries param: %w", retErr)
		}
	}()

	if _, ok := i.(uint32); !ok {
		return fmt.Errorf("invalid parameter type (%T, uint32 is expected)", i)
	}

	return
}

// validateCollateralMetasParam validates the CollateralMetas param.
func validateCollateralMetasParam(i interface{}) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("collateral_metas param: %w", retErr)
		}
	}()

	v, ok := i.([]TokenMeta)
	if !ok {
		return fmt.Errorf("invalid parameter type (%T, []string is expected)", i)
	}

	denomSet := make(map[string]struct{})
	for _, meta := range v {
		if err := meta.Validate(); err != nil {
			return err
		}

		if _, ok := denomSet[meta.Denom]; ok {
			return fmt.Errorf("tokenMeta (%s): duplicated", meta.Denom)
		}
		denomSet[meta.Denom] = struct{}{}
	}

	return
}

// validateUscMeta validates the Usc param.
func validateUscMeta(i interface{}) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("usc_meta param: %w", retErr)
		}
	}()

	v, ok := i.(TokenMeta)
	if !ok {
		return fmt.Errorf("invalid parameter type (%T, []string is expected)", i)
	}

	if err := v.Validate(); err != nil {
		return err
	}

	return
}
