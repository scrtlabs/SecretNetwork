package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	yaml "gopkg.in/yaml.v2"
)

// Validate performs a basic TokenMeta validation.
func (m TokenMeta) Validate() error {
	if err := sdk.ValidateDenom(m.Denom); err != nil {
		return fmt.Errorf("tokenMeta (%s): validation: %w", m.Denom, err)
	}

	if m.Decimals == 0 {
		return fmt.Errorf("tokenMeta (%s): decimals must be GT 0", m.Denom)
	}

	return nil
}

// String implements the fmt.Stringer interface.
func (m TokenMeta) String() string {
	out, _ := yaml.Marshal(m)

	return string(out)
}

// DecUnit returns minimal token sdk.Dec value.
func (m TokenMeta) DecUnit() sdk.Dec {
	return sdk.NewDecWithPrec(1, int64(m.Decimals))
}

// NewZeroCoin returns a new empty sdk.Coin for the meta.
func (m TokenMeta) NewZeroCoin() sdk.Coin {
	return sdk.NewCoin(m.Denom, sdk.ZeroInt())
}

// ConvertCoin converts sdk.Coin to a given TokenMeta.
// Function is a variation of sdk.ConvertCoin.
func (m TokenMeta) ConvertCoin(coin sdk.Coin, dstMeta TokenMeta) (sdk.Coin, error) {
	if coin.Denom != m.Denom {
		return sdk.Coin{}, fmt.Errorf("coin.Denom (%s) is NE srcMeta.Denom (%s)", coin.Denom, m.Denom)
	}
	if err := m.Validate(); err != nil {
		return sdk.Coin{}, fmt.Errorf("invalid srcMeta: %w", err)
	}
	if err := dstMeta.Validate(); err != nil {
		return sdk.Coin{}, fmt.Errorf("invalid dstMeta: %w", err)
	}

	srcUnit, dstUnit := m.DecUnit(), dstMeta.DecUnit()
	if srcUnit.Equal(dstUnit) {
		return sdk.NewCoin(dstMeta.Denom, coin.Amount), nil
	}

	return sdk.NewCoin(dstMeta.Denom, coin.Amount.ToDec().Mul(srcUnit).Quo(dstUnit).TruncateInt()), nil
}

// ConvertCoin2 converts sdk.Coin to a given TokenMeta returning a converted coin and an adjusted input coin.
// {dstCoin} - a conversion result coin (amount loss could happen due to truncation of decimals).
// {srcCoin} - a copy of the input coin with adjusted amount that indicates what amount was actually converted (equals to the input if no loss occurred).
func (m TokenMeta) ConvertCoin2(coin sdk.Coin, dstMeta TokenMeta) (dstCoin sdk.Coin, srcCoin sdk.Coin, retErr error) {
	// Validation
	if coin.Denom != m.Denom {
		retErr = fmt.Errorf("coin.Denom (%s) is NE srcMeta.Denom (%s)", coin.Denom, m.Denom)
		return
	}
	if err := m.Validate(); err != nil {
		retErr = fmt.Errorf("invalid srcMeta: %w", err)
		return
	}
	if err := dstMeta.Validate(); err != nil {
		retErr = fmt.Errorf("invalid dstMeta: %w", err)
		return
	}

	// Set resulting coins denom
	dstCoin.Denom, srcCoin.Denom = dstMeta.Denom, m.Denom

	// Noop, 1:1 conversion
	if m.Decimals == dstMeta.Decimals {
		dstCoin.Amount, srcCoin.Amount = coin.Amount, coin.Amount
		return
	}

	// Convert the destination coin amount
	srcUnit, dstUnit := m.DecUnit(), dstMeta.DecUnit()
	dstCoin.Amount = coin.Amount.ToDec().Mul(srcUnit).Quo(dstUnit).TruncateInt()

	// If dstMeta has a higher precision, source coin is converted without a loss
	if dstMeta.Decimals > m.Decimals {
		srcCoin.Amount = coin.Amount
		return
	}

	// Estimate how much of the source coin was converted
	srcCoin.Amount = dstCoin.Amount
	for i := uint32(0); i < m.Decimals-dstMeta.Decimals; i++ {
		srcCoin.Amount = srcCoin.Amount.MulRaw(10)
	}

	return
}

// NormalizeCoin converts sdk.Coin to a smaller decimals unit.
// Function is a variation of sdk.NormalizeCoin.
func (m TokenMeta) NormalizeCoin(coin sdk.Coin, dstMeta TokenMeta) (sdk.Coin, error) {
	if dstMeta.Decimals < m.Decimals {
		return sdk.Coin{}, fmt.Errorf("dstMeta.Decimals (%d) is LT srcMeta.Decimals (%d)", dstMeta.Decimals, m.Decimals)
	}

	coinNormalized, err := m.ConvertCoin(coin, dstMeta)
	if err != nil {
		return sdk.Coin{}, err
	}

	return sdk.NewCoin(m.Denom, coinNormalized.Amount), nil
}
