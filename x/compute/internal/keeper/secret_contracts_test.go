package keeper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	cosmwasm "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

type ContractEvent []cosmwasm.LogAttribute

// getDecryptedWasmEvents gets all "wasm" events and decrypt what's necessary
// Returns all "wasm" events, including from contract callbacks
func getDecryptedWasmEvents(t *testing.T, ctx sdk.Context, nonce []byte, skip uint) []ContractEvent {
	events := ctx.EventManager().Events()
	var res []ContractEvent
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

func execHelper(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress sdk.AccAddress, txSender sdk.AccAddress, execMsg string, skipEvents uint) ([]byte, []ContractEvent, cosmwasm.StdError) {
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

func initHelper(t *testing.T, keeper Keeper, ctx sdk.Context, codeID uint64, creator sdk.AccAddress, initMsg string, skipEvents uint) (sdk.AccAddress, []ContractEvent, cosmwasm.StdError) {
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
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "init", Value: "🌈"},
			},
		},
		initEvents,
	)

	data, execEvents, err := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"a":{"contract_addr":"%s","x":2,"y":3}}`, contractAddress.String()), 1)

	require.Empty(t, err)
	require.Equal(t,
		[]ContractEvent{
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
	data, wasmEvents, err := execHelper(t, keeper, ctx, contractAddress, walletA,
		fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String()), 0)

	require.Empty(t, err)
	require.Empty(t, data)
	require.Equal(t,
		[]ContractEvent{
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
		[]ContractEvent{
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

	_, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `{"emptylogkeyvalue":{}}`, 1)

	require.Empty(t, execErr)
	require.Equal(t,
		[]ContractEvent{
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

	data, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"emptydata":{}}`, 1)

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

	data, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"nodata":{}}`, 1)

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

	_, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `bad input`, 1)

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

func TestCallbackFromInitAndCallbackEvents(t *testing.T) {
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

	firstContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, 0)
	require.Empty(t, initErr)

	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: firstContractAddress.String()},
				{Key: "init", Value: "🌈"},
			},
		},
		initEvents,
	)

	// init second contract and callback to the first contract
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback":{"contract_addr":"%s"}}`, firstContractAddress.String()), 1)
	require.Empty(t, initErr)

	require.Equal(t,
		[]ContractEvent{
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

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, 0)
	require.Empty(t, initErr)

	data, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"unicodedata":{}}`, 1)

	require.Empty(t, err)
	require.Equal(t, "🍆🥑🍄", string(data))
}

func TestInitContractError(t *testing.T) {
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

	t.Run("generic_err", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{"error_type":"generic_err"}}`, 0)
		require.Error(t, err)
		require.Error(t, err.GenericErr)
		require.Equal(t, err.GenericErr.Msg, "la la 🤯")
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{"error_type":"invalid_base64"}}`, 0)
		require.Error(t, err)
		require.Error(t, err.InvalidBase64)
		require.Equal(t, err.InvalidBase64.Msg, "ra ra 🤯")
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{"error_type":"invalid_utf8"}}`, 0)
		require.Error(t, err)
		require.Error(t, err.InvalidUtf8)
		require.Equal(t, err.InvalidUtf8.Msg, "ka ka 🤯")
	})
	t.Run("not_found", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{"error_type":"not_found"}}`, 0)
		require.Error(t, err)
		require.Error(t, err.NotFound)
		require.Equal(t, err.NotFound.Kind, "za za 🤯")
	})
	t.Run("null_pointer", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{"error_type":"null_pointer"}}`, 0)
		require.Error(t, err)
		require.Error(t, err.NullPointer)
	})
	t.Run("parse_err", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{"error_type":"parse_err"}}`, 0)
		require.Error(t, err)
		require.Error(t, err.ParseErr)
		require.Equal(t, err.ParseErr.Target, "na na 🤯")
		require.Equal(t, err.ParseErr.Msg, "pa pa 🤯")
	})
	t.Run("serialize_err", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{"error_type":"serialize_err"}}`, 0)
		require.Error(t, err)
		require.Error(t, err.SerializeErr)
		require.Equal(t, err.SerializeErr.Source, "ba ba 🤯")
		require.Equal(t, err.SerializeErr.Msg, "ga ga 🤯")
	})
	t.Run("unauthorized", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{"error_type":"unauthorized"}}`, 0)
		require.Error(t, err)
		require.Error(t, err.Unauthorized)
	})
	t.Run("underflow", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contracterror":{"error_type":"underflow"}}`, 0)
		require.Error(t, err)
		require.Error(t, err.Underflow)
		require.Equal(t, err.Underflow.Minuend, "minuend 🤯")
		require.Equal(t, err.Underflow.Subtrahend, "subtrahend 🤯")
	})
}

func TestExecContractError(t *testing.T) {
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

	contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, 0)
	require.Empty(t, initErr)

	t.Run("generic_err", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contracterror":{"error_type":"generic_err"}}`, 1)
		require.Error(t, err)
		require.Error(t, err.GenericErr)
		require.Equal(t, err.GenericErr.Msg, "la la 🤯")
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contracterror":{"error_type":"invalid_base64"}}`, 1)
		require.Error(t, err)
		require.Error(t, err.InvalidBase64)
		require.Equal(t, err.InvalidBase64.Msg, "ra ra 🤯")
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contracterror":{"error_type":"invalid_utf8"}}`, 1)
		require.Error(t, err)
		require.Error(t, err.InvalidUtf8)
		require.Equal(t, err.InvalidUtf8.Msg, "ka ka 🤯")
	})
	t.Run("not_found", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contracterror":{"error_type":"not_found"}}`, 1)
		require.Error(t, err)
		require.Error(t, err.NotFound)
		require.Equal(t, err.NotFound.Kind, "za za 🤯")
	})
	t.Run("null_pointer", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contracterror":{"error_type":"null_pointer"}}`, 1)
		require.Error(t, err)
		require.Error(t, err.NullPointer)
	})
	t.Run("parse_err", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contracterror":{"error_type":"parse_err"}}`, 1)
		require.Error(t, err)
		require.Error(t, err.ParseErr)
		require.Equal(t, err.ParseErr.Target, "na na 🤯")
		require.Equal(t, err.ParseErr.Msg, "pa pa 🤯")
	})
	t.Run("serialize_err", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contracterror":{"error_type":"serialize_err"}}`, 1)
		require.Error(t, err)
		require.Error(t, err.SerializeErr)
		require.Equal(t, err.SerializeErr.Source, "ba ba 🤯")
		require.Equal(t, err.SerializeErr.Msg, "ga ga 🤯")
	})
	t.Run("unauthorized", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contracterror":{"error_type":"unauthorized"}}`, 1)
		require.Error(t, err)
		require.Error(t, err.Unauthorized)
	})
	t.Run("underflow", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contracterror":{"error_type":"underflow"}}`, 1)
		require.Error(t, err)
		require.Error(t, err.Underflow)
		require.Equal(t, err.Underflow.Minuend, "minuend 🤯")
		require.Equal(t, err.Underflow.Subtrahend, "subtrahend 🤯")
	})
}

func TestQueryContractError(t *testing.T) {
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

	contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, 0)
	require.Empty(t, initErr)

	t.Run("generic_err", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contracterror":{"error_type":"generic_err"}}`)
		require.Error(t, err)
		require.Error(t, err.GenericErr)
		require.Equal(t, err.GenericErr.Msg, "la la 🤯")
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contracterror":{"error_type":"invalid_base64"}}`)
		require.Error(t, err)
		require.Error(t, err.InvalidBase64)
		require.Equal(t, err.InvalidBase64.Msg, "ra ra 🤯")
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contracterror":{"error_type":"invalid_utf8"}}`)
		require.Error(t, err)
		require.Error(t, err.InvalidUtf8)
		require.Equal(t, err.InvalidUtf8.Msg, "ka ka 🤯")
	})
	t.Run("not_found", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contracterror":{"error_type":"not_found"}}`)
		require.Error(t, err)
		require.Error(t, err.NotFound)
		require.Equal(t, err.NotFound.Kind, "za za 🤯")
	})
	t.Run("null_pointer", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contracterror":{"error_type":"null_pointer"}}`)
		require.Error(t, err)
		require.Error(t, err.NullPointer)
	})
	t.Run("parse_err", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contracterror":{"error_type":"parse_err"}}`)
		require.Error(t, err)
		require.Error(t, err.ParseErr)
		require.Equal(t, err.ParseErr.Target, "na na 🤯")
		require.Equal(t, err.ParseErr.Msg, "pa pa 🤯")
	})
	t.Run("serialize_err", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contracterror":{"error_type":"serialize_err"}}`)
		require.Error(t, err)
		require.Error(t, err.SerializeErr)
		require.Equal(t, err.SerializeErr.Source, "ba ba 🤯")
		require.Equal(t, err.SerializeErr.Msg, "ga ga 🤯")
	})
	t.Run("unauthorized", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contracterror":{"error_type":"unauthorized"}}`)
		require.Error(t, err)
		require.Error(t, err.Unauthorized)
	})
	t.Run("underflow", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contracterror":{"error_type":"underflow"}}`)
		require.Error(t, err)
		require.Error(t, err.Underflow)
		require.Equal(t, err.Underflow.Minuend, "minuend 🤯")
		require.Equal(t, err.Underflow.Subtrahend, "subtrahend 🤯")
	})
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

func TestInitNoLogs(t *testing.T) {
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
	_, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nologs":{}}`, 0)

	require.Empty(t, initErr)
	require.Empty(t, initEvents)
}

func TestExecNoLogs(t *testing.T) {
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
	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, 0)
	require.Empty(t, initErr)

	_, execEvents, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"nologs":{}}`, 1)

	require.Empty(t, err)
	require.Empty(t, execEvents)
}

func TestExecCallbackToInit(t *testing.T) {
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

	// init first contract
	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, 0)
	require.Empty(t, initErr)

	// init second contract and callback to the first contract
	execData, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"callbacktoinit":{"code_id":%d}}`, codeID), 1)
	require.Empty(t, execErr)
	require.Empty(t, execData)

	require.Equal(t, 2, len(execEvents))
	require.Equal(t,
		ContractEvent{
			{Key: "contract_address", Value: contractAddress.String()},
			{Key: "instantiating a new contract", Value: "🪂"},
		},
		execEvents[0],
	)
	require.Equal(t,
		cosmwasm.LogAttribute{Key: "init", Value: "🌈"},
		execEvents[1][1],
	)
	require.Equal(t, "contract_address", execEvents[1][0].Key)

	secondContractAddressBech32 := execEvents[1][0].Value
	secondContractAddress, err := sdk.AccAddressFromBech32(secondContractAddressBech32)
	require.NoError(t, err)

	data, execEvents, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, `{"unicodedata":{}}`, 3)

	require.Empty(t, err)
	require.Empty(t, execEvents)
	require.Equal(t, "🍆🥑🍄", string(data))
}

func TestInitCallbackToInit(t *testing.T) {
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

	// init first contract
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callbacktoinit":{"code_id":%d}}`, codeID), 0)
	require.Empty(t, initErr)

	require.Equal(t, 2, len(initEvents))
	require.Equal(t,
		ContractEvent{
			{Key: "contract_address", Value: contractAddress.String()},
			{Key: "instantiating a new contract from init!", Value: "🐙"},
		},
		initEvents[0],
	)
	require.Equal(t,
		cosmwasm.LogAttribute{Key: "init", Value: "🌈"},
		initEvents[1][1],
	)
	require.Equal(t, "contract_address", initEvents[1][0].Key)

	secondContractAddressBech32 := initEvents[1][0].Value
	secondContractAddress, err := sdk.AccAddressFromBech32(secondContractAddressBech32)
	require.NoError(t, err)

	data, execEvents, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, `{"unicodedata":{}}`, 2)

	require.Empty(t, err)
	require.Empty(t, execEvents)
	require.Equal(t, "🍆🥑🍄", string(data))
}
