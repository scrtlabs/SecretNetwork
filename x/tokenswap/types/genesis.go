package types

// GenesisState - all distribution state that must be provided at genesis
type GenesisState struct {
	Params Params            `json:"params" yaml:"params"`
	Swaps  []TokenSwapRecord `json:"swaps"  yaml:"swaps"`
}

// get raw genesis raw message for testing
func DefaultGenesisState() GenesisState {
	return GenesisState{
		Params: DefaultParams(),
		Swaps:  nil,
	}
}

func NewGenesisState(params Params, swaps []TokenSwapRecord) GenesisState {
	return GenesisState{
		Params: params,
		Swaps:  swaps,
	}
}

// ValidateGenesis validates the genesis state of distribution genesis input
func ValidateGenesis(gs GenesisState) error {
	if err := gs.Params.ValidateBasic(); err != nil {
		return err
	}
	return nil
}
