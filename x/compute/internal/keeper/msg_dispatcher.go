package keeper

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/gogoproto/proto"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	v010wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	v1wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

// Messenger is an extension point for custom wasmd message handling

type Messenger interface {
	// DispatchMsg encodes the wasmVM message and dispatches it.
	DispatchMsg(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg v1wasmTypes.CosmosMsg) (events []sdk.Event, data [][]byte, err error)
}

// Replyer is a subset of keeper that can handle replies to submessages
type Replyer interface {
	reply(ctx sdk.Context, contractAddress sdk.AccAddress, reply v1wasmTypes.Reply, ogTx []byte, ogSigInfo wasmTypes.SigInfo) ([]byte, error)
	GetLastMsgMarkerContainer() *baseapp.LastMsgMarkerContainer
}

// MessageDispatcher coordinates message sending and submessage reply/ state commits
type MessageDispatcher struct {
	messenger Messenger
	keeper    Replyer
}

// NewMessageDispatcher constructor
func NewMessageDispatcher(messenger Messenger, keeper Replyer) *MessageDispatcher {
	return &MessageDispatcher{messenger: messenger, keeper: keeper}
}

func filterEvents(events []sdk.Event) []sdk.Event {
	// pre-allocate space for efficiency
	res := make([]sdk.Event, 0, len(events))
	for _, ev := range events {
		if ev.Type != "message" {
			res = append(res, ev)
		}
	}
	return res
}

func sdkAttributesToWasmVMAttributes(attrs []abci.EventAttribute) []v010wasmTypes.LogAttribute {
	res := make([]v010wasmTypes.LogAttribute, len(attrs))
	for i, attr := range attrs {
		res[i] = v010wasmTypes.LogAttribute{
			Key:   attr.Key,
			Value: attr.Value,
		}
	}
	return res
}

func sdkEventsToWasmVMEvents(events []sdk.Event) []v1wasmTypes.Event {
	res := make([]v1wasmTypes.Event, len(events))
	for i, ev := range events {
		res[i] = v1wasmTypes.Event{
			Type:       ev.Type,
			Attributes: sdkAttributesToWasmVMAttributes(ev.Attributes),
		}
	}
	return res
}

// dispatchMsgWithGasLimit sends a message with gas limit applied
func (d MessageDispatcher) dispatchMsgWithGasLimit(ctx sdk.Context, contractAddr sdk.AccAddress, ibcPort string, msg v1wasmTypes.CosmosMsg, gasLimit uint64) (events []sdk.Event, data [][]byte, err error) {
	limitedMeter := storetypes.NewGasMeter(gasLimit)
	subCtx := ctx.WithGasMeter(limitedMeter)

	// catch out of gas panic and just charge the entire gas limit
	defer func() {
		if r := recover(); r != nil {
			// if it's not an OutOfGas error, raise it again
			if _, ok := r.(storetypes.ErrorOutOfGas); !ok {
				// log it to get the original stack trace somewhere (as panic(r) keeps message but stacktrace to here
				moduleLogger(ctx).Info("SubMsg rethrowing panic: %#v", r)
				panic(r)
			}
			ctx.GasMeter().ConsumeGas(gasLimit, "Sub-Message OutOfGas panic")
			err = sdkerrors.ErrOutOfGas.Wrap("SubMsg hit gas limit")
		}
	}()
	events, data, err = d.messenger.DispatchMsg(subCtx, contractAddr, ibcPort, msg)

	// make sure we charge the parent what was spent
	spent := subCtx.GasMeter().GasConsumed()
	ctx.GasMeter().ConsumeGas(spent, "From limited Sub-Message")

	return events, data, err
}

type InvalidRequest struct {
	Err     string `json:"error"`
	Request []byte `json:"request"`
}

func (e InvalidRequest) Error() string {
	return fmt.Sprintf("invalid request: %s - original request: %s", e.Err, string(e.Request))
}

type InvalidResponse struct {
	Err      string `json:"error"`
	Response []byte `json:"response"`
}

func (e InvalidResponse) Error() string {
	return fmt.Sprintf("invalid response: %s - original response: %s", e.Err, string(e.Response))
}

type NoSuchContract struct {
	Addr string `json:"addr,omitempty"`
}

func (e NoSuchContract) Error() string {
	return fmt.Sprintf("no such contract: %s", e.Addr)
}

type Unknown struct{}

func (e Unknown) Error() string {
	return "unknown system error"
}

type UnsupportedRequest struct {
	Kind string `json:"kind,omitempty"`
}

func (e UnsupportedRequest) Error() string {
	return fmt.Sprintf("unsupported request: %s", e.Kind)
}

// Reply is encrypted only when it is a contract reply
func isReplyEncrypted(msg v1wasmTypes.SubMsg) bool {
	if msg.Msg.Wasm == nil {
		return false
	}

	return msg.WasMsgEncrypted
}

// Issue #759 - we don't return error string for worries of non-determinism
func redactError(err error) (bool, error) {
	// Do not redact encrypted wasm contract errors
	if strings.HasPrefix(err.Error(), "encrypted:") {
		// remove encrypted sign
		e := strings.ReplaceAll(err.Error(), "encrypted: ", "")
		e = strings.ReplaceAll(e, ": execute contract failed", "")
		e = strings.ReplaceAll(e, ": instantiate contract failed", "")
		e = strings.ReplaceAll(e, ": migrate contract failed", "")
		return false, fmt.Errorf("%s", e)
	}

	// Do not redact system errors
	// SystemErrors must be created in x/wasm and we can ensure determinism
	if wasmTypes.ToSystemError(err) != nil {
		return false, err
	}

	// FIXME: do we want to hardcode some constant string mappings here as well?
	// Or better document them? (SDK error string may change on a patch release to fix wording)
	// sdk/11 is out of gas
	// sdk/5 is insufficient funds (on bank send)
	// (we can theoretically redact less in the future, but this is a first step to safety)
	codespace, code, _ := errorsmod.ABCIInfo(err, false)

	// In software mode, ignore redaction in order the understand the errors.
	if os.Getenv("SGX_MODE") == "SW" {
		return true, err
	}

	return true, fmt.Errorf("the error was redacted (codespace: %s, code: %d). For more info use latest localsecret and reproduce the issue", codespace, code)
}

// DispatchSubmessages builds a sandbox to execute these messages and returns the execution result to the contract
// that dispatched them, both on success as well as failure
func (d MessageDispatcher) DispatchSubmessages(ctx sdk.Context, contractAddr sdk.AccAddress, ibcPort string, msgs []v1wasmTypes.SubMsg, ogTx []byte, ogSigInfo wasmTypes.SigInfo) ([]byte, error) {
	var rsp []byte
	for _, msg := range msgs {

		if d.keeper.GetLastMsgMarkerContainer().GetMarker() {
			return nil, sdkerrors.ErrLastTx.Wrap("Cannot send messages or submessages after last tx marker was set")
		}

		if msg.Msg.FinalizeTx != nil {
			d.keeper.GetLastMsgMarkerContainer().SetMarker(true)

			// no handler is defined for marker - it's just to get here
			break
		}

		// Check replyOn validity
		switch msg.ReplyOn {
		case v1wasmTypes.ReplySuccess, v1wasmTypes.ReplyError, v1wasmTypes.ReplyAlways, v1wasmTypes.ReplyNever:
		default:
			return nil, errorsmod.Wrap(types.ErrInvalid, "ReplyOn value")
		}

		// first, we build a sub-context which we can use inside the submessages
		subCtx, commit := ctx.CacheContext()
		em := sdk.NewEventManager()
		subCtx = subCtx.WithEventManager(em)

		// check how much gas left locally, optionally wrap the gas meter
		gasRemaining := ctx.GasMeter().Limit() - ctx.GasMeter().GasConsumed()
		limitGas := msg.GasLimit != nil && (*msg.GasLimit < gasRemaining)

		var err error
		var events []sdk.Event
		var data [][]byte
		if limitGas {
			events, data, err = d.dispatchMsgWithGasLimit(subCtx, contractAddr, ibcPort, msg.Msg, *msg.GasLimit)
		} else {
			events, data, err = d.messenger.DispatchMsg(subCtx, contractAddr, ibcPort, msg.Msg)
		}

		// if it succeeds, commit state changes from submessage, and pass on events to Event Manager
		var filteredEvents []sdk.Event
		if err == nil {
			commit()
			filteredEvents = filterEvents(append(em.Events(), events...))
			ctx.EventManager().EmitEvents(filteredEvents)

			if msg.Msg.Wasm == nil {
				filteredEvents = []sdk.Event{}
			} else {
				for _, e := range filteredEvents {
					attributes := e.Attributes
					sort.SliceStable(attributes, func(i, j int) bool {
						return attributes[i].Key < attributes[j].Key
					})
				}
			}
		} // on failure, revert state from sandbox, and ignore events (just skip doing the above)

		// we only callback if requested. Short-circuit here the cases we don't want to
		if err != nil && (msg.ReplyOn == v1wasmTypes.ReplySuccess || msg.ReplyOn == v1wasmTypes.ReplyNever) {
			// Note: this also handles the case of v0.10 submessage for which the execution failed
			return nil, err
		}

		if msg.ReplyOn == v1wasmTypes.ReplyNever || (err == nil && msg.ReplyOn == v1wasmTypes.ReplyError) {
			continue
		}

		// If we are here it means that ReplySuccess and success OR ReplyError and there were errors OR ReplyAlways.
		// Basically, handle replying to the contract
		// We need to create a SubMsgResult and pass it into the calling contract
		var result v1wasmTypes.SubMsgResult
		var redactedErr error

		isSdkError := false
		if err == nil {
			// just take the first one for now if there are multiple sub-sdk messages
			// and safely return nothing if no data
			var responseData []byte
			if len(data) > 0 {
				responseData = data[0]
			}
			result = v1wasmTypes.SubMsgResult{
				// Copy first 64 bytes of the OG message in order to preserve the pubkey.
				Ok: &v1wasmTypes.SubMsgResponse{
					Events: sdkEventsToWasmVMEvents(filteredEvents),
					Data:   responseData,
				},
			}
		} else {
			// Issue #759 - we don't return error string for worries of non-determinism
			moduleLogger(ctx).Info("Redacting submessage error", "cause", err)
			isSdkError, redactedErr = redactError(err)
			result = v1wasmTypes.SubMsgResult{
				Err: redactedErr.Error(),
			}
		}

		msg_id := []byte(fmt.Sprint(msg.ID))
		// now handle the reply, we use the parent context, and abort on error
		reply := v1wasmTypes.Reply{
			ID:                  msg_id,
			Result:              result,
			WasOrigMsgEncrypted: msg.WasMsgEncrypted,
			IsEncrypted:         false,
		}

		// we can ignore any result returned as there is nothing to do with the data
		// and the events are already in the ctx.EventManager()

		// In order to specify that the reply isn't signed by the enclave we use "SIGN_MODE_UNSPECIFIED"
		// The SGX will notice that the value is SIGN_MODE_UNSPECIFIED and will treat the message as plaintext.
		replySigInfo := wasmTypes.SigInfo{
			TxBytes:   []byte{},
			SignBytes: []byte{},
			ModeInfo:  []byte{},
			PublicKey: []byte{},
			Signature: []byte{},
			SignMode:  "SIGN_MODE_UNSPECIFIED",
		}

		// In a case when the reply is encrypted but the sdk failed (Most likely, funds issue)
		// we return a error
		if isReplyEncrypted(msg) && isSdkError {
			return nil, fmt.Errorf("an sdk error occurred while sending a sub-message: %s", redactedErr.Error())
		}

		if isReplyEncrypted(msg) {
			var dataWithInternalReplyInfo v1wasmTypes.DataWithInternalReplyInfo
			var err error

			var replyData []byte
			if reply.Result.Ok != nil {
				replyData = reply.Result.Ok.Data
			} else {
				replyData = data[0]
			}
			switch {
			case msg.Msg.Wasm.Execute != nil:
				sdkMsg := []sdk.Msg{&types.MsgExecuteContractResponse{}}
				err = proto.Unmarshal(replyData, sdkMsg[0])
				if err != nil {
					ctx.Logger().Error("Unmarshal MsgExecuteContractResponse", "proto", err.Error())
				}
				err = json.Unmarshal(sdkMsg[0].(*types.MsgExecuteContractResponse).GetData(), &dataWithInternalReplyInfo)
			case msg.Msg.Wasm.Instantiate != nil:
				sdkMsg := []sdk.Msg{&types.MsgInstantiateContractResponse{}}
				err = proto.Unmarshal(replyData, sdkMsg[0])
				if err != nil {
					ctx.Logger().Error("Unmarshal MsgInstantiateContractResponse", "proto", err.Error())
				}
				err = json.Unmarshal(sdkMsg[0].(*types.MsgInstantiateContractResponse).GetData(), &dataWithInternalReplyInfo)
			case msg.Msg.Wasm.Migrate != nil:
				sdkMsg := []sdk.Msg{&types.MsgMigrateContract{}}
				err = proto.Unmarshal(replyData, sdkMsg[0])
				if err != nil {
					ctx.Logger().Error("Unmarshal MsgMigrateContract", "proto", err.Error())
				}
				err = json.Unmarshal(sdkMsg[0].(*types.MsgMigrateContractResponse).GetData(), &dataWithInternalReplyInfo)
			// case msg.Msg.Wasm.UpdateAdmin != nil:
			// sdkMsg := []sdk.Msg{&types.MsgUpdateAdmin{}}
			// proto.Unmarshal(replyData, sdkMsg[0])
			// err = json.Unmarshal([]byte(sdkMsg[0].(*types.MsgUpdateAdminResponse).GetData()), &dataWithInternalReplyInfo)
			// break;
			// case msg.Msg.Wasm.ClearAdmin != nil:
			// sdkMsg := []sdk.Msg{&types.MsgClearAdmin{}}
			// proto.Unmarshal(replyData, sdkMsg[0])
			// err = json.Unmarshal([]byte(sdkMsg[0].(*types.MsgClearAdminResponse).GetData()), &dataWithInternalReplyInfo)
			// break;
			default:
			}
			if err != nil {
				return nil, fmt.Errorf("cannot serialize v1 DataWithInternalReplyInfo into json: %w", err)
			}
			if reply.Result.Ok != nil {
				reply.Result.Ok.Data = dataWithInternalReplyInfo.Data
			}

			if len(dataWithInternalReplyInfo.InternalMsgId) == 0 || len(dataWithInternalReplyInfo.InternaReplyEnclaveSig) == 0 {
				return nil, fmt.Errorf("when sending a reply both InternaReplyEnclaveSig and InternalMsgId are expected to be initialized")
			}

			replySigInfo = ogSigInfo
			reply.ID = dataWithInternalReplyInfo.InternalMsgId
			reply.IsEncrypted = true
			replySigInfo.CallbackSignature = dataWithInternalReplyInfo.InternaReplyEnclaveSig
		}

		rspData, err := d.keeper.reply(ctx, contractAddr, reply, ogTx, replySigInfo)
		switch {
		case err != nil:
			return nil, err
		case rspData != nil:
			rsp = rspData
		}
	}
	return rsp, nil
}
