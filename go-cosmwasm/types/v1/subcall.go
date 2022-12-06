package v1types

import (
	"encoding/json"
	"fmt"
)

type replyOn int

const (
	ReplyAlways replyOn = iota
	ReplySuccess
	ReplyError
	ReplyNever
)

var fromReplyOn = map[replyOn]string{
	ReplyAlways:  "always",
	ReplySuccess: "success",
	ReplyError:   "error",
	ReplyNever:   "never",
}

var toReplyOn = map[string]replyOn{
	"always":  ReplyAlways,
	"success": ReplySuccess,
	"error":   ReplyError,
	"never":   ReplyNever,
}

func (r replyOn) String() string {
	return fromReplyOn[r]
}

func (r replyOn) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

func (r *replyOn) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}

	voteOption, ok := toReplyOn[j]
	if !ok {
		return fmt.Errorf("invalid reply_on value '%s'", j)
	}
	*r = voteOption
	return nil
}

type SubMsgResponse struct {
	Events Events `json:"events"`
	Data   []byte `json:"data,omitempty"`
}

// SubMsgResult is the raw response we return from wasmd after executing a SubMsg.
// This mirrors Rust's SubMsgResult.
type SubMsgResult struct {
	Ok  *SubMsgResponse `json:"ok,omitempty"`
	Err string          `json:"error,omitempty"`
}

// SubMsg wraps a CosmosMsg with some metadata for handling replies (ID) and optionally
// limiting the gas usage (GasLimit)
type SubMsg struct {
	ID              uint64    `json:"id"`
	Msg             CosmosMsg `json:"msg"`
	GasLimit        *uint64   `json:"gas_limit,omitempty"`
	ReplyOn         replyOn   `json:"reply_on"`
	WasMsgEncrypted bool      `json:"was_msg_encrypted"`
}

type Reply struct {
	ID                  []byte       `json:"id"`
	Result              SubMsgResult `json:"result"`
	WasOrigMsgEncrypted bool         `json:"was_orig_msg_encrypted"`
	IsEncrypted         bool         `json:"is_encrypted"`
}

// SubcallResult is the raw response we return from the sdk -> reply after executing a SubMsg.
// This is mirrors Rust's ContractResult<SubcallResponse>.
type SubcallResult struct {
	Ok  *SubcallResponse `json:"ok,omitempty"`
	Err string           `json:"error,omitempty"`
}

type SubcallResponse struct {
	Events Events `json:"events"`
	Data   []byte `json:"data,omitempty"`
}

// Events must encode empty array as []
type Events []Event

// MarshalJSON ensures that we get [] for empty arrays
func (e Events) MarshalJSON() ([]byte, error) {
	if len(e) == 0 {
		return []byte("[]"), nil
	}
	var raw []Event = e
	return json.Marshal(raw)
}

// UnmarshalJSON ensures that we get [] for empty arrays
func (e *Events) UnmarshalJSON(data []byte) error {
	// make sure we deserialize [] back to null
	if string(data) == "[]" || string(data) == "null" {
		return nil
	}
	var raw []Event
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = raw
	return nil
}

type Event struct {
	Type       string        `json:"type"`
	Attributes LogAttributes `json:"attributes"`
}
