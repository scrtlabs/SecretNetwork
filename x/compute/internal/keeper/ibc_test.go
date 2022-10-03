package keeper

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	v010cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	v1types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

const defaultGasForIbcTests = 600_000

func ibcChannelConnectHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey,
	gas uint64, shouldSendOpenAck bool, channel v1types.IBCChannel,
) (sdk.Context, []ContractEvent, cosmwasm.StdError) {
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
		return ctx, nil, cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := tryDecryptWasmEvents(ctx, []byte{}, true)

	return ctx, wasmEvents, cosmwasm.StdError{}
}

func ibcChannelOpenHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey,
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
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey,
	gas uint64, shouldSendCloseConfirn bool, channel v1types.IBCChannel,
) (sdk.Context, []ContractEvent, cosmwasm.StdError) {
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
		return ctx, nil, cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := tryDecryptWasmEvents(ctx, []byte{}, true)

	return ctx, wasmEvents, cosmwasm.StdError{}
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
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey,
	shouldEncryptMsg bool, gas uint64, packet v1types.IBCPacket,
) (sdk.Context, []byte, []ContractEvent, []byte, cosmwasm.StdError) {
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

	// wasmEvents comes from all the callbacks as well
	wasmEvents := tryDecryptWasmEvents(ctx, nonce, !shouldEncryptMsg)

	if err != nil {
		if shouldEncryptMsg {
			return ctx, nil, nil, nil, extractInnerError(t, err, nonce, true, true)
		}

		return ctx, nil, nil, nil, cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	data := res
	if shouldEncryptMsg {
		data = getDecryptedData(t, res, nonce)
	}

	return ctx, nonce, wasmEvents, data, cosmwasm.StdError{}
}

func ibcPacketAckHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey, gas uint64, originalPacket v1types.IBCPacket, ack []byte,
) (sdk.Context, []ContractEvent, cosmwasm.StdError) {
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
		return ctx, nil, cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := tryDecryptWasmEvents(ctx, []byte{}, true)

	return ctx, wasmEvents, cosmwasm.StdError{}
}

func ibcPacketTimeoutHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, creatorPrivKey crypto.PrivKey, gas uint64, originalPacket v1types.IBCPacket,
) (sdk.Context, []ContractEvent, cosmwasm.StdError) {
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
		return ctx, nil, cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := tryDecryptWasmEvents(ctx, []byte{}, true)

	return ctx, wasmEvents, cosmwasm.StdError{}
}

func TestIBCChannelOpen(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	ibcChannel := v1types.IBCChannel{
		Endpoint:             createIBCEndpoint(PortIDForContract(contractAddress), "channel.0"),
		CounterpartyEndpoint: createIBCEndpoint(PortIDForContract(contractAddress), "channel.1"),
		Order:                v1types.Unordered,
		Version:              "1",
		ConnectionID:         "1",
	}

	version, err := ibcChannelOpenHelper(t, keeper, ctx, contractAddress, privKeyA, defaultGasForTests, false, ibcChannel)
	require.Empty(t, err)
	require.Equal(t, version, "ibc-v1")

	queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)
	require.Empty(t, err)

	require.Equal(t, "1", queryRes)
}

func TestIBCChannelOpenTry(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	ibcChannel := v1types.IBCChannel{
		Endpoint:             createIBCEndpoint(PortIDForContract(contractAddress), "channel.0"),
		CounterpartyEndpoint: createIBCEndpoint(PortIDForContract(contractAddress), "channel.1"),
		Order:                v1types.Unordered,
		Version:              "1",
		ConnectionID:         "1",
	}

	version, err := ibcChannelOpenHelper(t, keeper, ctx, contractAddress, privKeyA, defaultGasForTests, true, ibcChannel)
	require.Empty(t, err)
	require.Equal(t, version, "ibc-v1")

	queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)
	require.Empty(t, err)

	require.Equal(t, "2", queryRes)
}

func TestIBCChannelConnect(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	for _, test := range []struct {
		description   string
		connectionID  string
		output        string
		isSuccess     bool
		hasAttributes bool
		hasEvents     bool
	}{
		{
			description:   "Default",
			connectionID:  "0",
			output:        "4",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "SubmessageNoReply",
			connectionID:  "1",
			output:        "10",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "SubmessageWithReply",
			connectionID:  "2",
			output:        "17",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "Attributes",
			connectionID:  "3",
			output:        "7",
			isSuccess:     true,
			hasAttributes: true,
			hasEvents:     false,
		},
		{
			description:   "Events",
			connectionID:  "4",
			output:        "8",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     true,
		},
		{
			description:   "Error",
			connectionID:  "5",
			output:        "",
			isSuccess:     false,
			hasAttributes: false,
			hasEvents:     false,
		},
	} {
		t.Run(test.description, func(t *testing.T) {
			ibcChannel := v1types.IBCChannel{
				Endpoint:             createIBCEndpoint(PortIDForContract(contractAddress), "channel.0"),
				CounterpartyEndpoint: createIBCEndpoint(PortIDForContract(contractAddress), "channel.1"),
				Order:                v1types.Unordered,
				Version:              "1",
				ConnectionID:         test.connectionID,
			}

			ctx, events, err := ibcChannelConnectHelper(t, keeper, ctx, contractAddress, privKeyA, defaultGasForIbcTests, false, ibcChannel)

			if !test.isSuccess {
				require.Contains(t, fmt.Sprintf("%+v", err), "Intentional")
			} else {
				require.Empty(t, err)
				if test.hasAttributes {
					require.Equal(t,
						[]ContractEvent{
							{
								{Key: "contract_address", Value: contractAddress.String()},
								{Key: "attr1", Value: "😗"},
							},
						},
						events,
					)
				}

				if test.hasEvents {
					hadCyber1 := false
					evts := ctx.EventManager().Events()
					for _, e := range evts {
						if e.Type == "wasm-cyber1" {
							require.False(t, hadCyber1)
							attrs, err := parseAndDecryptAttributes(e.Attributes, []byte{}, false)
							require.Empty(t, err)

							require.Equal(t,
								[]v010cosmwasm.LogAttribute{
									{Key: "contract_address", Value: contractAddress.String()},
									{Key: "attr1", Value: "🤯"},
								},
								attrs,
							)

							hadCyber1 = true
						}
					}

					require.True(t, hadCyber1)
				}

				queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)

				require.Empty(t, err)

				require.Equal(t, test.output, queryRes)
			}
		})
	}
}

func TestIBCChannelConnectOpenAck(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	ibcChannel := v1types.IBCChannel{
		Endpoint:             createIBCEndpoint(PortIDForContract(contractAddress), "channel.0"),
		CounterpartyEndpoint: createIBCEndpoint(PortIDForContract(contractAddress), "channel.1"),
		Order:                v1types.Unordered,
		Version:              "1",
		ConnectionID:         "1",
	}

	ctx, _, err = ibcChannelConnectHelper(t, keeper, ctx, contractAddress, privKeyA, defaultGasForTests, true, ibcChannel)
	require.Empty(t, err)

	queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)
	require.Empty(t, err)

	require.Equal(t, "3", queryRes)
}

func TestIBCChannelClose(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForIbcTests)
	require.Empty(t, err)

	for _, test := range []struct {
		description   string
		connectionID  string
		output        string
		isSuccess     bool
		hasAttributes bool
		hasEvents     bool
	}{
		{
			description:   "Default",
			connectionID:  "0",
			output:        "6",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "SubmessageNoReply",
			connectionID:  "1",
			output:        "12",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "SubmessageWithReply",
			connectionID:  "2",
			output:        "19",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "Attributes",
			connectionID:  "3",
			output:        "9",
			isSuccess:     true,
			hasAttributes: true,
			hasEvents:     false,
		},
		{
			description:   "Events",
			connectionID:  "4",
			output:        "10",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     true,
		},
		{
			description:   "Error",
			connectionID:  "5",
			output:        "",
			isSuccess:     false,
			hasAttributes: false,
			hasEvents:     false,
		},
	} {
		t.Run(test.description, func(t *testing.T) {
			ibcChannel := v1types.IBCChannel{
				Endpoint:             createIBCEndpoint(PortIDForContract(contractAddress), "channel.0"),
				CounterpartyEndpoint: createIBCEndpoint(PortIDForContract(contractAddress), "channel.1"),
				Order:                v1types.Unordered,
				Version:              "1",
				ConnectionID:         test.connectionID,
			}

			ctx, events, err := ibcChannelCloseHelper(t, keeper, ctx, contractAddress, privKeyA, defaultGasForIbcTests, true, ibcChannel)

			if !test.isSuccess {
				require.Contains(t, fmt.Sprintf("%+v", err), "Intentional")
			} else {
				require.Empty(t, err)
				if test.hasAttributes {
					require.Equal(t,
						[]ContractEvent{
							{
								{Key: "contract_address", Value: contractAddress.String()},
								{Key: "attr1", Value: "😗"},
							},
						},
						events,
					)
				}

				if test.hasEvents {
					hadCyber1 := false
					evts := ctx.EventManager().Events()
					for _, e := range evts {
						if e.Type == "wasm-cyber1" {
							require.False(t, hadCyber1)
							attrs, err := parseAndDecryptAttributes(e.Attributes, []byte{}, false)
							require.Empty(t, err)

							require.Equal(t,
								[]v010cosmwasm.LogAttribute{
									{Key: "contract_address", Value: contractAddress.String()},
									{Key: "attr1", Value: "🤯"},
								},
								attrs,
							)

							hadCyber1 = true
						}
					}

					require.True(t, hadCyber1)
				}

				queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)

				require.Empty(t, err)

				require.Equal(t, test.output, queryRes)
			}
		})
	}
}

func TestIBCChannelCloseInit(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	ibcChannel := v1types.IBCChannel{
		Endpoint:             createIBCEndpoint(PortIDForContract(contractAddress), "channel.0"),
		CounterpartyEndpoint: createIBCEndpoint(PortIDForContract(contractAddress), "channel.1"),
		Order:                v1types.Unordered,
		Version:              "1",
		ConnectionID:         "1",
	}

	ctx, _, err = ibcChannelCloseHelper(t, keeper, ctx, contractAddress, privKeyA, defaultGasForTests, false, ibcChannel)
	require.Empty(t, err)

	queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)
	require.Empty(t, err)

	require.Equal(t, "5", queryRes)
}

func TestIBCPacketReceive(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	for _, isEncrypted := range []bool{false, true} {
		for _, test := range []struct {
			description   string
			sequence      uint64
			output        string
			isSuccess     bool
			hasAttributes bool
			hasEvents     bool
		}{
			{
				description:   "Default",
				sequence:      0,
				output:        "7",
				isSuccess:     true,
				hasAttributes: false,
				hasEvents:     false,
			},
			{
				description:   "SubmessageNoReply",
				sequence:      1,
				output:        "13",
				isSuccess:     true,
				hasAttributes: false,
				hasEvents:     false,
			},
			{
				description:   "SubmessageWithReply",
				sequence:      2,
				output:        "20",
				isSuccess:     true,
				hasAttributes: false,
				hasEvents:     false,
			},
			{
				description:   "Attributes",
				sequence:      3,
				output:        "10",
				isSuccess:     true,
				hasAttributes: true,
				hasEvents:     false,
			},
			{
				description:   "Events",
				sequence:      4,
				output:        "11",
				isSuccess:     true,
				hasAttributes: false,
				hasEvents:     true,
			},
			{
				description:   "Error",
				sequence:      5,
				output:        "",
				isSuccess:     false,
				hasAttributes: false,
				hasEvents:     false,
			},
			{
				description:   "SubmessageWithReplyThatCallsToSubmessage",
				sequence:      6,
				output:        "35",
				isSuccess:     true,
				hasAttributes: false,
				hasEvents:     false,
			},
		} {
			t.Run(fmt.Sprintf("%s-Encryption:%t", test.description, isEncrypted), func(t *testing.T) {
				ibcPacket := createIBCPacket(createIBCEndpoint(PortIDForContract(contractAddress), "channel.11231231231231232112312321321321331232132131232132131232"),
					createIBCEndpoint(PortIDForContract(contractAddress), "channel.0123123213213123123213123123123123123312321321321312321313213"),
					test.sequence,
					createIBCTimeout(math.MaxUint64),
					[]byte{},
				)
				ctx, nonce, events, data, err := ibcPacketReceiveHelper(t, keeper, ctx, contractAddress, privKeyA, isEncrypted, defaultGasForIbcTests, ibcPacket)

				if !test.isSuccess {
					require.Contains(t, fmt.Sprintf("%+v", err), "Intentional")
				} else {
					require.Empty(t, err)
					require.Equal(t, "\"out\"", string(data))

					if test.hasAttributes {
						require.Equal(t,
							[]ContractEvent{
								{
									{Key: "contract_address", Value: contractAddress.String()},
									{Key: "attr1", Value: "😗"},
								},
							},
							events,
						)
					}

					if test.hasEvents {
						hadCyber1 := false
						evts := ctx.EventManager().Events()
						for _, e := range evts {
							if e.Type == "wasm-cyber1" {
								require.False(t, hadCyber1)
								attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, isEncrypted)
								require.Empty(t, err)

								require.Equal(t,
									[]v010cosmwasm.LogAttribute{
										{Key: "contract_address", Value: contractAddress.String()},
										{Key: "attr1", Value: "🤯"},
									},
									attrs,
								)

								hadCyber1 = true
							}
						}

						require.True(t, hadCyber1)
					}

					queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)

					require.Empty(t, err)
					require.Equal(t, test.output, queryRes)
				}
			})
		}
	}
}

type ContractInfo struct {
	Address string `json:"address"`
	Hash    string `json:"hash"`
}

func TestIBCPacketReceiveCallsV010Contract(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForIbcTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"counter":{"counter":10}}`, true, false, defaultGasForIbcTests)
	require.Empty(t, err)
	contractInfo := ContractInfo{
		Address: v010ContractAddress.String(),
		Hash:    v010CodeHash,
	}

	contractInfoBz, err := json.Marshal(contractInfo)
	require.NoError(t, err)

	ibcPacket := createIBCPacket(createIBCEndpoint(PortIDForContract(contractAddress), "channel.1"),
		createIBCEndpoint(PortIDForContract(contractAddress), "channel.0"),
		7,
		createIBCTimeout(math.MaxUint64),
		contractInfoBz,
	)

	expected_v010_result := uint32(15)

	for _, isEncrypted := range []bool{true, true} {
		t.Run(fmt.Sprintf("Encryption:%t", isEncrypted), func(t *testing.T) {
			ctx, _, _, data, err := ibcPacketReceiveHelper(t, keeper, ctx, contractAddress, privKeyA, isEncrypted, defaultGasForIbcTests, ibcPacket)
			require.Empty(t, err)
			require.Equal(t, "\"out\"", string(data))

			queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)

			require.Empty(t, err)
			require.Equal(t, "20", queryRes)

			queryRes, qErr := queryHelper(t, keeper, ctx, v010ContractAddress, `{"get":{}}`, true, false, math.MaxUint64)
			require.Empty(t, qErr)

			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, expected_v010_result, resp.Get.Count)
			expected_v010_result += 5
		})
	}
}

func TestIBCPacketAck(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForIbcTests)
	require.Empty(t, err)

	for _, test := range []struct {
		description   string
		sequence      uint64
		output        string
		isSuccess     bool
		hasAttributes bool
		hasEvents     bool
	}{
		{
			description:   "Default",
			sequence:      0,
			output:        "8",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "SubmessageNoReply",
			sequence:      1,
			output:        "14",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "SubmessageWithReply",
			sequence:      2,
			output:        "21",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "Attributes",
			sequence:      3,
			output:        "11",
			isSuccess:     true,
			hasAttributes: true,
			hasEvents:     false,
		},
		{
			description:   "Events",
			sequence:      4,
			output:        "12",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     true,
		},
		{
			description:   "Error",
			sequence:      5,
			output:        "",
			isSuccess:     false,
			hasAttributes: false,
			hasEvents:     false,
		},
	} {
		t.Run(test.description, func(t *testing.T) {
			ibcPacket := createIBCPacket(createIBCEndpoint(PortIDForContract(contractAddress), "channel.1"),
				createIBCEndpoint(PortIDForContract(contractAddress), "channel.0"),
				test.sequence,
				createIBCTimeout(math.MaxUint64),
				[]byte{},
			)
			ack := make([]byte, 8)
			binary.LittleEndian.PutUint64(ack, uint64(test.sequence))

			ctx, events, err := ibcPacketAckHelper(t, keeper, ctx, contractAddress, privKeyA, defaultGasForIbcTests, ibcPacket, ack)

			if !test.isSuccess {
				require.Contains(t, fmt.Sprintf("%+v", err), "Intentional")
			} else {
				require.Empty(t, err)
				if test.hasAttributes {
					require.Equal(t,
						[]ContractEvent{
							{
								{Key: "contract_address", Value: contractAddress.String()},
								{Key: "attr1", Value: "😗"},
							},
						},
						events,
					)
				}

				if test.hasEvents {
					hadCyber1 := false
					evts := ctx.EventManager().Events()
					for _, e := range evts {
						if e.Type == "wasm-cyber1" {
							require.False(t, hadCyber1)
							attrs, err := parseAndDecryptAttributes(e.Attributes, []byte{}, false)
							require.Empty(t, err)

							require.Equal(t,
								[]v010cosmwasm.LogAttribute{
									{Key: "contract_address", Value: contractAddress.String()},
									{Key: "attr1", Value: "🤯"},
								},
								attrs,
							)

							hadCyber1 = true
						}
					}

					require.True(t, hadCyber1)
				}

				queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)

				require.Empty(t, err)

				require.Equal(t, test.output, queryRes)
			}
		})
	}
}

func TestIBCPacketTimeout(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForIbcTests)
	require.Empty(t, err)

	for _, test := range []struct {
		description   string
		sequence      uint64
		output        string
		isSuccess     bool
		hasAttributes bool
		hasEvents     bool
	}{
		{
			description:   "Default",
			sequence:      0,
			output:        "9",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "SubmessageNoReply",
			sequence:      1,
			output:        "15",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "SubmessageWithReply",
			sequence:      2,
			output:        "22",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     false,
		},
		{
			description:   "Attributes",
			sequence:      3,
			output:        "12",
			isSuccess:     true,
			hasAttributes: true,
			hasEvents:     false,
		},
		{
			description:   "Events",
			sequence:      4,
			output:        "13",
			isSuccess:     true,
			hasAttributes: false,
			hasEvents:     true,
		},
		{
			description:   "Error",
			sequence:      5,
			output:        "",
			isSuccess:     false,
			hasAttributes: false,
			hasEvents:     false,
		},
	} {
		t.Run(test.description, func(t *testing.T) {
			ibcPacket := createIBCPacket(createIBCEndpoint(PortIDForContract(contractAddress), "channel.1"),
				createIBCEndpoint(PortIDForContract(contractAddress), "channel.0"),
				test.sequence,
				createIBCTimeout(math.MaxUint64),
				[]byte{},
			)

			ctx, events, err := ibcPacketTimeoutHelper(t, keeper, ctx, contractAddress, privKeyA, defaultGasForIbcTests, ibcPacket)

			if !test.isSuccess {
				require.Contains(t, fmt.Sprintf("%+v", err), "Intentional")
			} else {
				require.Empty(t, err)
				if test.hasAttributes {
					require.Equal(t,
						[]ContractEvent{
							{
								{Key: "contract_address", Value: contractAddress.String()},
								{Key: "attr1", Value: "😗"},
							},
						},
						events,
					)
				}

				if test.hasEvents {
					hadCyber1 := false
					evts := ctx.EventManager().Events()
					for _, e := range evts {
						if e.Type == "wasm-cyber1" {
							require.False(t, hadCyber1)
							attrs, err := parseAndDecryptAttributes(e.Attributes, []byte{}, false)
							require.Empty(t, err)

							require.Equal(t,
								[]v010cosmwasm.LogAttribute{
									{Key: "contract_address", Value: contractAddress.String()},
									{Key: "attr1", Value: "🤯"},
								},
								attrs,
							)

							hadCyber1 = true
						}
					}

					require.True(t, hadCyber1)
				}

				queryRes, err := queryHelper(t, keeper, ctx, contractAddress, `{"q":{}}`, true, true, math.MaxUint64)

				require.Empty(t, err)

				require.Equal(t, test.output, queryRes)
			}
		})
	}
}
