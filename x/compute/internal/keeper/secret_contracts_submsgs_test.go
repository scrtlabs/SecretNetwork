package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	v010cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestV1ReplyOnMultipleSubmessages(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"multiple_sub_messages":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(102), binary.BigEndian.Uint32(data))
}

func TestV1MultipleSubmessagesNoReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"multiple_sub_messages_no_reply":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(10), binary.BigEndian.Uint32(data))

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(16), resp.Get.Count)
}

func TestV1StatePersistsAfterSubmessageFails(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment_and_send_failing_submessage":{"reply_on":"always"}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(11), binary.BigEndian.Uint32(data))

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(11), resp.Get.Count)
}

func TestV1StatePersistsAfterSubmessageFailsNoReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment_and_send_failing_submessage":{"reply_on":"never"}}`, true, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(11), resp.Get.Count)
}

func TestV1StatePersistsAfterSubmessageThatGeneratesBankMsgFails(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	fmt.Println("ESHELDEBUG before init") //todo remove
	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	fmt.Println("ESHELDEBUG before exec") //todo remove
	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment_and_send_submessage_with_bank_fail":{"reply_on":"never"}}`, false, true, math.MaxUint64, 0)

	fmt.Println("ESHELDEBUG before error check") //todo remove
	require.NotEmpty(t, err)
	fmt.Println("ESHELDEBUG after error check") //todo remove

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	fmt.Println("ESHELDEBUG after query") //todo remove
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(11), resp.Get.Count)
}

func TestSendEncryptedAttributesFromInitWithoutSubmessageWithoutReply(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_attributes":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
				},
				events,
			)
		})
	}
}

func TestSendEncryptedAttributesFromInitWithSubmessageWithoutReply(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
				},
				events,
			)
		})
	}
}

func TestV1SendsEncryptedAttributesFromInitWithSubmessageWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_attributes_with_submessage":{"id":2200}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr1", Value: "🦄"},
				{Key: "attr2", Value: "🌈"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🍉"},
				{Key: "attr4", Value: "🥝"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr5", Value: "🤯"},
				{Key: "attr6", Value: "🦄"},
			},
		},
		events,
	)
}

func TestSendEncryptedAttributesFromExecuteWithoutSubmessageWithoutReply(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attributes":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
				},
				events,
			)
		})
	}
}

func TestSendEncryptedAttributesFromExecuteWithSubmessageWithoutReply(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
				},
				events,
			)
		})
	}
}

func TestV1SendsEncryptedAttributesFromExecuteWithSubmessageWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":2200}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr1", Value: "🦄"},
				{Key: "attr2", Value: "🌈"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🍉"},
				{Key: "attr4", Value: "🥝"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr5", Value: "🤯"},
				{Key: "attr6", Value: "🦄"},
			},
		},
		events,
	)
}

func TestSendPlaintextFromInitWithoutSubmessageWithoutReply(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_plaintext_attributes":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, true)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
				},
				events,
			)
		})
	}
}

func TestSendPlaintextAttributesFromInitWithSubmessageWithoutReply(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, true)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
				},
				events,
			)
		})
	}
}

func TestV1SendsPlaintextAttributesFromInitWithSubmessageWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":2300}}`, true, true, defaultGasForTests, true)
	require.Empty(t, err)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr1", Value: "🦄"},
				{Key: "attr2", Value: "🌈"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🍉"},
				{Key: "attr4", Value: "🥝"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr5", Value: "🤯"},
				{Key: "attr6", Value: "🦄"},
			},
		},
		events,
	)
}

func TestSendPlaintextAttributesFromExecuteWithoutSubmessageWithoutReply(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0, true)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
				},
				events,
			)
		})
	}
}

func TestSendPlaintextAttributesFromExecuteWithSubmessageWithoutReply(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0, true)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
				},
				events,
			)
		})
	}
}

func TestV1SendsPlaintextAttributesFromExecuteWithSubmessageWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":2300}}`, true, true, defaultGasForTests, 0, true)
	require.Empty(t, err)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr1", Value: "🦄"},
				{Key: "attr2", Value: "🌈"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🍉"},
				{Key: "attr4", Value: "🥝"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr5", Value: "🤯"},
				{Key: "attr6", Value: "🦄"},
			},
		},
		events,
	)
}

func TestV1SendsEncryptedEventsFromInitWithoutSubmessageWithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	nonce, ctx, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_events":{}}`, true, true, defaultGasForTests)

	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false
	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "🐙"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)
}

func TestV1SendsEncryptedEventsFromInitWithSubmessageWithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	nonce, ctx, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_events_with_submessage":{"id":0}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false
	hadCyber3 := false
	hadCyber4 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "🐙"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}

		if e.Type == "wasm-cyber3" {
			require.False(t, hadCyber3)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🤯"},
					{Key: "attr2", Value: "🤟"},
				},
				attrs,
			)

			hadCyber3 = true
		}

		if e.Type == "wasm-cyber4" {
			require.False(t, hadCyber4)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "😅"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber4 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)
	require.True(t, hadCyber3)
	require.True(t, hadCyber4)
}

func TestV1SendsEncryptedEventsFromInitWithSubmessageWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	nonce, ctx, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_events_with_submessage":{"id":2400}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false
	hadCyber3 := false
	hadCyber4 := false
	hadCyber5 := false
	hadCyber6 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "🐙"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}

		if e.Type == "wasm-cyber3" {
			require.False(t, hadCyber3)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🤯"},
					{Key: "attr2", Value: "🤟"},
				},
				attrs,
			)

			hadCyber3 = true
		}

		if e.Type == "wasm-cyber4" {
			require.False(t, hadCyber4)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "😅"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber4 = true
		}

		if e.Type == "wasm-cyber5" {
			require.False(t, hadCyber5)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "😗"},
					{Key: "attr2", Value: "😋"},
				},
				attrs,
			)

			hadCyber5 = true
		}

		if e.Type == "wasm-cyber6" {
			require.False(t, hadCyber6)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "😉"},
					{Key: "attr4", Value: "😊"},
				},
				attrs,
			)

			hadCyber6 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)
	require.True(t, hadCyber3)
	require.True(t, hadCyber4)
	require.True(t, hadCyber5)
	require.True(t, hadCyber6)
}

func TestV1SendsEncryptedEventsFromExecuteWithoutSubmessageWithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_events":{}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "🐙"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)
}

func TestV1SendsEncryptedEventsFromExecuteWithSubmessageWithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_events_with_submessage":{"id":0}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false
	hadCyber3 := false
	hadCyber4 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "🐙"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}

		if e.Type == "wasm-cyber3" {
			require.False(t, hadCyber3)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🤯"},
					{Key: "attr2", Value: "🤟"},
				},
				attrs,
			)

			hadCyber3 = true
		}

		if e.Type == "wasm-cyber4" {
			require.False(t, hadCyber4)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "😅"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber4 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)
	require.True(t, hadCyber3)
	require.True(t, hadCyber4)
}

func TestV1SendsEncryptedEventsFromExecuteWithSubmessageWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_events_with_submessage":{"id":2400}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false
	hadCyber3 := false
	hadCyber4 := false
	hadCyber5 := false
	hadCyber6 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "🐙"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}

		if e.Type == "wasm-cyber3" {
			require.False(t, hadCyber3)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🤯"},
					{Key: "attr2", Value: "🤟"},
				},
				attrs,
			)

			hadCyber3 = true
		}

		if e.Type == "wasm-cyber4" {
			require.False(t, hadCyber4)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "😅"},
					{Key: "attr4", Value: "🦄"},
				},
				attrs,
			)

			hadCyber4 = true
		}

		if e.Type == "wasm-cyber5" {
			require.False(t, hadCyber5)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "😗"},
					{Key: "attr2", Value: "😋"},
				},
				attrs,
			)

			hadCyber5 = true
		}

		if e.Type == "wasm-cyber6" {
			require.False(t, hadCyber6)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr3", Value: "😉"},
					{Key: "attr4", Value: "😊"},
				},
				attrs,
			)

			hadCyber6 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)
	require.True(t, hadCyber3)
	require.True(t, hadCyber4)
	require.True(t, hadCyber5)
	require.True(t, hadCyber6)
}

func TestV1SendsMixedLogsFromInitWithoutSubmessageWithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	nonce, ctx, contractAddress, logs, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_mixed_attributes_and_events":{}}`, true, true, defaultGasForTests, true)

	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, false)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}
	}

	require.True(t, hadCyber1)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
		},
		logs,
	)
}

func TestV1SendsMixedAttributesAndEventsFromInitWithSubmessageWithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	nonce, ctx, contractAddress, logs, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":0}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr5", Value: "🐙"},
					{Key: "attr6", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr7", Value: "😅"},
				{Key: "attr8", Value: "🦄"},
			},
		},
		logs,
	)
}

func TestV1SendsMixedAttributesAndEventsFromInitWithSubmessageWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	nonce, ctx, contractAddress, logs, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":2500}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false
	hadCyber3 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr5", Value: "🐙"},
					{Key: "attr6", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}

		if e.Type == "wasm-cyber3" {
			require.False(t, hadCyber3)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr9", Value: "🤯"},
					{Key: "attr10", Value: "🤟"},
				},
				attrs,
			)

			hadCyber3 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)
	require.True(t, hadCyber3)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr7", Value: "😅"},
				{Key: "attr8", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr11", Value: "😉"},
				{Key: "attr12", Value: "😊"},
			},
		},
		logs,
	)
}

func TestV1SendsMixedAttributesAndEventsFromExecuteWithoutSubmessageWithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_mixed_attributes_and_events":{}}`, true, true, defaultGasForTests, 0, true)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, false)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}
	}

	require.True(t, hadCyber1)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
		},
		logs,
	)
}

func TestV1SendsMixedAttributesAndEventsFromExecuteWithSubmessageWithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":0}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr5", Value: "🐙"},
					{Key: "attr6", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr7", Value: "😅"},
				{Key: "attr8", Value: "🦄"},
			},
		},
		logs,
	)
}

func TestV1SendsMixedAttributesAndEventsFromExecuteWithSubmessageWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":2500}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false
	hadCyber3 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr5", Value: "🐙"},
					{Key: "attr6", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}

		if e.Type == "wasm-cyber3" {
			require.False(t, hadCyber3)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "attr9", Value: "🤯"},
					{Key: "attr10", Value: "🤟"},
				},
				attrs,
			)

			hadCyber3 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber2)
	require.True(t, hadCyber3)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr7", Value: "😅"},
				{Key: "attr8", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "attr11", Value: "😉"},
				{Key: "attr12", Value: "😊"},
			},
		},
		logs,
	)
}

func TestSubmessageGasExceedingMessageGas(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	defer func() {
		r := recover()
		require.NotNil(t, r)
		_, ok := r.(sdk.ErrorOutOfGas)
		require.True(t, ok, "%+v", r)
	}()
	_, _, _, _, _ = initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"measure_gas_for_submessage":{"id":0}}`, false, true, defaultGasForTests)
}

func TestReplyGasExceedingMessageGas(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	defer func() {
		r := recover()
		require.NotNil(t, r)
		_, ok := r.(sdk.ErrorOutOfGas)
		require.True(t, ok, "%+v", r)
	}()
	_, _, _, _, _ = initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"measure_gas_for_submessage":{"id":2600}}`, false, true, defaultGasForTests)
}
