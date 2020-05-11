package types

// GenesisState is the struct representation of the export genesis
type GenesisState struct {
	Registration []RegInfo `json:"reg_info"`
}

// Code struct encompasses CodeInfo and CodeBytes
type RegInfo struct {
	PublicKey   []byte `json:"pubkey"`
	Certificate []byte `json:"certificate"`
	Seed        []byte `json:"seed"`
}

// ValidateGenesis performs basic validation of supply genesis data returning an
// error for any failed validation criteria.
func ValidateGenesis(data GenesisState) error {
	return nil
}
