package v106

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	tmBytes "github.com/tendermint/tendermint/libs/bytes"
)

const (
	ModuleName = "compute"
)

type (
	// CodeInfo is data for the uploaded contract WASM code
	CodeInfo struct {
		CodeHash []byte         `json:"code_hash"`
		Creator  sdk.AccAddress `json:"creator"`
		Source   string         `json:"source"`
		Builder  string         `json:"builder"`
	}

	// Code struct encompasses CodeInfo and CodeBytes
	Code struct {
		CodeID     uint64   `json:"code_id"`
		CodeInfo   CodeInfo `json:"code_info"`
		CodesBytes []byte   `json:"code_bytes"`
	}

	// AbsoluteTxPosition can be used to sort contracts
	AbsoluteTxPosition struct {
		// BlockHeight is the block the contract was created at
		BlockHeight int64
		// TxIndex is a monotonic counter within the block (actual transaction index, or gas consumed)
		TxIndex uint64
	}

	// ContractInfo stores a WASM contract instance
	ContractInfo struct {
		CodeID  uint64              `json:"code_id"`
		Creator sdk.AccAddress      `json:"creator"`
		Label   string              `json:"label"`
		Created *AbsoluteTxPosition `json:"created,omitempty"`
	}

	// Model is a struct that holds a KV pair
	Model struct {
		// hex-encode key to read it better (this is often ascii)
		Key tmBytes.HexBytes `json:"key"`
		// base64-encode raw value
		Value []byte `json:"val"`
	}

	// ContractInfo stores a WASM contract instance
	ContractCustomInfo struct {
		EnclaveKey []byte `json:"enclave_key"`
		Label      string `json:"label"`
	}

	// Contract struct encompasses ContractAddress, ContractInfo, and ContractState
	Contract struct {
		ContractAddress    sdk.AccAddress     `json:"contract_address"`
		ContractInfo       ContractInfo       `json:"contract_info"`
		ContractState      []Model            `json:"contract_state"`
		ContractCustomInfo ContractCustomInfo `json:"contract_custom_info"`
	}

	Sequence struct {
		IDKey []byte `json:"id_key"`
		Value uint64 `json:"value"`
	}

	// GenesisState is the struct representation of the export genesis
	GenesisState struct {
		Codes     []Code     `json:"codes,omitempty"`
		Contracts []Contract `json:"contracts,omitempty"`
		Sequences []Sequence `json:"sequences,omitempty"`
	}
)
