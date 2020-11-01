package keeper

import (
	"encoding/json"
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

func EncodeGovMsg(sender sdk.AccAddress, msg *wasmTypes.GovMsg) ([]sdk.Msg, error) {
	if msg.Vote == nil {
		return nil, sdkerrors.Wrap(types.ErrInvalidMsg, "Unknown variant of Gov")
	}

	option, err := govtypes.VoteOptionFromString(msg.Vote.VoteOption)
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
	switch {
	case msg.Delegate != nil:
		coin, err := convertWasmCoinToSdkCoin(msg.Delegate.Amount)
		if err != nil {
			return nil, err
		}
		sdkMsg := stakingtypes.MsgDelegate{
			DelegatorAddress: sender.String(),
			ValidatorAddress: msg.Delegate.Validator,
			Amount:           coin,
		}
		return []sdk.Msg{&sdkMsg}, nil
	case msg.Redelegate != nil:
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
			rcpt = msg.Withdraw.Recipient
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

func (h MessageHandler) Dispatch(ctx sdk.Context, contractAddr sdk.AccAddress, msg wasmTypes.CosmosMsg) error {
	sdkMsgs, err := h.encoders.Encode(contractAddr, msg)
	if err != nil {
		return err
	}
	for _, sdkMsg := range sdkMsgs {
		if err := h.handleSdkMessage(ctx, contractAddr, sdkMsg); err != nil {
			return err
		}
	}
	return nil
}

func (h MessageHandler) handleSdkMessage(ctx sdk.Context, contractAddr sdk.Address, msg sdk.Msg) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// make sure this account can send it
	for _, acct := range msg.GetSigners() {
		if !acct.Equals(contractAddr) {
			return sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "contract doesn't have permission")
		}
	}

	// find the handler and execute it
	handler := h.router.Route(ctx, msg.Route())
	if handler == nil {
		return sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, msg.Route())
	}
	res, err := handler(ctx, msg)
	if err != nil {
		return err
	}

	events := make(sdk.Events, len(res.Events))
	for i := range res.Events {
		events[i] = sdk.Event(res.Events[i])
	}

	// redispatch all events, (type sdk.EventTypeMessage will be filtered out in the handler)
	ctx.EventManager().EmitEvents(events)

	return nil
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
