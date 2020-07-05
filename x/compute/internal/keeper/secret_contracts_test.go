package keeper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	cosmwasm "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

type ContractEvent []cosmwasm.LogAttribute

func setupTest(t *testing.T, wasmPath string) (sdk.Context, Keeper, string, uint64, sdk.AccAddress, sdk.AccAddress) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	walletA := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	walletB := createFakeFundedAccount(ctx, accKeeper, topUp)

	wasmCode, err := ioutil.ReadFile(wasmPath)
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	return ctx, keeper, tempDir, codeID, walletA, walletB
}

// getDecryptedWasmEvents gets all "wasm" events and decrypt what's necessary
// Returns all "wasm" events, including from contract callbacks
func getDecryptedWasmEvents(t *testing.T, ctx sdk.Context, nonce []byte) []ContractEvent {
	events := ctx.EventManager().Events()
	var res []ContractEvent
	for _, e := range events {
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

var contractErrorRegex = regexp.MustCompile(`wasm contract failed: generic: (.+)`)

func extractInnerError(t *testing.T, err error, nonce []byte, isEncrypted bool) cosmwasm.StdError {
	match := contractErrorRegex.FindAllStringSubmatch(err.Error(), -1)

	if match == nil {
		require.True(t, !isEncrypted, "Error message should be plaintext")
		return cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: err.Error()}}
	}

	require.True(t, isEncrypted, "Error message should be encrypted")
	require.NotEmpty(t, match)
	require.Equal(t, 1, len(match))
	require.Equal(t, 2, len(match[0]))
	errorCipherB64 := match[0][1]

	errorCipherBz, err := base64.StdEncoding.DecodeString(errorCipherB64)
	require.NoError(t, err)
	errorPlainBz, err := wasmCtx.Decrypt(errorCipherBz, nonce)
	require.NoError(t, err)

	var innerErr cosmwasm.StdError
	err = json.Unmarshal(errorPlainBz, &innerErr)
	require.NoError(t, err)

	return innerErr
}

var defaultGas uint64 = 200_000

func queryHelper(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddr sdk.AccAddress, input string, isErrorEncrypted bool, gas uint64) (string, cosmwasm.StdError) {
	queryBz, err := wasmCtx.Encrypt([]byte(input))
	require.NoError(t, err)
	nonce := queryBz[0:32]

	// create new ctx with the same storage and a gas limit
	// this is to reset the event manager, so we won't get
	// events from past calls
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(sdk.NewGasMeter(gas))

	resultCipherBz, err := keeper.QuerySmart(ctx, contractAddr, queryBz)
	if err != nil {
		return "", extractInnerError(t, err, nonce, isErrorEncrypted)
	}

	resultPlainBz, err := wasmCtx.Decrypt(resultCipherBz, nonce)
	require.NoError(t, err)

	resultBz, err := base64.StdEncoding.DecodeString(string(resultPlainBz))
	require.NoError(t, err)

	return string(resultBz), cosmwasm.StdError{}
}

func execHelper(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress sdk.AccAddress, txSender sdk.AccAddress, execMsg string, isErrorEncrypted bool, gas uint64) ([]byte, []ContractEvent, cosmwasm.StdError) {
	execMsgBz, err := wasmCtx.Encrypt([]byte(execMsg))
	require.NoError(t, err)
	nonce := execMsgBz[0:32]

	// create new ctx with the same storage and a gas limit
	// this is to reset the event manager, so we won't get
	// events from past calls
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(sdk.NewGasMeter(gas))

	execResult, err := keeper.Execute(ctx, contractAddress, txSender, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	if err != nil {
		return nil, nil, extractInnerError(t, err, nonce, isErrorEncrypted)
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := getDecryptedWasmEvents(t, ctx, nonce)

	// TODO check if we can extract the messages from ctx

	// Data is the output of only the first call
	data := getDecryptedData(t, execResult.Data, nonce)

	return data, wasmEvents, cosmwasm.StdError{}
}

func initHelper(t *testing.T, keeper Keeper, ctx sdk.Context, codeID uint64, creator sdk.AccAddress, initMsg string, isErrorEncrypted bool, gas uint64) (sdk.AccAddress, []ContractEvent, cosmwasm.StdError) {
	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	// create new ctx with the same storage and a gas limit
	// this is to reset the event manager, so we won't get
	// events from past calls
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(sdk.NewGasMeter(gas))

	contractAddress, err := keeper.Instantiate(ctx, codeID, creator, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	if err != nil {
		return nil, nil, extractInnerError(t, err, nonce, isErrorEncrypted)
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := getDecryptedWasmEvents(t, ctx, nonce)

	// TODO check if we can extract the messages from ctx

	return contractAddress, wasmEvents, cosmwasm.StdError{}
}

func TestCallbackSanity(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
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

	data, execEvents, err := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"a":{"contract_addr":"%s","x":2,"y":3}}`, contractAddress.String()), true, defaultGas)

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
	ctx, keeper, tempDir, codeID, walletA, walletB := setupTest(t, "./testdata/erc20.wasm")
	defer os.RemoveAll(tempDir)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, initMsg, true, defaultGas)
	require.Empty(t, err)
	require.Empty(t, initEvents)

	// check state after init
	qRes, qErr := queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()), true, defaultGas)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"108"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()), true, defaultGas)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"53"}`, qRes)

	// transfer 10 from A to B
	data, wasmEvents, err := execHelper(t, keeper, ctx, contractAddress, walletA,
		fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String()), true, defaultGas)

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
	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()), true, defaultGas)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"98"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()), true, defaultGas)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"63"}`, qRes)
}

func TestInitLogs(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)
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
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	_, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `{"empty_log_key_value":{}}`, true, defaultGas)

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
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	data, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"empty_data":{}}`, true, defaultGas)

	require.Empty(t, err)
	require.Empty(t, data)
}

func TestNoData(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	data, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"no_data":{}}`, true, defaultGas)

	require.Empty(t, err)
	require.Empty(t, data)
}

func TestExecuteIllegalInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	_, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `bad input`, true, defaultGas)

	require.Error(t, execErr)
	require.Error(t, execErr.ParseErr)
}

func TestInitIllegalInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `bad input`, true, defaultGas)

	require.Error(t, initErr)
	require.Error(t, initErr.ParseErr)
}

func TestCallbackFromInitAndCallbackEvents(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init first contract so we'd have someone to callback
	firstContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
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
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback":{"contract_addr":"%s"}}`, firstContractAddress.String()), true, defaultGas)
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
	ctx, keeper, tempDir, codeID, walletA, walletB := setupTest(t, "./testdata/erc20.wasm")
	defer os.RemoveAll(tempDir)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, initMsg, true, defaultGas)
	require.Empty(t, err)
	require.Empty(t, initEvents)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"address":"blabla"}}`, true, defaultGas)
	require.Error(t, qErr)
	require.Error(t, qErr.GenericErr)
	require.Equal(t, "canonicalize_address returned error", qErr.GenericErr.Msg)
}

func TestUnicodeData(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	data, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"unicode_data":{}}`, true, defaultGas)

	require.Empty(t, err)
	require.Equal(t, "🍆🥑🍄", string(data))
}

func TestInitContractError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	t.Run("generic_err", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"generic_err"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.GenericErr)
		require.Equal(t, "la la 🤯", err.GenericErr.Msg)
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.InvalidBase64)
		require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.InvalidUtf8)
		require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
	})
	t.Run("not_found", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"not_found"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.NotFound)
		require.Equal(t, "za za 🤯", err.NotFound.Kind)
	})
	t.Run("null_pointer", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"null_pointer"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.NullPointer)
	})
	t.Run("parse_err", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"parse_err"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.ParseErr)
		require.Equal(t, "na na 🤯", err.ParseErr.Target)
		require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
	})
	t.Run("serialize_err", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"serialize_err"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.SerializeErr)
		require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
		require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
	})
	t.Run("unauthorized", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"unauthorized"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.Unauthorized)
	})
	t.Run("underflow", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"underflow"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.Underflow)
		require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
		require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
	})
}

func TestExecContractError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	t.Run("generic_err", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"generic_err"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.GenericErr)
		require.Equal(t, "la la 🤯", err.GenericErr.Msg)
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.InvalidBase64)
		require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.InvalidUtf8)
		require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
	})
	t.Run("not_found", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"not_found"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.NotFound)
		require.Equal(t, "za za 🤯", err.NotFound.Kind)
	})
	t.Run("null_pointer", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"null_pointer"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.NullPointer)
	})
	t.Run("parse_err", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"parse_err"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.ParseErr)
		require.Equal(t, "na na 🤯", err.ParseErr.Target)
		require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
	})
	t.Run("serialize_err", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"serialize_err"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.SerializeErr)
		require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
		require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
	})
	t.Run("unauthorized", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"unauthorized"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.Unauthorized)
	})
	t.Run("underflow", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"underflow"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.Underflow)
		require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
		require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
	})
}

func TestQueryContractError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	t.Run("generic_err", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"generic_err"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.GenericErr)
		require.Equal(t, "la la 🤯", err.GenericErr.Msg)
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"invalid_base64"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.InvalidBase64)
		require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.InvalidUtf8)
		require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
	})
	t.Run("not_found", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"not_found"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.NotFound)
		require.Equal(t, "za za 🤯", err.NotFound.Kind)
	})
	t.Run("null_pointer", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"null_pointer"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.NullPointer)
	})
	t.Run("parse_err", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"parse_err"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.ParseErr)
		require.Equal(t, "na na 🤯", err.ParseErr.Target)
		require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
	})
	t.Run("serialize_err", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"serialize_err"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.SerializeErr)
		require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
		require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
	})
	t.Run("unauthorized", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"unauthorized"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.Unauthorized)
	})
	t.Run("underflow", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"underflow"}}`, true, defaultGas)
		require.Error(t, err)
		require.Error(t, err.Underflow)
		require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
		require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
	})
}

func TestInitParamError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"callback":{"contract_addr":"notanaddress"}}`, false, defaultGas)

	require.Contains(t, initErr.Error(), "invalid address")
}

func TestCallbackExecuteParamError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	_, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"a":{"contract_addr":"notanaddress","x":2,"y":3}}`, false, defaultGas)

	require.Contains(t, err.Error(), "invalid address")
}

func TestQueryInputStructureError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, walletB := setupTest(t, "./testdata/erc20.wasm")
	defer os.RemoveAll(tempDir)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, initMsg, true, defaultGas)
	require.Empty(t, err)
	require.Empty(t, initEvents)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"invalidkey":"invalidval"}}`, true, defaultGas)
	require.Error(t, qErr)
	require.Error(t, qErr.ParseErr)
	require.Contains(t, qErr.ParseErr.Msg, "missing field `address`")
}

func TestInitNotEncryptedInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := []byte(`{"nop":{}`)

	// init
	_, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsg, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)

	require.Contains(t, err.Error(), "DecryptionError")
}

func TestExecuteNotEncryptedInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	_, err := keeper.Execute(ctx, contractAddress, walletA, []byte(`{"empty_log_key_value":{}}`), sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)

	require.Contains(t, err.Error(), "DecryptionError")
}

func TestQueryNotEncryptedInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	_, err := keeper.QuerySmart(ctx, contractAddress, []byte(`{"owner":{}}`))
	require.Error(t, err)

	require.Contains(t, err.Error(), "DecryptionError")
}

func TestInitNoLogs(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	_, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"no_logs":{}}`, true, defaultGas)

	require.Empty(t, initErr)
	require.Empty(t, initEvents)
}

func TestExecNoLogs(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	_, execEvents, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"no_logs":{}}`, true, defaultGas)

	require.Empty(t, err)
	require.Empty(t, execEvents)
}

func TestExecCallbackToInit(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init first contract
	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	// init second contract and callback to the first contract
	execData, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d}}`, codeID), true, defaultGas)
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

	data, execEvents, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, `{"unicode_data":{}}`, true, defaultGas)

	require.Empty(t, err)
	require.Empty(t, execEvents)
	require.Equal(t, "🍆🥑🍄", string(data))
}

func TestInitCallbackToInit(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d}}`, codeID), true, defaultGas)
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

	data, execEvents, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, `{"unicode_data":{}}`, true, defaultGas)

	require.Empty(t, err)
	require.Empty(t, execEvents)
	require.Equal(t, "🍆🥑🍄", string(data))
}

func TestInitCallbackContratError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	secondContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s"}}`, contractAddress), true, defaultGas)
	require.Error(t, initErr)
	require.Error(t, initErr.GenericErr)
	require.Equal(t, "la la 🤯", initErr.GenericErr.Msg)
	require.Empty(t, secondContractAddress)
	require.Empty(t, initEvents)
}

func TestExecCallbackContratError(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	data, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s"}}`, contractAddress), true, defaultGas)
	require.Error(t, execErr)
	require.Error(t, execErr.GenericErr)
	require.Equal(t, "la la 🤯", execErr.GenericErr.Msg)
	require.Empty(t, execEvents)
	require.Empty(t, data)
}

func TestExecCallbackBadParam(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	data, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, defaultGas)
	require.Error(t, execErr)
	require.Error(t, execErr.ParseErr)
	require.Equal(t, "test_contract::contract::HandleMsg", execErr.ParseErr.Target)
	require.Contains(t, execErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
	require.Empty(t, execEvents)
	require.Empty(t, data)
}

func TestInitCallbackBadParam(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init first
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	secondContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, defaultGas)
	require.Empty(t, secondContractAddress)
	require.Empty(t, initEvents)
	require.Error(t, initErr)
	require.Error(t, initErr.ParseErr)
	require.Equal(t, "test_contract::contract::InitMsg", initErr.ParseErr.Target)
	require.Contains(t, initErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
}

func TestState(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	data, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `{"get_state":{"key":"banana"}}`, true, defaultGas)
	require.Empty(t, execErr)
	require.Empty(t, data)

	_, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, defaultGas)
	require.Empty(t, execErr)

	data, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, `{"get_state":{"key":"banana"}}`, true, defaultGas)
	require.Empty(t, execErr)
	require.Equal(t, "🍌", string(data))

	_, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, `{"remove_state":{"key":"banana"}}`, true, defaultGas)
	require.Empty(t, execErr)

	data, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, `{"get_state":{"key":"banana"}}`, true, defaultGas)
	require.Empty(t, execErr)
	require.Empty(t, data)
}

func TestCanonicalizeAddressErrors(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	// this function should handle errors internally and return gracefully
	data, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `{"test_canonicalize_address_errors":{}}`, true, defaultGas)
	require.Empty(t, execErr)
	require.Equal(t, "🤟", string(data))
}

func TestInitPanic(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"panic":{}}`, false, defaultGas)
	require.Error(t, initErr)
	require.Error(t, initErr.GenericErr)
	require.Equal(t, "instantiate wasm contract failed: Error calling the VM: EnclaveErr: Got an error from the enclave: FailedFunctionCall", initErr.GenericErr.Msg)
}

func TestExecPanic(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	_, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"panic":{}}`, false, defaultGas)
	require.Error(t, execErr)
	require.Error(t, execErr.GenericErr)
	require.Equal(t, "execute wasm contract failed: Error calling the VM: EnclaveErr: Got an error from the enclave: FailedFunctionCall", execErr.GenericErr.Msg)
}

func TestQueryPanic(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	_, queryErr := queryHelper(t, keeper, ctx, addr, `{"panic":{}}`, false, defaultGas)
	require.Error(t, queryErr)
	require.Error(t, queryErr.GenericErr)
	require.Equal(t, "query wasm contract failed: Error calling the VM: EnclaveErr: Got an error from the enclave: FailedFunctionCall", queryErr.GenericErr.Msg)
}

func TestAllocateOnHeapFailBecauseMemoryLimit(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	data, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"allocate_on_heap":{"bytes":13631488}}`, false, defaultGas)

	// this should fail with memory error because 13MiB is more than the allowed 12MiB

	require.Empty(t, data)
	require.Error(t, execErr)
	require.Error(t, execErr.GenericErr)
	require.Equal(t, "execute wasm contract failed: Error calling the VM: EnclaveErr: Got an error from the enclave: FailedFunctionCall", execErr.GenericErr.Msg)
}

func TestAllocateOnHeapFailBecauseGasLimit(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	data, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, defaultGas)

	// this should fail with out of gas because 1GiB will ask for
	// 134,217,728 gas units (8192 per page times 16,384 pages)
	// the default gas limit in ctx is 200,000 which translates into
	// 20,000,000 WASM gas units, so before the memory_grow opcode is reached
	// the gas metering sees a request that'll cost 134mn and the limit
	// is 20mn, so it throws an out of gas exception

	require.Empty(t, data)
	require.Error(t, execErr)
	require.Error(t, execErr.GenericErr)
	require.Equal(t, "execute wasm contract failed: Ran out of gas", execErr.GenericErr.Msg)
}

func TestAllocateOnHeapMoreThanSGXHasFailBecauseMemoryLimit(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	data, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, 9_000_000)

	// this should fail with memory error because 1GiB is more
	// than the allowed 12MiB, gas is 9mn so WASM gas is 900mn
	// which is bigger than the 134mn from the previous test

	require.Empty(t, data)
	require.Error(t, execErr)
	require.Error(t, execErr.GenericErr)
	require.Equal(t, "execute wasm contract failed: Error calling the VM: EnclaveErr: Got an error from the enclave: FailedFunctionCall", execErr.GenericErr.Msg)
}

func TestPassNullPointerToImports(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGas)
	require.Empty(t, initErr)

	tests := []string{
		"read_db_key",
		"write_db_key",
		"write_db_value",
		"remove_db_key",
		"canonicalize_address_input",
		"humanize_address_input",
	}

	for _, passType := range tests {
		t.Run(passType, func(t *testing.T) {
			_, _, execErr := execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"pass_null_pointer_to_imports_should_throw":{"pass_type":"%s"}}`, passType), false, defaultGas)

			require.Error(t, execErr)
			require.Error(t, execErr.GenericErr)
			require.Equal(t, "execute wasm contract failed: Error calling the VM: EnclaveErr: Got an error from the enclave: Unknown", execErr.GenericErr.Msg)
		})
	}
}
