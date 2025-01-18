package types

import (
	"cosmossdk.io/math"
)

const DefaultMaxContractSize = 2 * 1024 * 1024

var DefaultCompileCost = math.LegacyNewDecWithPrec(8, 1)

func NewParams(maxContractSize uint64, compileCost math.LegacyDec) Params {
	return Params{
		MaxContractSize: maxContractSize,
		CompileCost:     compileCost,
	}
}

// default module parameters.
func DefaultParams() Params {
	return NewParams(DefaultMaxContractSize, DefaultCompileCost)
}

// validate params.
func (p Params) Validate() error {
	return nil
}
