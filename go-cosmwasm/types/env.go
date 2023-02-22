package types

//---------- Env ---------

// Env defines the state of the blockchain environment this contract is
// running in. This must contain only trusted data - nothing from the Tx itself
// that has not been verfied (like Signer).
//
// Env are json encoded to a byte slice before passing to the wasm contract.
type Env struct {
	Block       BlockInfo        `json:"block"`
	Message     MessageInfo      `json:"message"`
	Contract    ContractInfo     `json:"contract"`
	Key         ContractKey      `json:"contract_key"`
	QueryDepth  uint32           `json:"query_depth"`
	Transaction *TransactionInfo `json:"transaction,omitempty"`
}

type TransactionInfo struct {
	// Position of this transaction in the block.
	// The first transaction has index 0
	//
	// Along with BlockInfo.Height, this allows you to get a unique
	// transaction identifier for the chain for future queries
	Index uint32 `json:"index"`
}

type BaseEnv[T Env] struct {
	First T
}

type ContractKey string

type BlockInfo struct {
	// block height this transaction is executed
	Height uint64 `json:"height"`
	// time in seconds since unix epoch - since cosmwasm 0.3
	Time    uint64 `json:"time"`
	ChainID string `json:"chain_id"`
	// Random  []byte `json:"random"`
}

type MessageInfo struct {
	// binary encoding of sdk.AccAddress executing the contract
	Sender HumanAddress `json:"sender"`
	// amount of funds send to the contract along with this message
	SentFunds Coins `json:"sent_funds"`
}

type ContractInfo struct {
	// binary encoding of sdk.AccAddress of the contract, to be used when sending messages
	Address HumanAddress `json:"address"`
}
