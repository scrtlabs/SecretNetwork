package types

import "C"
import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

type Sequence struct {
	IDKey []byte `json:"id_key"`
	Value uint64 `json:"value"`
}

func (s Sequence) ValidateBasic() error {
	if len(s.IDKey) == 0 {
		return sdkerrors.Wrap(ErrEmpty, "id key")
	}
	return nil
}

// GenesisState is the struct representation of the export genesis
type GenesisState struct {
	Params    Params     `json:"params"`
	Codes     []Code     `json:"codes,omitempty"`
	Contracts []Contract `json:"contracts,omitempty"`
	Sequences []Sequence `json:"sequences,omitempty"`
}

func (s GenesisState) ValidateBasic() error {
	if err := s.Params.ValidateBasic(); err != nil {
		return sdkerrors.Wrap(err, "params")
	}
	for i := range s.Codes {
		if err := s.Codes[i].ValidateBasic(); err != nil {
			return sdkerrors.Wrapf(err, "code: %d", i)
		}
	}
	for i := range s.Contracts {
		if err := s.Contracts[i].ValidateBasic(); err != nil {
			return sdkerrors.Wrapf(err, "contract: %d", i)
		}
	}
	for i := range s.Sequences {
		if err := s.Sequences[i].ValidateBasic(); err != nil {
			return sdkerrors.Wrapf(err, "sequence: %d", i)
		}
	}
	return nil
}

// Code struct encompasses CodeInfo and CodeBytes
type Code struct {
	CodeID     uint64   `json:"code_id"`
	CodeInfo   CodeInfo `json:"code_info"`
	CodesBytes []byte   `json:"code_bytes"`
}

func (c Code) ValidateBasic() error {
	if c.CodeID == 0 {
		return sdkerrors.Wrap(ErrEmpty, "code id")
	}
	if err := c.CodeInfo.ValidateBasic(); err != nil {
		return sdkerrors.Wrap(err, "code info")
	}
	if err := validateWasmCode(c.CodesBytes); err != nil {
		return sdkerrors.Wrap(err, "code bytes")
	}
	return nil
}

// Contract struct encompasses ContractAddress, ContractInfo, and ContractState
type Contract struct {
	ContractAddress sdk.AccAddress `json:"contract_address"`
	ContractInfo    ContractInfo   `json:"contract_info"`
	ContractState   []Model        `json:"contract_state"`
}

func (c Contract) ValidateBasic() error {
	if err := sdk.VerifyAddressFormat(c.ContractAddress); err != nil {
		return sdkerrors.Wrap(err, "contract address")
	}
	if err := c.ContractInfo.ValidateBasic(); err != nil {
		return sdkerrors.Wrap(err, "contract info")
	}

	if c.ContractInfo.Created != nil {
		return sdkerrors.Wrap(ErrInvalid, "created must be empty")
	}
	for i := range c.ContractState {
		if err := c.ContractState[i].ValidateBasic(); err != nil {
			return sdkerrors.Wrapf(err, "contract state %d", i)
		}
	}
	return nil
}

// ValidateGenesis performs basic validation of supply genesis data returning an
// error for any failed validation criteria.
func ValidateGenesis(data GenesisState) error {
	return data.ValidateBasic()
}
