package types

import (
	"fmt"
)

// DefaultGenesisState returns GenesisState with defaults.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:        DefaultParams(),
		RedeemEntries: []RedeemEntry{},
	}
}

// Validate perform a GenesisState object validation.
func (s GenesisState) Validate() error {
	if err := s.Params.Validate(); err != nil {
		return fmt.Errorf("params: %w", err)
	}

	for i, entry := range s.RedeemEntries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("redeem_entries [%d]: %w", i, err)
		}
	}

	return nil
}
