package types

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	yaml "gopkg.in/yaml.v2"
)

// NewRedeemEntry creates a new RedeemEntry object.
func NewRedeemEntry(accAddr sdk.AccAddress) RedeemEntry {
	return RedeemEntry{
		Address: accAddr.String(),
	}
}

// AddOperation appends a new redeem operation.
func (e *RedeemEntry) AddOperation(creationHeight int64, completionTime time.Time, amount sdk.Coins) {
	op := NewRedeemEntryOperation(creationHeight, completionTime, amount)
	e.Operations = append(e.Operations, op)
}

// RemoveMatureOperations returns and removes all mature redeem operations.
func (e *RedeemEntry) RemoveMatureOperations(currentTime time.Time) []RedeemEntryOperation {
	matureOps := make([]RedeemEntryOperation, 0, len(e.Operations))

	n := 0
	for _, op := range e.Operations {
		// Keep
		if !op.IsMature(currentTime) {
			e.Operations[n] = op
			n++
			continue
		}

		// Remove
		matureOps = append(matureOps, op)
	}
	e.Operations = e.Operations[:n]

	return matureOps
}

// OperationsLeft returns the number of redeem operation to go.
func (e RedeemEntry) OperationsLeft() int {
	return len(e.Operations)
}

// Validate performs an object validation.
func (e RedeemEntry) Validate() error {
	if _, err := sdk.AccAddressFromBech32(e.Address); err != nil {
		return fmt.Errorf("address: parsing Bech32: %w", err)
	}

	for i, op := range e.Operations {
		if err := op.Validate(); err != nil {
			return fmt.Errorf("operations [%d]: %w", i, err)
		}
	}

	return nil
}

// String implements the fmt.Stringer interface.
func (e RedeemEntry) String() string {
	out, _ := yaml.Marshal(e)
	return string(out)
}

// NewRedeemEntryOperation creates a new RedeemEntryOperation object.
func NewRedeemEntryOperation(creationHeight int64, completionTime time.Time, amount sdk.Coins) RedeemEntryOperation {
	return RedeemEntryOperation{
		CreationHeight:   creationHeight,
		CompletionTime:   completionTime,
		CollateralAmount: amount,
	}
}

// IsMature checks if operation is mature to be competed.
func (o RedeemEntryOperation) IsMature(currentTime time.Time) bool {
	return !o.CompletionTime.After(currentTime)
}

// Validate performs an object validation.
func (o RedeemEntryOperation) Validate() error {
	if o.CreationHeight < 0 {
		return fmt.Errorf("creation_height: must be GTE 0 (%d)", o.CreationHeight)
	}

	if o.CompletionTime.IsZero() {
		return fmt.Errorf("completion_time: empty")
	}

	for i, coin := range o.CollateralAmount {
		if err := sdk.ValidateDenom(coin.Denom); err != nil {
			return fmt.Errorf("collateral_amount [%d]: invalid denom (%s): %w", i, coin.Denom, err)
		}

		if coin.Amount.IsNegative() || coin.Amount.IsZero() {
			return fmt.Errorf("collateral_amount [%d]: amount must be GT 0", i)
		}
	}

	return nil
}

// String implements the fmt.Stringer interface.
func (o RedeemEntryOperation) String() string {
	out, _ := yaml.Marshal(o)
	return string(out)
}

// String implements the fmt.Stringer interface.
func (m RedeemingQueueData) String() string {
	out, _ := yaml.Marshal(m)
	return string(out)
}
