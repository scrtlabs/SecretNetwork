package types

import (
	"fmt"

	"cosmossdk.io/math"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Parameter store keys
var (
	KeyMintDenom        = []byte("MintDenom")
	KeyFixedBlockReward = []byte("FixedBlockReward")
	KeyBlocksPerYear    = []byte("BlocksPerYear")
)

// ParamKeyTable returns the parameter key table for the mint module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// Params holds parameters for the mint module
type Params struct {
	// MintDenom is the type of coin to mint
	MintDenom string `json:"mint_denom"`
	// FixedBlockReward is the fixed amount of tokens minted per block (in base denom)
	FixedBlockReward math.Int `json:"fixed_block_reward"`
	// BlocksPerYear is expected blocks per year (used for annual provisions calculation)
	BlocksPerYear uint64 `json:"blocks_per_year"`
}

// NewParams creates a new Params instance
func NewParams(mintDenom string, fixedBlockReward math.Int, blocksPerYear uint64) Params {
	return Params{
		MintDenom:        mintDenom,
		FixedBlockReward: fixedBlockReward,
		BlocksPerYear:    blocksPerYear,
	}
}

// DefaultParams returns default minting module parameters
// 4 SCRT per block = 4,000,000 uscrt per block
// BlocksPerYear is a governance-adjustable target (not derived from block time assumptions)
// This default is only used for genesis/tests; upgrades preserve the existing chain value
func DefaultParams() Params {
	return Params{
		MintDenom:        "uscrt",
		FixedBlockReward: math.NewInt(4_000_000), // 4 SCRT = 4,000,000 uscrt
		BlocksPerYear:    6_311_520,              // Standard cosmos-sdk default (governance-adjustable target)
	}
}

// Validate validates params
func (p Params) Validate() error {
	if err := validateMintDenom(p.MintDenom); err != nil {
		return err
	}
	if err := validateFixedBlockReward(p.FixedBlockReward); err != nil {
		return err
	}
	if err := validateBlocksPerYear(p.BlocksPerYear); err != nil {
		return err
	}
	return nil
}

func validateMintDenom(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == "" {
		return fmt.Errorf("mint denom cannot be empty")
	}

	return nil
}

func validateFixedBlockReward(i interface{}) error {
	v, ok := i.(math.Int)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v.IsNegative() {
		return fmt.Errorf("fixed block reward cannot be negative: %s", v)
	}

	return nil
}

func validateBlocksPerYear(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("blocks per year must be positive")
	}

	return nil
}

// ParamSetPairs implements params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyMintDenom, &p.MintDenom, validateMintDenom),
		paramtypes.NewParamSetPair(KeyFixedBlockReward, &p.FixedBlockReward, validateFixedBlockReward),
		paramtypes.NewParamSetPair(KeyBlocksPerYear, &p.BlocksPerYear, validateBlocksPerYear),
	}
}
