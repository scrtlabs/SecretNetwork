package types

// NewGenesisState creates a new GenesisState object
func NewGenesisState(minter Minter, params Params) *GenesisState {
	return &GenesisState{
		Minter: minter,
		Params: params,
	}
}

// DefaultGenesisState creates a default GenesisState object
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Minter: DefaultMinter(),
		Params: DefaultParams(),
	}
}

// GenesisState defines the mint module's genesis state.
type GenesisState struct {
	// Minter is the current minter
	Minter Minter `json:"minter"`
	// Params defines all the parameters of the mint module.
	Params Params `json:"params"`
}

// Validate performs basic validation of mint genesis data.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	return nil
}
