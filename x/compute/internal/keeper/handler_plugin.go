package keeper

import (
	"encoding/json"
	"errors"
	"fmt"

	"cosmossdk.io/math"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"

	ibcclienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	channelkeeper "github.com/cosmos/ibc-go/v8/modules/core/04-channel/keeper"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"

	host "github.com/cosmos/ibc-go/v8/modules/core/24-host"
	v1wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	v010wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"

	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

// MessageHandlerChain defines a chain of handlers that are called one by one until it can be handled.
type MessageHandlerChain struct {
	handlers []Messenger
}

// SDKMessageHandler can handles messages that can be encoded into sdk.Message types and routed.
type SDKMessageHandler struct {
	router   MessageRouter
	encoders MessageEncoders
}

func NewSDKMessageHandler(router MessageRouter /*legacyRouter sdk.Router,*/, encoders MessageEncoders) SDKMessageHandler {
	return SDKMessageHandler{
		router:   router,
		encoders: encoders,
	}
}

// IBCRawPacketHandler handels IBC.SendPacket messages which are published to an IBC channel.
type IBCRawPacketHandler struct {
	channelKeeper    channelkeeper.Keeper
	ics4Wrapper      porttypes.ICS4Wrapper
	capabilityKeeper capabilitykeeper.ScopedKeeper
}

func NewIBCRawPacketHandler(channelKeeper channelkeeper.Keeper, ics4Wrapper porttypes.ICS4Wrapper, capabilityKeeper capabilitykeeper.ScopedKeeper) IBCRawPacketHandler {
	return IBCRawPacketHandler{
		channelKeeper:    channelKeeper,
		ics4Wrapper:      ics4Wrapper,
		capabilityKeeper: capabilityKeeper,
	}
}

func NewMessageHandlerChain(first Messenger, others ...Messenger) *MessageHandlerChain {
	r := &MessageHandlerChain{handlers: append([]Messenger{first}, others...)}
	for i := range r.handlers {
		if r.handlers[i] == nil {
			panic(fmt.Sprintf("handler must not be nil at position : %d", i))
		}
	}
	return r
}

func NewMessageHandler(
	msgRouter MessageRouter,
	customEncoders *MessageEncoders,
	channelKeeper channelkeeper.Keeper,
	ics4Wrapper porttypes.ICS4Wrapper,
	capabilityKeeper capabilitykeeper.ScopedKeeper,
	portSource types.ICS20TransferPortSource,
	unpacker codectypes.AnyUnpacker,
) Messenger {
	encoders := DefaultEncoders(portSource, unpacker).Merge(customEncoders)
	return NewMessageHandlerChain(
		NewSDKMessageHandler(msgRouter, encoders),
		NewIBCRawPacketHandler(channelKeeper, ics4Wrapper, capabilityKeeper),
	)
}

// DispatchMsg dispatch message and calls chained handlers one after another in
// order to find the right one to process given message. If a handler cannot
// process given message (returns ErrUnknownMsg), its result is ignored and the
// next handler is executed.
func (m MessageHandlerChain) DispatchMsg(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg v1wasmTypes.CosmosMsg) ([]sdk.Event, [][]byte, error) {
	for _, h := range m.handlers {
		events, data, err := h.DispatchMsg(ctx, contractAddr, contractIBCPortID, msg)
		switch {
		case err == nil:
			return events, data, nil
		case errors.Is(err, types.ErrUnknownMsg):
			continue
		default:
			return events, data, err
		}
	}
	return nil, nil, errorsmod.Wrap(types.ErrUnknownMsg, "no handler found")
}

// DispatchMsg publishes a raw IBC packet onto the channel.
func (h IBCRawPacketHandler) DispatchMsg(ctx sdk.Context, _ sdk.AccAddress, contractIBCPortID string, msg v1wasmTypes.CosmosMsg) (events []sdk.Event, data [][]byte, err error) {
	if msg.IBC == nil || msg.IBC.SendPacket == nil {
		return nil, nil, types.ErrUnknownMsg
	}

	if contractIBCPortID == "" {
		return nil, nil, errorsmod.Wrapf(types.ErrUnsupportedForContract, "ibc not supported")
	}
	contractIBCChannelID := msg.IBC.SendPacket.ChannelID
	if contractIBCChannelID == "" {
		return nil, nil, errorsmod.Wrapf(types.ErrEmpty, "ibc channel")
	}

	_, found := h.channelKeeper.GetNextSequenceSend(ctx, contractIBCPortID, contractIBCChannelID)
	if !found {
		return nil, nil, errorsmod.Wrapf(channeltypes.ErrSequenceSendNotFound,
			"source port: %s, source channel: %s", contractIBCPortID, contractIBCChannelID,
		)
	}

	channelCap, ok := h.capabilityKeeper.GetCapability(ctx, host.ChannelCapabilityPath(contractIBCPortID, contractIBCChannelID))
	if !ok {
		return nil, nil, errorsmod.Wrap(channeltypes.ErrChannelCapabilityNotFound, "module does not own channel capability")
	}
	_, err = h.ics4Wrapper.SendPacket(ctx, channelCap, contractIBCPortID, contractIBCChannelID, convertWasmIBCTimeoutHeightToCosmosHeight(msg.IBC.SendPacket.Timeout.Block), msg.IBC.SendPacket.Timeout.Timestamp, msg.IBC.SendPacket.Data)
	return nil, nil, err
}

type (
	BankEncoder         func(sender sdk.AccAddress, msg *v1wasmTypes.BankMsg) ([]sdk.Msg, error)
	CustomEncoder       func(sender sdk.AccAddress, msg json.RawMessage) ([]sdk.Msg, error)
	DistributionEncoder func(sender sdk.AccAddress, msg *v1wasmTypes.DistributionMsg) ([]sdk.Msg, error)
	GovEncoder          func(sender sdk.AccAddress, msg *v1wasmTypes.GovMsg) ([]sdk.Msg, error)
	IBCEncoder          func(ctx sdk.Context, sender sdk.AccAddress, contractIBCPortID string, msg *v1wasmTypes.IBCMsg) ([]sdk.Msg, error)
	StakingEncoder      func(sender sdk.AccAddress, msg *v1wasmTypes.StakingMsg) ([]sdk.Msg, error)
	StargateEncoder     func(sender sdk.AccAddress, msg *v1wasmTypes.StargateMsg) ([]sdk.Msg, error)
	WasmEncoder         func(sender sdk.AccAddress, msg *v1wasmTypes.WasmMsg) ([]sdk.Msg, error)
)

type MessageEncoders struct {
	Bank         BankEncoder
	Custom       CustomEncoder
	Distribution DistributionEncoder
	Gov          GovEncoder
	IBC          IBCEncoder
	Staking      StakingEncoder
	Stargate     StargateEncoder
	Wasm         WasmEncoder
}

func DefaultEncoders(portSource types.ICS20TransferPortSource, unpacker codectypes.AnyUnpacker) MessageEncoders {
	return MessageEncoders{
		Bank:         EncodeBankMsg,
		Custom:       NoCustomMsg,
		Distribution: EncodeDistributionMsg,
		Gov:          EncodeGovMsg,
		IBC:          EncodeIBCMsg(portSource),
		Staking:      EncodeStakingMsg,
		Stargate:     EncodeStargateMsg(unpacker),
		Wasm:         EncodeWasmMsg,
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

func isValidV010Msg(msg v010wasmTypes.CosmosMsg) bool {
	count := 0
	if msg.Bank != nil {
		if msg.Bank.Send != nil {
			count++
		}
	}
	if msg.Custom != nil {
		count++
	}
	if msg.Staking != nil {
		if msg.Staking.Delegate != nil {
			count++
		}
		if msg.Staking.Undelegate != nil {
			count++
		}
		if msg.Staking.Redelegate != nil {
			count++
		}
		if msg.Staking.Withdraw != nil {
			count++
		}
	}
	if msg.Wasm != nil {
		if msg.Wasm.Execute != nil {
			count++
		}
		if msg.Wasm.Instantiate != nil {
			count++
		}
	}
	if msg.Gov != nil {
		if msg.Gov.Vote != nil {
			count++
		}
	}

	return count == 1
}

func (e MessageEncoders) Encode(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg v1wasmTypes.CosmosMsg) ([]sdk.Msg, error) {
	switch {
	case msg.Bank != nil:
		return e.Bank(contractAddr, msg.Bank)
	case msg.Custom != nil:
		return e.Custom(contractAddr, msg.Custom)
	case msg.Distribution != nil:
		return e.Distribution(contractAddr, msg.Distribution)
	case msg.Gov != nil:
		return e.Gov(contractAddr, msg.Gov)
	case msg.IBC != nil:
		return e.IBC(ctx, contractAddr, contractIBCPortID, msg.IBC)
	case msg.Staking != nil:
		return e.Staking(contractAddr, msg.Staking)
	case msg.Stargate != nil:
		return e.Stargate(contractAddr, msg.Stargate)
	case msg.Wasm != nil:
		return e.Wasm(contractAddr, msg.Wasm)
	}

	return nil, errorsmod.Wrap(types.ErrInvalidMsg, "Unknown variant of Wasm")
}

var VoteOptionMap = map[v1wasmTypes.VoteOption]string{
	v1wasmTypes.Yes:        "VOTE_OPTION_YES",
	v1wasmTypes.Abstain:    "VOTE_OPTION_ABSTAIN",
	v1wasmTypes.No:         "VOTE_OPTION_NO",
	v1wasmTypes.NoWithVeto: "VOTE_OPTION_NO_WITH_VETO",
}

func EncodeGovMsg(sender sdk.AccAddress, msg *v1wasmTypes.GovMsg) ([]sdk.Msg, error) {
	if msg.Vote == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidMsg, "Unknown variant of Gov")
	}

	opt, exists := VoteOptionMap[msg.Vote.Vote]
	if !exists {
		// if it's not found, let the `VoteOptionFromString` below fail
		opt = ""
	}

	option, err := govtypes.VoteOptionFromString(opt)
	if err != nil {
		return nil, err
	}

	sdkMsg := govtypes.NewMsgVote(sender, msg.Vote.ProposalId, option, "")
	return []sdk.Msg{sdkMsg}, nil
}

func EncodeIBCMsg(portSource types.ICS20TransferPortSource) func(ctx sdk.Context, sender sdk.AccAddress, contractIBCPortID string, msg *v1wasmTypes.IBCMsg) ([]sdk.Msg, error) {
	return func(ctx sdk.Context, sender sdk.AccAddress, _ string, msg *v1wasmTypes.IBCMsg) ([]sdk.Msg, error) {
		// param contractIBCPortID unused - replace with _
		switch {
		case msg.CloseChannel != nil:
			return []sdk.Msg{&channeltypes.MsgChannelCloseInit{
				PortId:    PortIDForContract(sender),
				ChannelId: msg.CloseChannel.ChannelID,
				Signer:    sender.String(),
			}}, nil
		case msg.Transfer != nil:
			amount, err := convertWasmCoinToSdkCoin(msg.Transfer.Amount)
			if err != nil {
				return nil, errorsmod.Wrap(err, "amount")
			}
			msg := &ibctransfertypes.MsgTransfer{
				SourcePort:       portSource.GetPort(ctx),
				SourceChannel:    msg.Transfer.ChannelID,
				Token:            amount,
				Sender:           sender.String(),
				Receiver:         msg.Transfer.ToAddress,
				TimeoutHeight:    convertWasmIBCTimeoutHeightToCosmosHeight(msg.Transfer.Timeout.Block),
				TimeoutTimestamp: msg.Transfer.Timeout.Timestamp,
				Memo:             msg.Transfer.Memo,
			}
			return []sdk.Msg{msg}, nil
		default:
			return nil, errorsmod.Wrap(types.ErrUnknownMsg, "Unknown variant of IBC")
		}
	}
}

func EncodeBankMsg(sender sdk.AccAddress, msg *v1wasmTypes.BankMsg) ([]sdk.Msg, error) {
	if msg.Send == nil {
		return nil, errorsmod.Wrap(types.ErrInvalidMsg, "Unknown variant of Bank")
	}
	if len(msg.Send.Amount) == 0 {
		return nil, nil
	}
	// validate that the addresses are valid
	_, stderr := sdk.AccAddressFromBech32(msg.Send.ToAddress)
	if stderr != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrap(msg.Send.ToAddress)
	}

	toSend, err := convertWasmCoinsToSdkCoins(msg.Send.Amount)
	if err != nil {
		return nil, err
	}
	sdkMsg := banktypes.MsgSend{
		FromAddress: sender.String(),
		ToAddress:   msg.Send.ToAddress,
		Amount:      toSend,
	}
	return []sdk.Msg{&sdkMsg}, nil
}

func NoCustomMsg(_ sdk.AccAddress, _ json.RawMessage) ([]sdk.Msg, error) {
	return nil, errorsmod.Wrap(types.ErrInvalidMsg, "Custom variant not supported")
}

func EncodeDistributionMsg(sender sdk.AccAddress, msg *v1wasmTypes.DistributionMsg) ([]sdk.Msg, error) {
	switch {
	case msg.SetWithdrawAddress != nil:
		setMsg := distrtypes.MsgSetWithdrawAddress{
			DelegatorAddress: sender.String(),
			WithdrawAddress:  msg.SetWithdrawAddress.Address,
		}
		return []sdk.Msg{&setMsg}, nil
	case msg.WithdrawDelegatorReward != nil:
		withdrawMsg := distrtypes.MsgWithdrawDelegatorReward{
			DelegatorAddress: sender.String(),
			ValidatorAddress: msg.WithdrawDelegatorReward.Validator,
		}
		return []sdk.Msg{&withdrawMsg}, nil
	default:
		return nil, errorsmod.Wrap(types.ErrUnknownMsg, "unknown variant of Distribution")
	}
}

func EncodeStakingMsg(sender sdk.AccAddress, msg *v1wasmTypes.StakingMsg) ([]sdk.Msg, error) {
	var err error
	switch {
	case msg.Delegate != nil:
		// Check that the address belongs to a validator.
		validator, err := sdk.ValAddressFromBech32(msg.Delegate.Validator)
		if err != nil {
			return nil, sdkerrors.ErrInvalidAddress.Wrap(msg.Delegate.Validator)
		}
		coin, err := convertWasmCoinToSdkCoin(msg.Delegate.Amount)
		if err != nil {
			return nil, err
		}
		// sdkMsg := stakingtypes.MsgDelegate{
		//	DelegatorAddress: sender.String(),
		//	ValidatorAddress: msg.Delegate.Validator,
		//	Amount:           coin,
		//}
		sdkMsg := stakingtypes.NewMsgDelegate(sender.String(), validator.String(), coin)
		return []sdk.Msg{sdkMsg}, nil

	case msg.Redelegate != nil:
		// Check that the addresses belong to validators.
		_, err = sdk.ValAddressFromBech32(msg.Redelegate.SrcValidator)
		if err != nil {
			return nil, sdkerrors.ErrInvalidAddress.Wrap(msg.Redelegate.SrcValidator)
		}
		_, err = sdk.ValAddressFromBech32(msg.Redelegate.DstValidator)
		if err != nil {
			return nil, sdkerrors.ErrInvalidAddress.Wrap(msg.Redelegate.DstValidator)
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
			return nil, sdkerrors.ErrInvalidAddress.Wrap(msg.Undelegate.Validator)
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
				return nil, sdkerrors.ErrInvalidAddress.Wrap(msg.Withdraw.Recipient)
			}
			rcpt = msg.Withdraw.Recipient
		}
		// Check that the address belongs to a validator.
		_, err = sdk.ValAddressFromBech32(msg.Withdraw.Validator)
		if err != nil {
			return nil, sdkerrors.ErrInvalidAddress.Wrap(msg.Withdraw.Validator)
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
		return nil, errorsmod.Wrap(types.ErrInvalidMsg, "Unknown variant of Staking")
	}
}

func EncodeStargateMsg(unpacker codectypes.AnyUnpacker) StargateEncoder {
	return func(_ sdk.AccAddress, msg *v1wasmTypes.StargateMsg) ([]sdk.Msg, error) {
		anyObj := codectypes.Any{
			TypeUrl: msg.TypeURL,
			Value:   msg.Value,
		}
		var sdkMsg sdk.Msg
		if err := unpacker.UnpackAny(&anyObj, &sdkMsg); err != nil {
			return nil, errorsmod.Wrap(types.ErrInvalidMsg, fmt.Sprintf("Cannot unpack proto message with type URL: %s", msg.TypeURL))
		}
		if err := codectypes.UnpackInterfaces(sdkMsg, unpacker); err != nil {
			return nil, errorsmod.Wrap(types.ErrInvalidMsg, fmt.Sprintf("UnpackInterfaces inside msg: %s", err))
		}
		return []sdk.Msg{sdkMsg}, nil
	}
}

func EncodeWasmMsg(sender sdk.AccAddress, msg *v1wasmTypes.WasmMsg) ([]sdk.Msg, error) {
	switch {
	case msg.Execute != nil:
		contractAddr, err := sdk.AccAddressFromBech32(msg.Execute.ContractAddr)
		if err != nil {
			return nil, sdkerrors.ErrInvalidAddress.Wrap(msg.Execute.ContractAddr)
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
			Sender:           sender,
			CodeID:           msg.Instantiate.CodeID,
			Label:            msg.Instantiate.Label,
			CallbackCodeHash: msg.Instantiate.CallbackCodeHash,
			InitMsg:          msg.Instantiate.Msg,
			InitFunds:        coins,
			CallbackSig:      msg.Instantiate.CallbackSignature,
			Admin:            msg.Instantiate.Admin,
		}
		return []sdk.Msg{&sdkMsg}, nil
	case msg.Migrate != nil:
		sdkMsg := types.MsgMigrateContract{
			Sender:           sender.String(),
			Contract:         msg.Migrate.Contract,
			CodeID:           msg.Migrate.CodeID,
			Msg:              msg.Migrate.Msg,
			CallbackSig:      msg.Migrate.CallbackSignature,
			CallbackCodeHash: msg.Migrate.CallbackCodeHash,
		}
		return []sdk.Msg{&sdkMsg}, nil
	case msg.UpdateAdmin != nil:
		sdkMsg := types.MsgUpdateAdmin{
			Sender:      sender.String(),
			Contract:    msg.UpdateAdmin.Contract,
			NewAdmin:    msg.UpdateAdmin.NewAdmin,
			CallbackSig: msg.UpdateAdmin.CallbackSignature,
		}
		return []sdk.Msg{&sdkMsg}, nil
	case msg.ClearAdmin != nil:
		sdkMsg := types.MsgClearAdmin{
			Sender:      sender.String(),
			Contract:    msg.ClearAdmin.Contract,
			CallbackSig: msg.ClearAdmin.CallbackSignature,
		}
		return []sdk.Msg{&sdkMsg}, nil
	default:
		return nil, errorsmod.Wrap(types.ErrInvalidMsg, "Unknown variant of Wasm")
	}
}

func (h SDKMessageHandler) DispatchMsg(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg v1wasmTypes.CosmosMsg) ([]sdk.Event, [][]byte, error) {
	sdkMsgs, err := h.encoders.Encode(ctx, contractAddr, contractIBCPortID, msg)
	if err != nil {
		return nil, nil, err
	}

	var (
		events []sdk.Event
		data   [][]byte
	)
	for _, sdkMsg := range sdkMsgs {
		res, err := h.handleSdkMessage(ctx, sdkMsg)
		if err != nil {
			if res != nil {
				data = append(data, res.Data)
			}
			return nil, data, err
		}
		// append data
		data = append(data, res.Data)

		// append events
		sdkEvents := make([]sdk.Event, len(res.Events))
		for i := range res.Events {
			sdkEvents[i] = sdk.Event(res.Events[i])
		}
		events = append(events, sdkEvents...)
	}

	return events, data, nil
}

func (h SDKMessageHandler) handleSdkMessage(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
	// find the handler and execute it
	if handler := h.router.Handler(msg); handler != nil {
		// ADR 031 request type routing
		return handler(ctx, msg)
	}

	// Assuming that the app developer has migrated all their Msgs to
	// proto messages and has registered all `Msg services`, then this
	// path should never be called, because all those Msgs should be
	// registered within the `msgServiceRouter` already.
	return nil, sdkerrors.ErrUnknownRequest.Wrapf("can't route message %+v", msg)
}

// callDepthMessageHandler is a wrapper around a Messenger that checks the call depth before dispatching a message.
type callDepthMessageHandler struct {
	Messenger
	MaxCallDepth uint32
}

func (h callDepthMessageHandler) DispatchMsg(ctx sdk.Context, contractAddr sdk.AccAddress, contractIBCPortID string, msg v1wasmTypes.CosmosMsg) (events []sdk.Event, data [][]byte, err error) {
	ctx, err = checkAndIncreaseCallDepth(ctx, h.MaxCallDepth)
	if err != nil {
		return nil, nil, err
	}

	return h.Messenger.DispatchMsg(ctx, contractAddr, contractIBCPortID, msg)
}

// convertWasmIBCTimeoutHeightToCosmosHeight converts a wasm type ibc timeout height to ibc module type height
func convertWasmIBCTimeoutHeightToCosmosHeight(ibcTimeoutBlock *v1wasmTypes.IBCTimeoutBlock) ibcclienttypes.Height {
	if ibcTimeoutBlock == nil {
		return ibcclienttypes.NewHeight(0, 0)
	}
	return ibcclienttypes.NewHeight(ibcTimeoutBlock.Revision, ibcTimeoutBlock.Height)
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
	amount, ok := math.NewIntFromString(coin.Amount)
	if !ok {
		return sdk.Coin{}, sdkerrors.ErrInvalidCoins.Wrap(coin.Amount + coin.Denom)
	}
	return sdk.Coin{
		Denom:  coin.Denom,
		Amount: amount,
	}, nil
}
