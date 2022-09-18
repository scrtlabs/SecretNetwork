package v1types

import (
	"encoding/json"
	"fmt"

	types "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	v010msgtypes "github.com/enigmampc/SecretNetwork/go-cosmwasm/types/v010"
)

//------- Results / Msgs -------------

// ContractResult is the raw response from the instantiate/execute/migrate calls.
// This is mirrors Rust's ContractResult<Response>.
type ContractResult struct {
	Ok  *Response `json:"ok,omitempty"`
	Err string    `json:"error,omitempty"`
}

// Response defines the return value on a successful instantiate/execute/migrate.
// This is the counterpart of [Response](https://github.com/CosmWasm/cosmwasm/blob/v0.14.0-beta1/packages/std/src/results/response.rs#L73-L88)
type Response struct {
	// Messages comes directly from the contract and is its request for action.
	// If the ReplyOn value matches the result, the runtime will invoke this
	// contract's `reply` entry point after execution. Otherwise, this is all
	// "fire and forget".
	Messages []SubMsg `json:"messages"`
	// base64-encoded bytes to return as ABCI.Data field
	Data []byte `json:"data"`
	// attributes for a log event to return over abci interface
	Attributes []v010msgtypes.LogAttribute `json:"attributes"`
	// custom events (separate from the main one that contains the attributes
	// above)
	Events []Event `json:"events"`
}

// Used to serialize both the data and the internal reply information in order to keep the api without changes
type DataWithInternalReplyInfo struct {
	InternaReplyEnclaveSig []byte `json:"internal_reply_enclave_sig"`
	InternalMsgId          []byte `json:"internal_msg_id"`
	Data                   []byte `json:"data,omitempty"`
}

// LogAttributes must encode empty array as []
type LogAttributes []v010msgtypes.LogAttribute

// MarshalJSON ensures that we get [] for empty arrays
func (a LogAttributes) MarshalJSON() ([]byte, error) {
	if len(a) == 0 {
		return []byte("[]"), nil
	}
	var raw []v010msgtypes.LogAttribute = a
	return json.Marshal(raw)
}

// UnmarshalJSON ensures that we get [] for empty arrays
func (a *LogAttributes) UnmarshalJSON(data []byte) error {
	// make sure we deserialize [] back to null
	if string(data) == "[]" || string(data) == "null" {
		return nil
	}
	var raw []v010msgtypes.LogAttribute
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = raw
	return nil
}

// CosmosMsg is an rust enum and only (exactly) one of the fields should be set
// Should we do a cleaner approach in Go? (type/data?)
type CosmosMsg struct {
	Bank         *BankMsg         `json:"bank,omitempty"`
	Custom       json.RawMessage  `json:"custom,omitempty"`
	Distribution *DistributionMsg `json:"distribution,omitempty"`
	Gov          *GovMsg          `json:"gov,omitempty"`
	IBC          *IBCMsg          `json:"ibc,omitempty"`
	Staking      *StakingMsg      `json:"staking,omitempty"`
	Stargate     *StargateMsg     `json:"stargate,omitempty"`
	Wasm         *WasmMsg         `json:"wasm,omitempty"`
}

type BankMsg struct {
	Send *SendMsg `json:"send,omitempty"`
	Burn *BurnMsg `json:"burn,omitempty"`
}

// SendMsg contains instructions for a Cosmos-SDK/SendMsg
// It has a fixed interface here and should be converted into the proper SDK format before dispatching
type SendMsg struct {
	ToAddress string      `json:"to_address"`
	Amount    types.Coins `json:"amount"`
}

// BurnMsg will burn the given coins from the contract's account.
// There is no Cosmos SDK message that performs this, but it can be done by calling the bank keeper.
// Important if a contract controls significant token supply that must be retired.
type BurnMsg struct {
	Amount types.Coins `json:"amount"`
}

type IBCMsg struct {
	Transfer     *TransferMsg     `json:"transfer,omitempty"`
	SendPacket   *SendPacketMsg   `json:"send_packet,omitempty"`
	CloseChannel *CloseChannelMsg `json:"close_channel,omitempty"`
}

type GovMsg struct {
	// This maps directly to [MsgVote](https://github.com/cosmos/cosmos-sdk/blob/v0.42.5/proto/cosmos/gov/v1beta1/tx.proto#L46-L56) in the Cosmos SDK with voter set to the contract address.
	Vote *VoteMsg `json:"vote,omitempty"`
}

type VoteOption int

type VoteMsg struct {
	ProposalId uint64     `json:"proposal_id"`
	Vote       VoteOption `json:"vote"`
}

const (
	Yes VoteOption = iota
	No
	Abstain
	NoWithVeto
)

var fromVoteOption = map[VoteOption]string{
	Yes:        "yes",
	No:         "no",
	Abstain:    "abstain",
	NoWithVeto: "no_with_veto",
}

var ToVoteOption = map[string]VoteOption{
	"yes":          Yes,
	"no":           No,
	"abstain":      Abstain,
	"no_with_veto": NoWithVeto,
}

func (v VoteOption) String() string {
	return fromVoteOption[v]
}

func (v VoteOption) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

func (v *VoteOption) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}

	voteOption, ok := ToVoteOption[j]
	if !ok {
		return fmt.Errorf("invalid vote option '%s'", j)
	}
	*v = voteOption
	return nil
}

type TransferMsg struct {
	ChannelID string     `json:"channel_id"`
	ToAddress string     `json:"to_address"`
	Amount    types.Coin `json:"amount"`
	Timeout   IBCTimeout `json:"timeout"`
}

type SendPacketMsg struct {
	ChannelID string     `json:"channel_id"`
	Data      []byte     `json:"data"`
	Timeout   IBCTimeout `json:"timeout"`
}

type CloseChannelMsg struct {
	ChannelID string `json:"channel_id"`
}

type StakingMsg struct {
	Delegate   *v010msgtypes.DelegateMsg   `json:"delegate,omitempty"`
	Undelegate *v010msgtypes.UndelegateMsg `json:"undelegate,omitempty"`
	Redelegate *v010msgtypes.RedelegateMsg `json:"redelegate,omitempty"`
	Withdraw   *v010msgtypes.WithdrawMsg   `json:"withdraw,omitempty"`
}

type DistributionMsg struct {
	SetWithdrawAddress      *SetWithdrawAddressMsg      `json:"set_withdraw_address,omitempty"`
	WithdrawDelegatorReward *WithdrawDelegatorRewardMsg `json:"withdraw_delegator_reward,omitempty"`
}

// SetWithdrawAddressMsg is translated to a [MsgSetWithdrawAddress](https://github.com/cosmos/cosmos-sdk/blob/v0.42.4/proto/cosmos/distribution/v1beta1/tx.proto#L29-L37).
// `delegator_address` is automatically filled with the current contract's address.
type SetWithdrawAddressMsg struct {
	// Address contains the `delegator_address` of a MsgSetWithdrawAddress
	Address string `json:"address"`
}

// WithdrawDelegatorRewardMsg is translated to a [MsgWithdrawDelegatorReward](https://github.com/cosmos/cosmos-sdk/blob/v0.42.4/proto/cosmos/distribution/v1beta1/tx.proto#L42-L50).
// `delegator_address` is automatically filled with the current contract's address.
type WithdrawDelegatorRewardMsg struct {
	// Validator contains `validator_address` of a MsgWithdrawDelegatorReward
	Validator string `json:"validator"`
}

// StargateMsg is encoded the same way as a protobof [Any](https://github.com/protocolbuffers/protobuf/blob/master/src/google/protobuf/any.proto).
// This is the same structure as messages in `TxBody` from [ADR-020](https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-020-protobuf-transaction-encoding.md)
type StargateMsg struct {
	TypeURL string `json:"type_url"`
	Value   []byte `json:"value"`
}

type WasmMsg struct {
	Execute     *v010msgtypes.ExecuteMsg     `json:"execute,omitempty"`
	Instantiate *v010msgtypes.InstantiateMsg `json:"instantiate,omitempty"`
}
