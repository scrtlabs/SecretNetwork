package types

import (
	"encoding/json"
)

//------- Results / Msgs -------------

// HandleResult is the raw response from the handle call
type HandleResult struct {
	Ok  *HandleResponse `json:"Ok,omitempty"`
	Err *StdError       `json:"Err,omitempty"`
}

// HandleResponse defines the return value on a successful handle
type HandleResponse struct {
	// Messages comes directly from the contract and is it's request for action
	Messages []CosmosMsg `json:"messages"`
	// base64-encoded bytes to return as ABCI.Data field
	Data []byte `json:"data"`
	// log message to return over abci interface
	Log []LogAttribute `json:"log"`
}

// InitResult is the raw response from the handle call
type InitResult struct {
	Ok  *InitResponse `json:"Ok,omitempty"`
	Err *StdError     `json:"Err,omitempty"`
}

// InitResponse defines the return value on a successful handle
type InitResponse struct {
	// Messages comes directly from the contract and is it's request for action
	Messages []CosmosMsg `json:"messages"`
	// log message to return over abci interface
	Log []LogAttribute `json:"log"`
}

// MigrateResult is the raw response from the handle call
type MigrateResult struct {
	Ok  *MigrateResponse `json:"Ok,omitempty"`
	Err *StdError        `json:"Err,omitempty"`
}

// MigrateResponse defines the return value on a successful handle
type MigrateResponse struct {
	// Messages comes directly from the contract and is it's request for action
	Messages []CosmosMsg `json:"messages"`
	// base64-encoded bytes to return as ABCI.Data field
	Data []byte `json:"data"`
	// log message to return over abci interface
	Log []LogAttribute `json:"log"`
}

// LogAttribute
type LogAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CosmosMsg is an rust enum and only (exactly) one of the fields should be set
// Should we do a cleaner approach in Go? (type/data?)
type CosmosMsg struct {
	Bank    *BankMsg        `json:"bank,omitempty"`
	Custom  json.RawMessage `json:"custom,omitempty"`
	Staking *StakingMsg     `json:"staking,omitempty"`
	Wasm    *WasmMsg        `json:"wasm,omitempty"`
	Gov     *GovMsg         `json:"gov,omitempty"`
}

type BankMsg struct {
	Send *SendMsg `json:"send,omitempty"`
}

type GovMsg struct {
	Vote *VoteMsg `json:"vote,omitempty"`
}

// VoteMsg contains instructions for a Cosmos-SDK/GovVote
// It has a fixed interface here and should be converted into the proper SDK format before dispatching
type VoteMsg struct {
	Proposal   uint64 `json:"proposal"`
	VoteOption string `json:"vote_option"`
}

var (
	Yes = "Yes"
	Abstain = "Abstain"
	No = "No"
	NoWithVeto = "NoWithVeto"
)

// SendMsg contains instructions for a Cosmos-SDK/SendMsg
// It has a fixed interface here and should be converted into the proper SDK format before dispatching
type SendMsg struct {
	FromAddress string `json:"from_address"`
	ToAddress   string `json:"to_address"`
	Amount      Coins  `json:"amount"`
}

type StakingMsg struct {
	Delegate   *DelegateMsg   `json:"delegate,omitempty"`
	Undelegate *UndelegateMsg `json:"undelegate,omitempty"`
	Redelegate *RedelegateMsg `json:"redelegate,omitempty"`
	Withdraw   *WithdrawMsg   `json:"withdraw,omitempty"`
}

type DelegateMsg struct {
	Validator string `json:"validator"`
	Amount    Coin   `json:"amount"`
}

type UndelegateMsg struct {
	Validator string `json:"validator"`
	Amount    Coin   `json:"amount"`
}

type RedelegateMsg struct {
	SrcValidator string `json:"src_validator"`
	DstValidator string `json:"dst_validator"`
	Amount       Coin   `json:"amount"`
}

type WithdrawMsg struct {
	Validator string `json:"validator"`
	// this is optional
	Recipient string `json:"recipient,omitempty"`
}

type WasmMsg struct {
	Execute     *ExecuteMsg     `json:"execute,omitempty"`
	Instantiate *InstantiateMsg `json:"instantiate,omitempty"`
}

// ExecuteMsg is used to call another defined contract on this chain.
// The calling contract requires the callee to be defined beforehand,
// and the address should have been defined in initialization.
// And we assume the developer tested the ABIs and coded them together.
//
// Since a contract is immutable once it is deployed, we don't need to transform this.
// If it was properly coded and worked once, it will continue to work throughout upgrades.
type ExecuteMsg struct {
	// ContractAddr is the sdk.AccAddress of the contract, which uniquely defines
	// the contract ID and instance ID. The sdk module should maintain a reverse lookup table.
	ContractAddr string `json:"contract_addr"`
	// Custom addition to support binding a message to specific code to harden against offline & replay attacks
	// This is only needed when creating a callback message
	CallbackCodeHash string `json:"callback_code_hash"`
	// Msg is assumed to be a json-encoded message, which will be passed directly
	// as `userMsg` when calling `Handle` on the above-defined contract
	Msg []byte `json:"msg"`
	// Send is an optional amount of coins this contract sends to the called contract
	Send              Coins  `json:"send"`
	CallbackSignature []byte `json:"callback_sig"` // Optional
}

type InstantiateMsg struct {
	// CodeID is the reference to the wasm byte code as used by the Cosmos-SDK
	CodeID uint64 `json:"code_id"`
	// Custom addition to support binding a message to specific code to harden against offline & replay attacks
	// This is only needed when creating a callback message
	CallbackCodeHash string `json:"callback_code_hash"`
	// Msg is assumed to be a json-encoded message, which will be passed directly
	// as `userMsg` when calling `Handle` on the above-defined contract
	Msg []byte `json:"msg"`
	/// Label is a mandatory human-readbale label for the contract
	Label string `json:"label"`
	// Send is an optional amount of coins this contract sends to the called contract
	Send              Coins  `json:"send"`
	CallbackSignature []byte `json:"callback_sig"` // Optional
}
