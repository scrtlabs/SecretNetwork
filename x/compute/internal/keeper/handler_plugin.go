package keeper

import (
	"encoding/json"
	"github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	wasmTypes "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

type MessageHandler struct {
	router   sdk.Router
	encoders MessageEncoders
}

func NewMessageHandler(router sdk.Router, customEncoders *MessageEncoders) MessageHandler {
	encoders := DefaultEncoders().Merge(customEncoders)
	return MessageHandler{
		router:   router,
		encoders: encoders,
	}
}

type BankEncoder func(sender sdk.AccAddress, msg *wasmTypes.BankMsg) ([]sdk.Msg, error)
type CustomEncoder func(sender sdk.AccAddress, msg json.RawMessage) ([]sdk.Msg, error)
type StakingEncoder func(sender sdk.AccAddress, msg *wasmTypes.StakingMsg) ([]sdk.Msg, error)
type WasmEncoder func(sender sdk.AccAddress, msg *wasmTypes.WasmMsg) ([]sdk.Msg, error)
type GovEncoder func(sender sdk.AccAddress, msg *wasmTypes.GovMsg) ([]sdk.Msg, error)

type MessageEncoders struct {
	Bank    BankEncoder
	Custom  CustomEncoder
	Staking StakingEncoder
	Wasm    WasmEncoder
	Gov     GovEncoder
}

func DefaultEncoders() MessageEncoders {
	return MessageEncoders{
		Bank:    EncodeBankMsg,
		Custom:  NoCustomMsg,
		Staking: EncodeStakingMsg,
		Wasm:    EncodeWasmMsg,
		Gov:     EncodeGovMsg,
	}
}

func (e MessageEncoders) Merge(o *MessageEncoders) MessageEncoders {
	if o == nil {
		return e
	}
	if o.Bank != nil {
		e.Bank = o.Bank
	}
	if o.Custom != nil {
		e.Custom = o.Custom
	}
	if o.Staking != nil {
		e.Staking = o.Staking
	}
	if o.Wasm != nil {
		e.Wasm = o.Wasm
	}
	if o.Gov != nil {
		e.Gov = o.Gov
	}
	return e
}

func (e MessageEncoders) Encode(contractAddr sdk.AccAddress, msg wasmTypes.CosmosMsg) ([]sdk.Msg, error) {
	switch {
	case msg.Bank != nil:
		return e.Bank(contractAddr, msg.Bank)
	case msg.Custom != nil:
		return e.Custom(contractAddr, msg.Custom)
	case msg.Staking != nil:
		return e.Staking(contractAddr, msg.Staking)
	case msg.Wasm != nil:
		return e.Wasm(contractAddr, msg.Wasm)
	case msg.Gov != nil:
		return e.Gov(contractAddr, msg.Gov)
	}

	return nil, sdkerrors.Wrap(types.ErrInvalidMsg, "Unknown variant of Wasm")
}

var VoteOptionMap = map[string]string{
	"Yes":        "VOTE_OPTION_YES",
	"Abstain":    "VOTE_OPTION_ABSTAIN",
	"No":         "VOTE_OPTION_NO",
	"NoWithVeto": "VOTE_OPTION_NO_WITH_VETO",
}

func EncodeGovMsg(sender sdk.AccAddress, msg *wasmTypes.GovMsg) ([]sdk.Msg, error) {
	if msg.Vote == nil {
		return nil, sdkerrors.Wrap(types.ErrInvalidMsg, "Unknown variant of Gov")
	}

	opt, exists := VoteOptionMap[msg.Vote.VoteOption]
	if !exists {
		// if it's not found, let the `VoteOptionFromString` below fail
		opt = msg.Vote.VoteOption
	}

	option, err := govtypes.VoteOptionFromString(opt)
	if err != nil {
		return nil, err
	}

	sdkMsg := govtypes.NewMsgVote(sender, msg.Vote.Proposal, option)
	return []sdk.Msg{sdkMsg}, nil
}

func EncodeBankMsg(sender sdk.AccAddress, msg *wasmTypes.BankMsg) ([]sdk.Msg, error) {
	if msg.Send == nil {
		return nil, sdkerrors.Wrap(types.ErrInvalidMsg, "Unknown variant of Bank")
	}
	if len(msg.Send.Amount) == 0 {
		return nil, nil
	}
	// validate that the addresses are valid
	_, stderr := sdk.AccAddressFromBech32(msg.Send.FromAddress)
	if stderr != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Send.FromAddress)
	}
	_, stderr = sdk.AccAddressFromBech32(msg.Send.ToAddress)
	if stderr != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Send.ToAddress)
	}

	toSend, err := convertWasmCoinsToSdkCoins(msg.Send.Amount)
	if err != nil {
		return nil, err
	}
	sdkMsg := banktypes.MsgSend{
		FromAddress: msg.Send.FromAddress,
		ToAddress:   msg.Send.ToAddress,
		Amount:      toSend,
	}
	return []sdk.Msg{&sdkMsg}, nil
}

func NoCustomMsg(sender sdk.AccAddress, msg json.RawMessage) ([]sdk.Msg, error) {
	return nil, sdkerrors.Wrap(types.ErrInvalidMsg, "Custom variant not supported")
}

func EncodeStakingMsg(sender sdk.AccAddress, msg *wasmTypes.StakingMsg) ([]sdk.Msg, error) {
	var err error
	switch {
	case msg.Delegate != nil:
		// Check that the address belongs to a validator.
		validator, err := sdk.ValAddressFromBech32(msg.Delegate.Validator)
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Delegate.Validator)
		}
		coin, err := convertWasmCoinToSdkCoin(msg.Delegate.Amount)
		if err != nil {
			return nil, err
		}
		//sdkMsg := stakingtypes.MsgDelegate{
		//	DelegatorAddress: sender.String(),
		//	ValidatorAddress: msg.Delegate.Validator,
		//	Amount:           coin,
		//}
		sdkMsg := stakingtypes.NewMsgDelegate(sender, validator, coin)
		return []sdk.Msg{sdkMsg}, nil

	case msg.Redelegate != nil:
		// Check that the addresses belong to validators.
		_, err = sdk.ValAddressFromBech32(msg.Redelegate.SrcValidator)
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Redelegate.SrcValidator)
		}
		_, err = sdk.ValAddressFromBech32(msg.Redelegate.DstValidator)
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Redelegate.DstValidator)
		}
		coin, err := convertWasmCoinToSdkCoin(msg.Redelegate.Amount)
		if err != nil {
			return nil, err
		}
		sdkMsg := stakingtypes.MsgBeginRedelegate{
			DelegatorAddress:    sender.String(),
			ValidatorSrcAddress: msg.Redelegate.SrcValidator,
			ValidatorDstAddress: msg.Redelegate.DstValidator,
			Amount:              coin,
		}
		return []sdk.Msg{&sdkMsg}, nil
	case msg.Undelegate != nil:
		// Check that the address belongs to a validator.
		_, err = sdk.ValAddressFromBech32(msg.Undelegate.Validator)
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Undelegate.Validator)
		}
		coin, err := convertWasmCoinToSdkCoin(msg.Undelegate.Amount)
		if err != nil {
			return nil, err
		}
		sdkMsg := stakingtypes.MsgUndelegate{
			DelegatorAddress: sender.String(),
			ValidatorAddress: msg.Undelegate.Validator,
			Amount:           coin,
		}
		return []sdk.Msg{&sdkMsg}, nil
	case msg.Withdraw != nil:
		senderAddr := sender.String()
		rcpt := senderAddr
		if len(msg.Withdraw.Recipient) != 0 {
			// Check that the address belongs to a real account.
			_, err = sdk.AccAddressFromBech32(msg.Withdraw.Recipient)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Withdraw.Recipient)
			}
			rcpt = msg.Withdraw.Recipient
		}
		// Check that the address belongs to a validator.
		_, err = sdk.ValAddressFromBech32(msg.Withdraw.Validator)
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Withdraw.Validator)
		}
		setMsg := distrtypes.MsgSetWithdrawAddress{
			DelegatorAddress: senderAddr,
			WithdrawAddress:  rcpt,
		}
		withdrawMsg := distrtypes.MsgWithdrawDelegatorReward{
			DelegatorAddress: senderAddr,
			ValidatorAddress: msg.Withdraw.Validator,
		}
		return []sdk.Msg{&setMsg, &withdrawMsg}, nil
	default:
		return nil, sdkerrors.Wrap(types.ErrInvalidMsg, "Unknown variant of Staking")
	}
}

func EncodeWasmMsg(sender sdk.AccAddress, msg *wasmTypes.WasmMsg) ([]sdk.Msg, error) {
	switch {
	case msg.Execute != nil:
		contractAddr, err := sdk.AccAddressFromBech32(msg.Execute.ContractAddr)
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, msg.Execute.ContractAddr)
		}
		coins, err := convertWasmCoinsToSdkCoins(msg.Execute.Send)
		if err != nil {
			return nil, err
		}

		sdkMsg := types.MsgExecuteContract{
			Sender:           sender,
			Contract:         contractAddr,
			CallbackCodeHash: msg.Execute.CallbackCodeHash,
			Msg:              msg.Execute.Msg,
			SentFunds:        coins,
			CallbackSig:      msg.Execute.CallbackSignature,
		}
		return []sdk.Msg{&sdkMsg}, nil
	case msg.Instantiate != nil:
		coins, err := convertWasmCoinsToSdkCoins(msg.Instantiate.Send)
		if err != nil {
			return nil, err
		}

		sdkMsg := types.MsgInstantiateContract{
			Sender: sender,
			CodeID: msg.Instantiate.CodeID,
			// TODO: add this to CosmWasm
			Label:            msg.Instantiate.Label,
			CallbackCodeHash: msg.Instantiate.CallbackCodeHash,
			InitMsg:          msg.Instantiate.Msg,
			InitFunds:        coins,
			CallbackSig:      msg.Instantiate.CallbackSignature,
		}
		return []sdk.Msg{&sdkMsg}, nil
	default:
		return nil, sdkerrors.Wrap(types.ErrInvalidMsg, "Unknown variant of Wasm")
	}
}

func (k Keeper) Dispatch(ctx sdk.Context, contractAddr sdk.AccAddress, msg wasmTypes.CosmosMsg) (events sdk.Events, data []byte, err error) {

	sdkMsgs, err := k.messenger.encoders.Encode(contractAddr, msg)
	if err != nil {
		return nil, nil, err
	}
	for _, sdkMsg := range sdkMsgs {
		_, _, err := k.handleSdkMessage(ctx, contractAddr, sdkMsg)
		if err != nil {
			return nil, nil, err
		}
		//return sdkEvents, msgData, err
	}
	return nil, nil, nil
}

func (k Keeper) handleSdkMessage(ctx sdk.Context, contractAddr sdk.Address, msg sdk.Msg) (sdk.Events, []byte, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, nil, err
	}

	// make sure this account can send it
	for _, acct := range msg.GetSigners() {
		if !acct.Equals(contractAddr) {
			return nil, nil, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "contract doesn't have permission")
		}
	}

	var res *sdk.Result
	var err error
	if legacyMsg, ok := msg.(legacytx.LegacyMsg); ok {
		msgRoute := legacyMsg.Route()
		handler := k.messenger.router.Route(ctx, msgRoute)
		if handler == nil {
			return nil, nil, sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized message route: %s", msgRoute)
		}

		res, err = handler(ctx, msg)
		if err != nil {
			return nil, nil, err
		}
	} else {
		return nil, nil, sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized legacy message route: %s", sdk.MsgTypeURL(msg))

		// todo: grpc routing
		//handler := k.serviceRouter.Handler(msg)
		//if handler == nil {
		//	return nil, nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, sdk.MsgTypeURL(msg))
		//}
		//res, err := handler(ctx, msg)
		//if err != nil {
		//	return nil, nil, err
		//}
	}

	// todo: remove this when adding submessages
	events := make(sdk.Events, len(res.Events))
	for i := range res.Events {
		events[i] = sdk.Event(res.Events[i])
	}
	//
	// redispatch all events, (type sdk.EventTypeMessage will be filtered out in the handler)
	ctx.EventManager().EmitEvents(events)

	// todo: add this when adding submessages
	//data = make([]byte, len(res.Data))
	//copy(data, res.Data)
	//
	//// convert Tendermint.Events to sdk.Event
	//sdkEvents := make(sdk.Events, len(res.Events))
	//for i := range res.Events {
	//	sdkEvents[i] = sdk.Event(res.Events[i])
	//}

	// append message action attribute
	//events = append(events, sdkEvents...)

	return nil, nil, nil
}

func convertWasmCoinsToSdkCoins(coins []wasmTypes.Coin) (sdk.Coins, error) {
	var toSend sdk.Coins
	for _, coin := range coins {
		c, err := convertWasmCoinToSdkCoin(coin)
		if err != nil {
			return nil, err
		}
		toSend = append(toSend, c)
	}
	return toSend, nil
}

func convertWasmCoinToSdkCoin(coin wasmTypes.Coin) (sdk.Coin, error) {
	amount, ok := sdk.NewIntFromString(coin.Amount)
	if !ok {
		return sdk.Coin{}, sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, coin.Amount+coin.Denom)
	}
	return sdk.Coin{
		Denom:  coin.Denom,
		Amount: amount,
	}, nil
}
