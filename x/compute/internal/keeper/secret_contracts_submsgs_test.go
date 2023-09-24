package keeper

import (
	"encoding/binary"
	"encoding/json"
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

/*
Test case:
 0. Initial counter stored in state: 10
 1. Contract increments 1 (=11)
 2. Contract sends itself submessage, with reply_on=always
 3. Submessage execution sets counter to 123456
 4. Submessage fails with error
 5. Contract's reply() returns Ok

Expected Outcome:
  - Counter is still on 11, revert only submessage changes
  - No Error (tx as a whole succeeds - counter will stay at 11)
*/
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

/*
Test case: Same as previous one but there's no reply.
Expected Outcome: Since the caller contract is not executed again, the final result of this message is determined by
the submessage recipient. It errors, so the whole transaction will revert.
*/
func TestV1StateRevertsAfterSubmessageFailsAndNoReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment_and_send_failing_submessage":{"reply_on":"never"}}`, true, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	// The state here should have been reverted by the APP but in go-tests we create our own keeper so it is not reverted
	// in this case. Only the submessage changes revert, since we manage them in a sub-context in our keeper.
	require.Equal(t, uint32(11), resp.Get.Count)
}

/*
Test case:
 0. Initial counter stored in state: 10
 1. Contract increments 1 (=11)
 2. Contract sends itself submessage with reply=always
 3. Submessage execution increments 1 (=12)
 4. Submessage adds a failing bank message to the messages list
 5. Submessage returns Ok, since the bank message did not execute yet
 6. Bank message fails on go side
    Note: If the reply was called, it would have incremented the counter again (to 12)

Observed Outcome:
  - Contract does not handle reply, even though it is reply_on=always, because sdk messages revert the whole tx.
    This is because regular messages are not postponed until all Submsgs are processed. Instead, they are processed
    in the same ordering as SubMsgs. This differs from cosmwasm's documentation at https://github.com/CosmWasm/cosmwasm/blob/main/SEMANTICS.md#order-and-rollback
    The difference in outcomes is small when sdk message fails, because either way, the whole tx is reverted, but may
	be present in cases of contract-queries-to-chain if the sdk message succeeds.

  - Counter is still on 11. SubMsg changes reverted, reply not reached. (first increment remains as it is reverted in sdk)
  - Error is not empty, whole tx will revert because of sdk message failing.
*/
func TestV1StateRevertsAfterSubmessageThatGeneratesBankMsgFails(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment_and_send_submessage_with_bank_fail":{"reply_on":"always"}}`, false, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)

	// The state here should have been reverted by the APP but in go-tests we create our own keeper so it is not reverted
	// in this case. Only the submessage changes revert, since we manage them in a sub-context in our keeper.
	require.Equal(t, uint32(11), resp.Get.Count)
}

/*
Test case:
 0. Initial counter stored in state: 10
 1. Contract increments 1 (=11)
 2. Contract sends:
    2a. Submessage to itself:
    2a-1. Submessage execution increments counter by 3 (=14)
    2a-2. Submessage succeeds
    2b. Failing Bank message.

Expected Result:
  - Error is not empty - will revert all changes
  - Counter is on 11 (main message revert happens in sdk, not reflected here)
*/
func TestV1SubmessageStateRevertsIfCallerFails(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"send_succeeding_submessage_and_failing_message":{}}`, true, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)

	// The state here should have been reverted by the APP but in go-tests we create our own keeper so it is not reverted
	// in this case. Only the submessage changes revert, since we manage them in a sub-context in our keeper.
	require.Equal(t, uint32(11), resp.Get.Count)
}

/*
Test case:
 0. Initial counter stored in state: 10
 1. Contract increments 1 (=11)
 2. Contract sends itself submessage
 3. Submessage execution increments counter by 3 (=14)
 4. Submessage succeeds
 5. Contract receives reply
 6. Contract increments 1 again (=15)
 7. In reply, Contract issues a failing bank message to the go code

Expected Result:
  - Error is not empty - will revert all changes
  - Counter is on 15 - whole tx revert happens on sdk
*/
func TestV1SubmessageStateRevertsIfCallerFailsOnReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"send_succeeding_submessage_then_failing_message_on_reply":{}}`, false, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(15), resp.Get.Count)
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
