package types

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Minter represents the minting state.
type Minter struct {
	// AnnualProvisions tracks current annual expected provisions
	// This is purely informational for queries
	AnnualProvisions math.LegacyDec `json:"annual_provisions"`
}

// NewMinter returns a new Minter object
func NewMinter(annualProvisions math.LegacyDec) Minter {
	return Minter{
		AnnualProvisions: annualProvisions,
	}
}

// DefaultMinter returns a default initial Minter object
func DefaultMinter() Minter {
	return NewMinter(math.LegacyNewDec(0))
}

// BlockProvision returns the fixed provisions for a block based on the fixed block reward parameter.
// Unlike the standard SDK mint module which calculates based on inflation and supply,
// this returns a constant amount per block.
func (m Minter) BlockProvision(params Params) sdk.Coin {
	return sdk.NewCoin(params.MintDenom, params.FixedBlockReward)
}
