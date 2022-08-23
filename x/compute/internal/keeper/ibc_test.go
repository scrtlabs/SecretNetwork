package keeper

import (
	"encoding/hex"
	"testing"

	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	cosmwasm "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	v1types "github.com/enigmampc/SecretNetwork/go-cosmwasm/types/v1"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func ibcChannelConnectHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey, msg string,
	gas uint64, shouldSendOpenAck bool, channel v1types.IBCChannel,
) cosmwasm.StdError {
	// create new ctx with the same storage and a gas limit
	// this is to reset the event manager, so we won't get
	// events from past calls
	gasMeter := &WasmCounterGasMeter{0, sdk.NewGasMeter(gas)}
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(gasMeter)

	var ibcChannelConnectMsg v1types.IBCChannelConnectMsg
	if shouldSendOpenAck {
		ibcChannelConnectMsg = v1types.IBCChannelConnectMsg{
			OpenAck: &v1types.IBCOpenAck{
				Channel:             channel,
				CounterpartyVersion: "",
			},
			OpenConfirm: nil,
		}
	} else {
		ibcChannelConnectMsg = v1types.IBCChannelConnectMsg{
			OpenAck: nil,
			OpenConfirm: &v1types.IBCOpenConfirm{
				Channel: channel,
			},
		}
	}

	err := keeper.OnConnectChannel(ctx, contractAddr, ibcChannelConnectMsg)

	require.NotZero(t, gasMeter.GetWasmCounter(), err)

	if err != nil {
		return cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	return cosmwasm.StdError{}
}

func ibcChannelOpenHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey, msg string,
	gas uint64, shouldSendOpenTry bool, channel v1types.IBCChannel,
) (string, cosmwasm.StdError) {
	// create new ctx with the same storage and a gas limit
	// this is to reset the event manager, so we won't get
	// events from past calls
	gasMeter := &WasmCounterGasMeter{0, sdk.NewGasMeter(gas)}
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(gasMeter)

	var ibcChannelOpenMsg v1types.IBCChannelOpenMsg
	if shouldSendOpenTry {
		ibcChannelOpenMsg = v1types.IBCChannelOpenMsg{
			OpenTry: &v1types.IBCOpenTry{
				Channel:             channel,
				CounterpartyVersion: "",
			},
			OpenInit: nil,
		}
	} else {
		ibcChannelOpenMsg = v1types.IBCChannelOpenMsg{
			OpenTry: nil,
			OpenInit: &v1types.IBCOpenInit{
				Channel: channel,
			},
		}
	}

	res, err := keeper.OnOpenChannel(ctx, contractAddr, ibcChannelOpenMsg)

	require.NotZero(t, gasMeter.GetWasmCounter(), err)

	if err != nil {
		return "", cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	return res, cosmwasm.StdError{}
}

func ibcChannelCloseHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey, msg string,
	gas uint64, shouldSendCloseConfirn bool, channel v1types.IBCChannel,
) cosmwasm.StdError {
	// create new ctx with the same storage and a gas limit
	// this is to reset the event manager, so we won't get
	// events from past calls
	gasMeter := &WasmCounterGasMeter{0, sdk.NewGasMeter(gas)}
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(gasMeter)

	var ibcChannelCloseMsg v1types.IBCChannelCloseMsg
	if shouldSendCloseConfirn {
		ibcChannelCloseMsg = v1types.IBCChannelCloseMsg{
			CloseConfirm: &v1types.IBCCloseConfirm{
				Channel: channel,
			},
			CloseInit: nil,
		}
	} else {
		ibcChannelCloseMsg = v1types.IBCChannelCloseMsg{
			CloseConfirm: nil,
			CloseInit: &v1types.IBCCloseInit{
				Channel: channel,
			},
		}
	}

	err := keeper.OnCloseChannel(ctx, contractAddr, ibcChannelCloseMsg)

	require.NotZero(t, gasMeter.GetWasmCounter(), err)

	if err != nil {
		return cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	return cosmwasm.StdError{}
}

func createIBCEndpoint(port string, channel string) v1types.IBCEndpoint {
	return v1types.IBCEndpoint{
		PortID:    port,
		ChannelID: channel,
	}
}

func createIBCTimeout(timeout uint64) v1types.IBCTimeout {
	return v1types.IBCTimeout{
		Block:     nil,
		Timestamp: timeout,
	}
}

func createIBCPacket(src v1types.IBCEndpoint, dest v1types.IBCEndpoint, sequence uint64, timeout v1types.IBCTimeout, data []byte) v1types.IBCPacket {
	return v1types.IBCPacket{
		Data:     data,
		Src:      src,
		Dest:     dest,
		Sequence: sequence,
		Timeout:  timeout,
	}
}

func ibcPacketReceiveHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey, msg string,
	shouldEncryptMsg bool, gas uint64, packet v1types.IBCPacket,
) ([]byte, cosmwasm.StdError) {
	var nonce []byte
	internalPacket := packet

	if shouldEncryptMsg {
		contractHash, err := keeper.GetContractHash(ctx, contractAddr)
		require.NoError(t, err)
		hashStr := hex.EncodeToString(contractHash)

		msg := types.SecretMsg{
			CodeHash: []byte(hashStr),
			Msg:      packet.Data,
		}

		dataBz, err := wasmCtx.Encrypt(msg.Serialize())
		require.NoError(t, err)
		nonce = dataBz[0:32]
		internalPacket.Data = dataBz
	}

	// create new ctx with the same storage and a gas limit
	// this is to reset the event manager, so we won't get
	// events from past calls
	gasMeter := &WasmCounterGasMeter{0, sdk.NewGasMeter(gas)}
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(gasMeter)

	ibcPacketReceiveMsg := v1types.IBCPacketReceiveMsg{
		Packet:  internalPacket,
		Relayer: "relayer",
	}

	res, err := keeper.OnRecvPacket(ctx, contractAddr, ibcPacketReceiveMsg)

	require.NotZero(t, gasMeter.GetWasmCounter(), err)

	if err != nil {
		if shouldEncryptMsg {
			return nil, extractInnerError(t, err, nonce, true, true)
		}

		return nil, cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	return res, cosmwasm.StdError{}
}

func ibcPacketAckHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey, msg string,
	shouldEncryptMsg bool, gas uint64, originalPacket v1types.IBCPacket, ack []byte,
) cosmwasm.StdError {
	var nonce []byte

	if shouldEncryptMsg {
		contractHash, err := keeper.GetContractHash(ctx, contractAddr)
		require.NoError(t, err)
		hashStr := hex.EncodeToString(contractHash)

		msg := types.SecretMsg{
			CodeHash: []byte(hashStr),
			Msg:      ack,
		}

		ackBz, err := wasmCtx.Encrypt(msg.Serialize())
		require.NoError(t, err)
		nonce = ackBz[0:32]
		ack = ackBz
	}

	// create new ctx with the same storage and a gas limit
	// this is to reset the event manager, so we won't get
	// events from past calls
	gasMeter := &WasmCounterGasMeter{0, sdk.NewGasMeter(gas)}
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(gasMeter)

	ibcPacketAckMsg := v1types.IBCPacketAckMsg{
		Acknowledgement: v1types.IBCAcknowledgement{
			Data: ack,
		},
		OriginalPacket: originalPacket,
		Relayer:        "relayer",
	}

	err := keeper.OnAckPacket(ctx, contractAddr, ibcPacketAckMsg)

	require.NotZero(t, gasMeter.GetWasmCounter(), err)

	if err != nil {
		if shouldEncryptMsg {
			return extractInnerError(t, err, nonce, true, true)
		}

		return cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	return cosmwasm.StdError{}
}

func ibcPacketTimeoutHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey, msg string,
	shouldEncryptMsg bool, gas uint64, originalPacket v1types.IBCPacket,
) cosmwasm.StdError {
	var nonce []byte

	if shouldEncryptMsg {
		contractHash, err := keeper.GetContractHash(ctx, contractAddr)
		require.NoError(t, err)
		hashStr := hex.EncodeToString(contractHash)

		msg := types.SecretMsg{
			CodeHash: []byte(hashStr),
			Msg:      originalPacket.Data,
		}

		dataBz, err := wasmCtx.Encrypt(msg.Serialize())
		require.NoError(t, err)
		nonce = dataBz[0:32]
		originalPacket.Data = dataBz
	}

	// create new ctx with the same storage and a gas limit
	// this is to reset the event manager, so we won't get
	// events from past calls
	gasMeter := &WasmCounterGasMeter{0, sdk.NewGasMeter(gas)}
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(gasMeter)

	ibcPacketTimeoutMsg := v1types.IBCPacketTimeoutMsg{
		Packet:  originalPacket,
		Relayer: "relayer",
	}

	err := keeper.OnTimeoutPacket(ctx, contractAddr, ibcPacketTimeoutMsg)

	require.NotZero(t, gasMeter.GetWasmCounter(), err)

	if err != nil {
		if shouldEncryptMsg {
			return extractInnerError(t, err, nonce, true, true)
		}

		return cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	return cosmwasm.StdError{}
}
