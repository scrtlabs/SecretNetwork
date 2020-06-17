package keeper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	cosmwasm "github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/types"

	"github.com/stretchr/testify/require"
)

// getDecryptedWasmEvents gets all "wasm" events and decrypt what's necessary
// Returns all "wasm" events, including from contract callbacks
func getDecryptedWasmEvents(t *testing.T, ctx sdk.Context, nonce []byte, skip uint) [][]cosmwasm.LogAttribute {
	events := ctx.EventManager().Events()
	var res [][]cosmwasm.LogAttribute
	for _, e := range events[skip:] {
		if e.Type == "wasm" {
			newEvent := []cosmwasm.LogAttribute{}
			for _, oldLog := range e.Attributes {
				newLog := cosmwasm.LogAttribute{
					Key:   string(oldLog.Key),
					Value: string(oldLog.Value),
				}

				if newLog.Key != "contract_address" {
					// key
					keyCipherBz, err := base64.StdEncoding.DecodeString(newLog.Key)
					require.NoError(t, err)
					keyPlainBz, err := wasmCtx.Decrypt(keyCipherBz, nonce)
					require.NoError(t, err)
					newLog.Key = string(keyPlainBz)

					// value
					valueCipherBz, err := base64.StdEncoding.DecodeString(newLog.Value)
					require.NoError(t, err)
					valuePlainBz, err := wasmCtx.Decrypt(valueCipherBz, nonce)
					require.NoError(t, err)
					newLog.Value = string(valuePlainBz)
				}

				newEvent = append(newEvent, newLog)
			}
			res = append(res, newEvent)
		}
	}
	return res
}

// getDecryptedData decrytes the output of the first function to be called
// Only returns the data, logs and messages from the first function call
func getDecryptedData(t *testing.T, data []byte, nonce []byte) cosmwasm.CosmosResponse {
	var res cosmwasm.CosmosResponse
	err := json.Unmarshal(data, &res)
	require.NoError(t, err)

	// err
	if res.Err != "" {
		errCipherBz, err := base64.StdEncoding.DecodeString(res.Err)
		require.NoError(t, err)
		errPlainBz, err := wasmCtx.Decrypt(errCipherBz, nonce)
		require.NoError(t, err)

		res.Err = string(errPlainBz)
	}

	// data
	if res.Ok.Data != "" {
		dataCiphertextBz, err := base64.StdEncoding.DecodeString(res.Ok.Data)
		require.NoError(t, err)
		dataPlaintext, err := wasmCtx.Decrypt(dataCiphertextBz, nonce)
		require.NoError(t, err)

		res.Ok.Data = string(dataPlaintext)
	}

	// logs
	for i, log := range res.Ok.Log {
		// key
		if log.Key != "" {
			keyCipherBz, err := base64.StdEncoding.DecodeString(log.Key)
			require.NoError(t, err)
			keyPlainBz, err := wasmCtx.Decrypt(keyCipherBz, nonce)
			require.NoError(t, err)
			log.Key = string(keyPlainBz)
		}

		// value
		if log.Value != "" {
			valueCipherBz, err := base64.StdEncoding.DecodeString(log.Value)
			require.NoError(t, err)
			valuePlainBz, err := wasmCtx.Decrypt(valueCipherBz, nonce)
			require.NoError(t, err)
			log.Value = string(valuePlainBz)
		}

		res.Ok.Log[i] = log
	}

	// messages
	for i, msg := range res.Ok.Messages {
		msgPlaintext, err := wasmCtx.Decrypt(msg.Contract.Msg[64:], nonce)
		require.NoError(t, err)
		msg.Contract.Msg = msgPlaintext

		res.Ok.Messages[i] = msg
	}

	return res
}

func requireQueryResult(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddr sdk.AccAddress, input string, expectedOutput string) {
	queryBz, err := wasmCtx.Encrypt([]byte(input))
	require.NoError(t, err)

	resultCipherBz, err := keeper.QuerySmart(ctx, contractAddr, queryBz)
	require.NoError(t, err)

	nonce := queryBz[0:32]
	resultPlainBz, err := wasmCtx.Decrypt(resultCipherBz, nonce)
	require.NoError(t, err)

	resultBz, err := base64.StdEncoding.DecodeString(string(resultPlainBz))
	require.NoError(t, err)

	require.JSONEq(t, expectedOutput, string(resultBz))
}

func executeHelper(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress sdk.AccAddress, txSender sdk.AccAddress, execMsg string, skipEvents uint) (cosmwasm.CosmosResponse, [][]cosmwasm.LogAttribute) {
	execMsgBz, err := wasmCtx.Encrypt([]byte(execMsg))
	require.NoError(t, err)

	execResult, err := keeper.Execute(ctx, contractAddress, txSender, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	nonce := execMsgBz[0:32]

	// Events is from all callbacks
	wasmEvents := getDecryptedWasmEvents(t, ctx, nonce, skipEvents)

	// Data is the output of only the first call
	data := getDecryptedData(t, execResult.Data, nonce)

	return data, wasmEvents
}

func TestCallbackSanity(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	// check init events (no data in init)
	initEvents := getDecryptedWasmEvents(t, ctx, initMsgBz[0:32], 0)

	require.Equal(t, 1, len(initEvents))
	require.Equal(t,
		[][]cosmwasm.LogAttribute{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "init", Value: "🌈"},
			},
		},
		initEvents,
	)

	data, execEvents := executeHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"a":{"contract_addr":"%s","x":2,"y":3}}`, contractAddress.String()), 1)

	require.Equal(t, 3, len(execEvents))
	require.Equal(t,
		[][]cosmwasm.LogAttribute{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "banana", Value: "🍌"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "kiwi", Value: "🥝"},
			},
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "watermelon", Value: "🍉"},
			},
		},
		execEvents,
	)

	require.Empty(t, data.Err)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte{2, 3}), data.Ok.Data)
	require.Equal(t, []cosmwasm.LogAttribute{{Key: "banana", Value: "🍌"}}, data.Ok.Log)
	require.Equal(t, 1, len(data.Ok.Messages))
	require.NotNil(t, data.Ok.Messages[0].Contract)
	require.Equal(t, data.Ok.Messages[0].Contract.ContractAddr, contractAddress.String())
	require.JSONEq(t,
		string(data.Ok.Messages[0].Contract.Msg),
		fmt.Sprintf(`{"b":{"x":2,"y":3,"contract_addr":"%s"}}`, contractAddress.String()),
	)
}

func TestSanity(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	walletA := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	walletB := createFakeFundedAccount(ctx, accKeeper, topUp)

	// https://github.com/CosmWasm/cosmwasm-examples/blob/f5ea00a85247abae8f8cbcba301f94ef21c66087/erc20/src/contract.rs
	wasmCode, err := ioutil.ReadFile("./testdata/erc20-f5ea00a85247abae8f8cbcba301f94ef21c66087.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)

	contractAddress, err := keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", deposit)
	require.NoError(t, err)

	// check init events (no data in init)
	initEvents := getDecryptedWasmEvents(t, ctx, initMsgBz[0:32], 0)
	require.Empty(t, initEvents)

	// check state after init
	requireQueryResult(t,
		keeper, ctx, contractAddress,
		fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()),
		`{"balance":"108"}`,
	)
	requireQueryResult(t,
		keeper, ctx, contractAddress,
		fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()),
		`{"balance":"53"}`,
	)

	// transfer 10 from A to B
	data, wasmEvents := executeHelper(t, keeper, ctx, contractAddress, walletA,
		fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String()), 0)

	require.Empty(t, data.Err)
	require.Empty(t, data.Ok.Data)
	require.Empty(t, data.Ok.Messages)
	require.Equal(t,
		[]cosmwasm.LogAttribute{
			{Key: "action", Value: "transfer"},
			{Key: "sender", Value: walletA.String()},
			{Key: "recipient", Value: walletB.String()}},
		data.Ok.Log,
	)

	require.Equal(t, 1, len(wasmEvents))
	require.Equal(t,
		[][]cosmwasm.LogAttribute{{
			{Key: "contract_address", Value: contractAddress.String()},
			{Key: "action", Value: "transfer"},
			{Key: "sender", Value: walletA.String()},
			{Key: "recipient", Value: walletB.String()},
		}},
		wasmEvents,
	)

	// check state after transfer
	requireQueryResult(t,
		keeper, ctx, contractAddress,
		fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()),
		`{"balance":"98"}`,
	)
	requireQueryResult(t,
		keeper, ctx, contractAddress,
		fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()),
		`{"balance":"63"}`,
	)
}

func TestInitLogs(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	// check init events (no data in init)
	initEvents := getDecryptedWasmEvents(t, ctx, initMsgBz[0:32], 0)

	require.Equal(t, 1, len(initEvents))
	require.Equal(t,
		[][]cosmwasm.LogAttribute{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "init", Value: "🌈"},
			},
		},
		initEvents,
	)
}

func TestEmptyLogKeyValue(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	data, execEvents := executeHelper(t, keeper, ctx, contractAddress, walletA, `{"emptylogkeyvalue":{}}`, 1)

	require.Empty(t, data.Err)
	require.Equal(t, 1, len(execEvents))
	require.Equal(t,
		[][]cosmwasm.LogAttribute{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "my value is empty", Value: ""},
				{Key: "", Value: "my key is empty"},
			},
		},
		execEvents,
	)
	require.Equal(t,
		[]cosmwasm.LogAttribute{
			{Key: "my value is empty", Value: ""},
			{Key: "", Value: "my key is empty"},
		},
		data.Ok.Log,
	)
}

func TestEmptyData(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	data, _ := executeHelper(t, keeper, ctx, contractAddress, walletA, `{"emptydata":{}}`, 1)

	require.Empty(t, data.Err)
	require.Equal(t, "", data.Ok.Data)
}

func TestNoData(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	data, _ := executeHelper(t, keeper, ctx, contractAddress, walletA, `{"nodata":{}}`, 1)

	require.Empty(t, data.Err)
	require.Equal(t, "", data.Ok.Data)
}

func TestExecuteError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	data, _ := executeHelper(t, keeper, ctx, contractAddress, walletA, `bad input`, 1)

	require.Contains(t, data.Err, "Error parsing HandleMsg")
}

func TestInitError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`bad input`))
	require.NoError(t, err)

	// init
	_, err = keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	require.Contains(t, err.Error(), "instantiate wasm contract failed")

	errorCipherB64 := strings.ReplaceAll(err.Error(), "instantiate wasm contract failed: ", "")
	errorCipherBz, err := base64.StdEncoding.DecodeString(errorCipherB64)
	require.NoError(t, err)
	errorPlainBz, err := wasmCtx.Decrypt(errorCipherBz, initMsgBz[0:32])
	require.NoError(t, err)
	initErrorPlain := string(errorPlainBz)

	require.Contains(t, initErrorPlain, "Error parsing InitMsg")
}

func TestInitCallback(t *testing.T) {
	t.SkipNow() // still not implemented in CosmWasm 0.9

	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	// init first contract so we'd have someone to callback
	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)
	firstContractAddress, err := keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	// check init events (no data in init)
	initEvents := getDecryptedWasmEvents(t, ctx, initMsgBz[0:32], 0)

	require.Equal(t, 1, len(initEvents))
	require.Equal(t,
		[][]cosmwasm.LogAttribute{
			{
				{Key: "contract_address", Value: firstContractAddress.String()},
				{Key: "init", Value: "🌈"},
			},
		},
		initEvents,
	)

	// init second contract and callback to the first contract
	initMsgBz, err = wasmCtx.Encrypt([]byte(fmt.Sprintf(`{"callback":{"contract_addr":"%s"}}`, firstContractAddress.String())))
	require.NoError(t, err)

	contractAddress, err := keeper.Instantiate(ctx, contractID, walletA, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	// check init events (no data in init)
	initEvents = getDecryptedWasmEvents(t, ctx, initMsgBz[0:32], 1)

	require.Equal(t, 2, len(initEvents))
	require.Equal(t,
		[][]cosmwasm.LogAttribute{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "init with a callback", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: firstContractAddress.String()},
				{Key: "watermelon", Value: "🍉"},
			},
		},
		initEvents,
	)
}
