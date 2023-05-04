package keeper

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdktxsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	v1types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"

	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

var _ types.IBCContractKeeper = (*Keeper)(nil)

func (k Keeper) ibcContractCall(ctx sdk.Context,
	contractAddress sdk.AccAddress,
	msgBz []byte,
	callType wasmTypes.HandleType,
) (interface{}, error) {
	signBytes, signMode, modeInfoBytes, pkBytes, signerSig, err := k.GetTxInfo(ctx, nil)
	if err != nil {
		return nil, err
	}

	verificationInfo := types.NewVerificationInfo(signBytes, signMode, modeInfoBytes, pkBytes, signerSig, nil)

	_, codeInfo, prefixStore, err := k.contractInstance(ctx, contractAddress)
	if err != nil {
		return "", err
	}

	store := ctx.KVStore(k.storeKey)

	contractKey := store.Get(types.GetContractEnclaveKey(contractAddress))
	random := store.Get(types.GetRandomKey(ctx.BlockHeight()))
	env := types.NewEnv(
		ctx,
		sdk.AccAddress{}, /* there's no MessageInfo for IBC contract calls */
		sdk.NewCoins(),   /* there's no MessageInfo for IBC contract calls */
		contractAddress,
		contractKey,
		random,
	)

	// prepare querier
	querier := QueryHandler{
		Ctx:     ctx,
		Plugins: k.queryPlugins,
	}

	gas := gasForContract(ctx)
	res, gasUsed, err := k.wasmer.Execute(codeInfo.CodeHash, env, msgBz, prefixStore, cosmwasmAPI, querier, ctx.GasMeter(), gas, verificationInfo, callType)
	consumeGas(ctx, gasUsed)

	return res, err
}

func (k Keeper) parseThenHandleIBCBasicContractResponse(ctx sdk.Context,
	contractAddress sdk.AccAddress,
	inputMsg []byte,
	res interface{},
) error {
	switch resp := res.(type) {
	case *v1types.IBCBasicResponse:
		if resp != nil {
			contractInfo, _, _, err := k.contractInstance(ctx, contractAddress)
			if err != nil {
				return err
			}

			return k.handleIBCBasicContractResponse(ctx, contractAddress, contractInfo.IBCPortID, inputMsg, resp)
		}

		return sdkerrors.Wrap(types.ErrExecuteFailed, fmt.Sprintf("null pointer IBCBasicResponse: %+v", res))
	default:
		return sdkerrors.Wrap(types.ErrExecuteFailed, fmt.Sprintf("cannot cast res to IBCBasicResponse: %+v", res))
	}
}

// OnOpenChannel calls the contract to participate in the IBC channel handshake step.
// In the IBC protocol this is either the `Channel Open Init` event on the initiating chain or
// `Channel Open Try` on the counterparty chain.
// Protocol version and channel ordering should be verified for example.
// See https://github.com/cosmos/ics/tree/master/spec/ics-004-channel-and-packet-semantics#channel-lifecycle-management
func (k Keeper) OnOpenChannel(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	msg v1types.IBCChannelOpenMsg,
) (string, error) {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "ibc-open-channel")

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading Compute module: ibc-open-channel")

	msgBz, err := json.Marshal(msg)
	if err != nil {
		return "", sdkerrors.Wrap(err, "ibc-open-channel")
	}

	res, err := k.ibcContractCall(ctx, contractAddress, msgBz, wasmTypes.HandleTypeIbcChannelOpen)
	if err != nil {
		return "", sdkerrors.Wrap(types.ErrExecuteFailed, err.Error())
	}

	switch resp := res.(type) {
	case *string:
		return *resp, nil
	default:
		return "", sdkerrors.Wrap(types.ErrExecuteFailed, fmt.Sprintf("ibc-open-channel: cannot cast res to IBC3ChannelOpenResponse: %+v", res))
	}
}

// OnConnectChannel calls the contract to let it know the IBC channel was established.
// In the IBC protocol this is either the `Channel Open Ack` event on the initiating chain or
// `Channel Open Confirm` on the counterparty chain.
//
// There is an open issue with the [cosmos-sdk](https://github.com/cosmos/cosmos-sdk/issues/8334)
// that the counterparty channelID is empty on the initiating chain
// See https://github.com/cosmos/ics/tree/master/spec/ics-004-channel-and-packet-semantics#channel-lifecycle-management
func (k Keeper) OnConnectChannel(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	msg v1types.IBCChannelConnectMsg,
) error {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "ibc-connect-channel")

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading Compute module: ibc-connect-channel")

	msgBz, err := json.Marshal(msg)
	if err != nil {
		return sdkerrors.Wrap(err, "ibc-connect-channel")
	}

	res, err := k.ibcContractCall(ctx, contractAddress, msgBz, wasmTypes.HandleTypeIbcChannelConnect)
	if err != nil {
		return sdkerrors.Wrap(types.ErrExecuteFailed, err.Error())
	}
	err = k.parseThenHandleIBCBasicContractResponse(ctx, contractAddress, msgBz, res)
	if err != nil {
		return sdkerrors.Wrap(err, "ibc-connect-channel")
	}
	return nil
}

// OnCloseChannel calls the contract to let it know the IBC channel is closed.
// Calling modules MAY atomically execute appropriate application logic in conjunction with calling chanCloseConfirm.
//
// Once closed, channels cannot be reopened and identifiers cannot be reused. Identifier reuse is prevented because
// we want to prevent potential replay of previously sent packets
// See https://github.com/cosmos/ics/tree/master/spec/ics-004-channel-and-packet-semantics#channel-lifecycle-management
func (k Keeper) OnCloseChannel(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	msg v1types.IBCChannelCloseMsg,
) error {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "ibc-close-channel")

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading Compute module: ibc-close-channel")

	msgBz, err := json.Marshal(msg)
	if err != nil {
		return sdkerrors.Wrap(err, "ibc-close-channel")
	}

	res, err := k.ibcContractCall(ctx, contractAddress, msgBz, wasmTypes.HandleTypeIbcChannelClose)
	if err != nil {
		return sdkerrors.Wrap(types.ErrExecuteFailed, err.Error())
	}

	err = k.parseThenHandleIBCBasicContractResponse(ctx, contractAddress, msgBz, res)
	if err != nil {
		return sdkerrors.Wrap(err, "ibc-close-channel")
	}
	return nil
}

// OnRecvPacket calls the contract to process the incoming IBC packet. The contract fully owns the data processing and
// returns the acknowledgement data for the chain level. This allows custom applications and protocols on top
// of IBC. Although it is recommended to use the standard acknowledgement envelope defined in
// https://github.com/cosmos/ics/tree/master/spec/ics-004-channel-and-packet-semantics#acknowledgement-envelope
//
// For more information see: https://github.com/cosmos/ics/tree/master/spec/ics-004-channel-and-packet-semantics#packet-flow--handling
func (k Keeper) OnRecvPacket(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	msg v1types.IBCPacketReceiveMsg,
) ([]byte, error) {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "ibc-recv-packet")

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading Compute module: ibc-recv-packet")

	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "ibc-recv-packet")
	}

	res, err := k.ibcContractCall(ctx, contractAddress, msgBz, wasmTypes.HandleTypeIbcPacketReceive)
	if err != nil {
		return nil, sdkerrors.Wrap(types.ErrExecuteFailed, err.Error())
	}

	switch resp := res.(type) {
	case *v1types.IBCReceiveResponse:
		if resp != nil {
			contractInfo, _, _, err := k.contractInstance(ctx, contractAddress)
			if err != nil {
				return nil, err
			}
			verificationInfo := types.NewVerificationInfo([]byte{}, sdktxsigning.SignMode_SIGN_MODE_DIRECT, []byte{}, []byte{}, []byte{}, nil)

			ogTx := msg.Packet.Data

			// If the data contains less than 64 bytes (means plaintext)
			// use the whole message just for compilation
			if len(ogTx) < 64 {
				ogTx = msgBz
			}

			// note submessage reply results can overwrite the `Acknowledgement` data
			return k.handleContractResponse(ctx, contractAddress, contractInfo.IBCPortID, resp.Messages, resp.Attributes, resp.Events, resp.Acknowledgement, ogTx, verificationInfo, wasmTypes.CosmosMsgVersionV1)
		}

		// should never get here as it's already checked in
		// https://github.com/scrtlabs/SecretNetwork/blob/bd46776c/go-cosmwasm/lib.go#L358
		return nil, sdkerrors.Wrap(types.ErrExecuteFailed, fmt.Sprintf("ibc-recv-packet: null pointer IBCReceiveResponse: %+v", res))
	default:
		return nil, sdkerrors.Wrap(types.ErrExecuteFailed, fmt.Sprintf("ibc-recv-packet: cannot cast res to IBCReceiveResponse: %+v", res))
	}
}

// OnAckPacket calls the contract to handle the "acknowledgement" data which can contain success or failure of a packet
// acknowledgement written on the receiving chain for example. This is application level data and fully owned by the
// contract. The use of the standard acknowledgement envelope is recommended: https://github.com/cosmos/ics/tree/master/spec/ics-004-channel-and-packet-semantics#acknowledgement-envelope
//
// On application errors the contract can revert an operation like returning tokens as in ibc-transfer.
//
// For more information see: https://github.com/cosmos/ics/tree/master/spec/ics-004-channel-and-packet-semantics#packet-flow--handling
func (k Keeper) OnAckPacket(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	msg v1types.IBCPacketAckMsg,
) error {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "ibc-ack-packet")

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading Compute module: ibc-ack-packet")

	msgBz, err := json.Marshal(msg)
	if err != nil {
		return sdkerrors.Wrap(err, "ibc-ack-packet")
	}

	res, err := k.ibcContractCall(ctx, contractAddress, msgBz, wasmTypes.HandleTypeIbcPacketAck)
	if err != nil {
		return sdkerrors.Wrap(types.ErrExecuteFailed, err.Error())
	}

	err = k.parseThenHandleIBCBasicContractResponse(ctx, contractAddress, msgBz, res)
	if err != nil {
		return sdkerrors.Wrap(err, "ibc-ack-packet")
	}
	return nil
}

// OnTimeoutPacket calls the contract to let it know the packet was never received on the destination chain within
// the timeout boundaries.
// The contract should handle this on the application level and undo the original operation
func (k Keeper) OnTimeoutPacket(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	msg v1types.IBCPacketTimeoutMsg,
) error {
	defer telemetry.MeasureSince(time.Now(), "compute", "keeper", "ibc-timeout-packet")

	ctx.GasMeter().ConsumeGas(types.InstanceCost, "Loading Compute module: ibc-timeout-packet")

	msgBz, err := json.Marshal(msg)
	if err != nil {
		return sdkerrors.Wrap(err, "ibc-timeout-packet")
	}

	res, err := k.ibcContractCall(ctx, contractAddress, msgBz, wasmTypes.HandleTypeIbcPacketTimeout)
	if err != nil {
		return sdkerrors.Wrap(types.ErrExecuteFailed, err.Error())
	}

	err = k.parseThenHandleIBCBasicContractResponse(ctx, contractAddress, msgBz, res)
	if err != nil {
		return sdkerrors.Wrap(err, "ibc-timeout-packet")
	}
	return nil
}

func (k Keeper) handleIBCBasicContractResponse(ctx sdk.Context, addr sdk.AccAddress, ibcPortID string, inputMsg []byte, res *v1types.IBCBasicResponse) error {
	verificationInfo := types.NewVerificationInfo([]byte{}, sdktxsigning.SignMode_SIGN_MODE_UNSPECIFIED, []byte{}, []byte{}, []byte{}, nil)

	_, err := k.handleContractResponse(ctx, addr, ibcPortID, res.Messages, res.Attributes, res.Events, nil, inputMsg, verificationInfo, wasmTypes.CosmosMsgVersionV1)
	return err
}
