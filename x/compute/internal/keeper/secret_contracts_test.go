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
func getDecryptedData(t *testing.T, data []byte, nonce []byte) []byte {
	// data
	if len(data) == 0 {
		return data
	}

	dataCiphertextBz, err := base64.StdEncoding.DecodeString(string(data))
	require.NoError(t, err)
	dataPlaintextBase64, err := wasmCtx.Decrypt(dataCiphertextBz, nonce)
	require.NoError(t, err)

	dataPlaintext, err := base64.StdEncoding.DecodeString(string(dataPlaintextBase64))
	require.NoError(t, err)

	return dataPlaintext
}

func queryHelper(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddr sdk.AccAddress, input string) (string, cosmwasm.StdError) {
	queryBz, err := wasmCtx.Encrypt([]byte(input))
	require.NoError(t, err)
	nonce := queryBz[0:32]

	resultCipherBz, err := keeper.QuerySmart(ctx, contractAddr, queryBz)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "EnclaveErr: Got an error from the enclave") {
			return "", cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: errMsg}}
		}

		errorCipherB64 := strings.ReplaceAll(errMsg, "query wasm contract failed: generic: ", "")
		errorCipherBz, err := base64.StdEncoding.DecodeString(errorCipherB64)
		require.NoError(t, err)
		errorPlainBz, err := wasmCtx.Decrypt(errorCipherBz, nonce)
		require.NoError(t, err)

		var trueErr cosmwasm.StdError
		err = json.Unmarshal(errorPlainBz, &trueErr)
		require.NoError(t, err)

		return "", trueErr
	}

	resultPlainBz, err := wasmCtx.Decrypt(resultCipherBz, nonce)
	require.NoError(t, err)

	resultBz, err := base64.StdEncoding.DecodeString(string(resultPlainBz))
	require.NoError(t, err)

	return string(resultBz), cosmwasm.StdError{}
}

func executeHelper(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress sdk.AccAddress, txSender sdk.AccAddress, execMsg string, skipEvents uint) ([]byte, [][]cosmwasm.LogAttribute, cosmwasm.StdError) {
	execMsgBz, err := wasmCtx.Encrypt([]byte(execMsg))
	require.NoError(t, err)
	nonce := execMsgBz[0:32]

	execResult, err := keeper.Execute(ctx, contractAddress, txSender, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "EnclaveErr: Got an error from the enclave") {
			return nil, nil, cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: errMsg}}
		}

		errorCipherB64 := strings.ReplaceAll(errMsg, "execute wasm contract failed: generic: ", "")
		errorCipherBz, err := base64.StdEncoding.DecodeString(errorCipherB64)
		require.NoError(t, err)
		errorPlainBz, err := wasmCtx.Decrypt(errorCipherBz, nonce)
		require.NoError(t, err)

		var trueErr cosmwasm.StdError
		err = json.Unmarshal(errorPlainBz, &trueErr)
		require.NoError(t, err)

		return nil, nil, trueErr
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := getDecryptedWasmEvents(t, ctx, nonce, skipEvents)

	// TODO check if we can extract the messages from ctx

	// Data is the output of only the first call
	data := getDecryptedData(t, execResult.Data, nonce)

	return data, wasmEvents, cosmwasm.StdError{}
}

func initHelper(t *testing.T, keeper Keeper, ctx sdk.Context, codeID uint64, creator sdk.AccAddress, initMsg string, skipEvents uint) (sdk.AccAddress, [][]cosmwasm.LogAttribute, cosmwasm.StdError) {
	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	contractAddress, err := keeper.Instantiate(ctx, codeID, creator, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "EnclaveErr: Got an error from the enclave") {
			return nil, nil, cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: errMsg}}
		}

		errorCipherB64 := strings.ReplaceAll(err.Error(), "instantiate wasm contract failed: generic: ", "")
		errorCipherBz, err := base64.StdEncoding.DecodeString(errorCipherB64)
		require.NoError(t, err)
		errorPlainBz, err := wasmCtx.Decrypt(errorCipherBz, nonce)
		require.NoError(t, err)

		var trueErr cosmwasm.StdError
		err = json.Unmarshal(errorPlainBz, &trueErr)
		require.NoError(t, err)

		return nil, nil, trueErr
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := getDecryptedWasmEvents(t, ctx, nonce, skipEvents)

	// TODO check if we can extract the messages from ctx

	return contractAddress, wasmEvents, cosmwasm.StdError{}
}

func TestCallbackSanity(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	// init
	contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, 0)
	require.Empty(t, err)

	require.Equal(t,
		[][]cosmwasm.LogAttribute{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "init", Value: "🌈"},
			},
		},
		initEvents,
	)

	data, execEvents, err := executeHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"a":{"contract_addr":"%s","x":2,"y":3}}`, contractAddress.String()), 1)

	require.Empty(t, err)
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
	require.Equal(t, []byte{2, 3}, data)
}

func TestSanity(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	walletA := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	walletB := createFakeFundedAccount(ctx, accKeeper, topUp)

	wasmCode, err := ioutil.ReadFile("./testdata/erc20.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)

	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", deposit)
	require.NoError(t, err)

	// check init events (no data in init)
	initEvents := getDecryptedWasmEvents(t, ctx, initMsgBz[0:32], 0)
	require.Empty(t, initEvents)

	// check state after init
	qRes, qErr := queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()))
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"108"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()))
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"53"}`, qRes)

	// transfer 10 from A to B
	data, wasmEvents, err := executeHelper(t, keeper, ctx, contractAddress, walletA,
		fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String()), 0)

	require.Empty(t, err)
	require.Empty(t, data)
	require.Equal(t,
		[][]cosmwasm.LogAttribute{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "action", Value: "transfer"},
				{Key: "sender", Value: walletA.String()},
				{Key: "recipient", Value: walletB.String()},
			},
		},
		wasmEvents,
	)

	// check state after transfer
	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()))
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"98"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()))
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"63"}`, qRes)
}

func TestInitLogs(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
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
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	_, execEvents, execErr := executeHelper(t, keeper, ctx, contractAddress, walletA, `{"emptylogkeyvalue":{}}`, 1)

	require.Empty(t, execErr)
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
}

func TestEmptyData(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	data, _, err := executeHelper(t, keeper, ctx, contractAddress, walletA, `{"emptydata":{}}`, 1)

	require.Empty(t, err)
	require.Empty(t, data)
}

func TestNoData(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	data, _, err := executeHelper(t, keeper, ctx, contractAddress, walletA, `{"nodata":{}}`, 1)

	require.Empty(t, err)
	require.Empty(t, data)
}

func TestExecuteIllegalInputError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	_, _, execErr := executeHelper(t, keeper, ctx, contractAddress, walletA, `bad input`, 1)

	require.Error(t, execErr)
	require.Error(t, execErr.ParseErr)
}

func TestInitIllegalInputError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `bad input`, 0)

	require.Error(t, initErr)
	require.Error(t, initErr.ParseErr)
}

func TestInitCallback(t *testing.T) {
	t.SkipNow() // still not implemented in CosmWasm 0.9

	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	// init first contract so we'd have someone to callback
	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)
	firstContractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
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

	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
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

func TestQueryInputParamError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	walletA := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	walletB := createFakeFundedAccount(ctx, accKeeper, topUp)

	wasmCode, err := ioutil.ReadFile("./testdata/erc20.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)

	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", deposit)
	require.NoError(t, err)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"address":"blabla"}}`)
	require.Error(t, qErr)
	require.Error(t, qErr.GenericErr)
	require.Equal(t, qErr.GenericErr.Msg, "canonicalize_address returned error")
}

func TestUnicodeData(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	data, _, err := executeHelper(t, keeper, ctx, contractAddress, walletA, `{"unicodedata":{}}`, 1)

	require.Empty(t, err)
	require.Equal(t, "🍆🥑🍄", string(data))
}

func TestInitContractErrorUnicode(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	// init
	_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{}}`, 0)

	require.Equal(t, initErr.GenericErr.Msg, "Test error! 🌈")
}

func TestExecuteContractErrorUnicode(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	_, _, execErr := executeHelper(t, keeper, ctx, contractAddress, walletA, `{"contracterror":{}}`, 1)

	require.Equal(t, execErr.GenericErr.Msg, "Test error! 🌈")
}

func TestInitParamError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"callback":{"contract_addr":"notanaddress"}}`))
	require.NoError(t, err)

	// init
	_, err = keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)

	errorMsg := err.Error()
	require.Contains(t, errorMsg, "invalid address")
}

func TestCallbackExecuteParamError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	execMsgBz, err := wasmCtx.Encrypt([]byte(`{"a":{"contract_addr":"notanaddress","x":2,"y":3}}`))
	require.NoError(t, err)

	_, err = keeper.Execute(ctx, contractAddress, walletA, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)

	errorMsg := err.Error()
	require.Contains(t, errorMsg, "invalid address")
}

func TestQueryInputStructureError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	walletA := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	walletB := createFakeFundedAccount(ctx, accKeeper, topUp)

	wasmCode, err := ioutil.ReadFile("./testdata/erc20.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)

	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", deposit)
	require.NoError(t, err)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"invalidkey":"invalidval"}}`)
	require.Error(t, qErr)
	require.Error(t, qErr.ParseErr)
	require.Contains(t, qErr.ParseErr.Msg, "missing field `address`")
}

func TestQueryContractErrorUnicode(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"state":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	queryBz, err := wasmCtx.Encrypt([]byte(`{"contracterror":{}}`))
	require.NoError(t, err)

	_, err = keeper.QuerySmart(ctx, contractAddress, queryBz)

	errorCipherB64 := strings.ReplaceAll(err.Error(), "query wasm contract failed: generic: ", "")
	errorCipherBz, err := base64.StdEncoding.DecodeString(errorCipherB64)
	require.NoError(t, err)

	nonce := queryBz[0:32]
	errorPlainBz, err := wasmCtx.Decrypt(errorCipherBz, nonce)
	require.NoError(t, err)

	errorPlaintext := string(errorPlainBz)
	require.Contains(t, errorPlaintext, "Test error! 🌈")
}

func TestInitNotEncryptedInputError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsg := []byte(`{"nop":{}`)

	// init
	_, err = keeper.Instantiate(ctx, codeID, walletA, nil, initMsg, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)

	errorMsg := err.Error()
	require.Contains(t, errorMsg, "DecryptionError")
}

func TestExecuteNotEncryptedInputError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"nop":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	_, err = keeper.Execute(ctx, contractAddress, walletA, []byte(`{"emptylogkeyvalue":{}}`), sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)

	errorMsg := err.Error()
	require.Contains(t, errorMsg, "DecryptionError")
}

func TestQueryNotEncryptedInputError(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	initMsgBz, err := wasmCtx.Encrypt([]byte(`{"state":{}}`))
	require.NoError(t, err)

	// init
	contractAddress, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	query := []byte(`{"owner":{}}`)
	require.NoError(t, err)

	_, err = keeper.QuerySmart(ctx, contractAddress, query)
	require.Error(t, err)

	errorMsg := err.Error()
	require.Contains(t, errorMsg, "DecryptionError")
}
