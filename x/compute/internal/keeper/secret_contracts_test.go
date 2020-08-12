package keeper

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"

	cosmwasm "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

type ContractEvent []cosmwasm.LogAttribute

// if codeID isn't 0, it will try to use that. Otherwise will take the contractAddress
func testEncrypt(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress sdk.AccAddress, codeId uint64, msg []byte) ([]byte, error) {

	var hash []byte
	if codeId != 0 {
		hash = keeper.GetCodeInfo(ctx, codeId).CodeHash
	} else {
		hash = keeper.GetContractHash(ctx, contractAddress)
	}

	if hash == nil {
		return nil, cosmwasm.StdError{}
	}

	intMsg := types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(hash)),
		Msg:      msg,
	}

	queryBz, err := wasmCtx.Encrypt(intMsg.Serialize())
	require.NoError(t, err)

	return queryBz, nil
}

func setupTest(t *testing.T, wasmPath string) (sdk.Context, Keeper, string, uint64, string, sdk.AccAddress, sdk.AccAddress) {
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

	codeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, codeID).CodeHash)

	return ctx, keeper, tempDir, codeID, codeHash, walletA, walletB
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

var contractErrorRegex = regexp.MustCompile(`contract failed: encrypted: (.+)`)

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

const defaultGasForTests uint64 = 200_000

// wrap the defualt gas meter with a counter of wasm calls
// in order to verify that every wasm call consumes gas
type WasmCounterGasMeter struct {
	wasmCounter uint64
	gasMeter    sdk.GasMeter
}

func (wasmGasMeter *WasmCounterGasMeter) GasConsumed() sdk.Gas {
	return wasmGasMeter.gasMeter.GasConsumed()
}
func (wasmGasMeter *WasmCounterGasMeter) GasConsumedToLimit() sdk.Gas {
	return wasmGasMeter.gasMeter.GasConsumedToLimit()
}
func (wasmGasMeter *WasmCounterGasMeter) Limit() sdk.Gas {
	return wasmGasMeter.gasMeter.Limit()
}
func (wasmGasMeter *WasmCounterGasMeter) ConsumeGas(amount sdk.Gas, descriptor string) {
	if descriptor == "wasm contract" && amount > 0 {
		wasmGasMeter.wasmCounter++
	}
	wasmGasMeter.gasMeter.ConsumeGas(amount, descriptor)
}
func (wasmGasMeter *WasmCounterGasMeter) IsPastLimit() bool {
	return wasmGasMeter.gasMeter.IsPastLimit()
}
func (wasmGasMeter *WasmCounterGasMeter) IsOutOfGas() bool {
	return wasmGasMeter.gasMeter.IsOutOfGas()
}
func (wasmGasMeter *WasmCounterGasMeter) GetWasmCounter() uint64 {
	return wasmGasMeter.wasmCounter
}

var _ sdk.GasMeter = (*WasmCounterGasMeter)(nil) // check interface

func queryHelper(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddr sdk.AccAddress, input string, isErrorEncrypted bool, gas uint64) (string, cosmwasm.StdError) {
	return queryHelperImpl(t, keeper, ctx, contractAddr, input, isErrorEncrypted, gas, -1)
}

func queryHelperImpl(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddr sdk.AccAddress, input string, isErrorEncrypted bool, gas uint64, wasmCallCount int64) (string, cosmwasm.StdError) {
	hashStr := hex.EncodeToString(keeper.GetContractHash(ctx, contractAddr))

	msg := types.SecretMsg{
		CodeHash: []byte(hashStr),
		Msg:      []byte(input),
	}

	queryBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := queryBz[0:32]

	// create new ctx with the same storage and set our gas meter
	// this is to reset the event manager, so we won't get
	// events from past calls
	gasMeter := &WasmCounterGasMeter{0, sdk.NewGasMeter(gas)}
	ctx = sdk.NewContext(
		ctx.MultiStore(),
		ctx.BlockHeader(),
		ctx.IsCheckTx(),
		log.NewNopLogger(),
	).WithGasMeter(gasMeter)

	resultCipherBz, err := keeper.QuerySmart(ctx, contractAddr, queryBz, true)

	if wasmCallCount < 0 {
		// default, just check that at least 1 call happend
		require.NotZero(t, gasMeter.GetWasmCounter())
	} else {
		require.Equal(t, uint64(wasmCallCount), gasMeter.GetWasmCounter())
	}

	if err != nil {
		return "", extractInnerError(t, err, nonce, isErrorEncrypted)
	}

	resultPlainBz, err := wasmCtx.Decrypt(resultCipherBz, nonce)
	require.NoError(t, err)

	resultBz, err := base64.StdEncoding.DecodeString(string(resultPlainBz))
	require.NoError(t, err)

	return string(resultBz), cosmwasm.StdError{}
}

func execHelper(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress sdk.AccAddress, txSender sdk.AccAddress, execMsg string, isErrorEncrypted bool, gas uint64, coin int64) ([]byte, []ContractEvent, cosmwasm.StdError) {
	return execHelperImpl(t, keeper, ctx, contractAddress, txSender, execMsg, isErrorEncrypted, gas, coin, -1)
}

func execHelperImpl(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress sdk.AccAddress, txSender sdk.AccAddress, execMsg string, isErrorEncrypted bool, gas uint64, coin int64, wasmCallCount int64) ([]byte, []ContractEvent, cosmwasm.StdError) {
	hashStr := hex.EncodeToString(keeper.GetContractHash(ctx, contractAddress))

	msg := types.SecretMsg{
		CodeHash: []byte(hashStr),
		Msg:      []byte(execMsg),
	}

	execMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := execMsgBz[0:32]

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

	execResult, err := keeper.Execute(ctx, contractAddress, txSender, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", coin)))

	if wasmCallCount < 0 {
		// default, just check that at least 1 call happend
		require.NotZero(t, gasMeter.GetWasmCounter())
	} else {
		require.Equal(t, uint64(wasmCallCount), gasMeter.GetWasmCounter())
	}

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
	return initHelperImpl(t, keeper, ctx, codeID, creator, initMsg, isErrorEncrypted, gas, -1)
}

func initHelperImpl(t *testing.T, keeper Keeper, ctx sdk.Context, codeID uint64, creator sdk.AccAddress, initMsg string, isErrorEncrypted bool, gas uint64, wasmCallCount int64) (sdk.AccAddress, []ContractEvent, cosmwasm.StdError) {
	hashStr := hex.EncodeToString(keeper.GetCodeInfo(ctx, codeID).CodeHash)

	msg := types.SecretMsg{
		CodeHash: []byte(hashStr),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

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

	// make the label a random base64 string, because why not?
	contractAddress, err := keeper.Instantiate(ctx, codeID, creator, nil, initMsgBz, base64.RawURLEncoding.EncodeToString(nonce), sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	if wasmCallCount < 0 {
		// default, just check that at least 1 call happend
		require.NotZero(t, gasMeter.GetWasmCounter())
	} else {
		require.Equal(t, uint64(wasmCallCount), gasMeter.GetWasmCounter())
	}

	if err != nil {
		return nil, nil, extractInnerError(t, err, nonce, isErrorEncrypted)
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := getDecryptedWasmEvents(t, ctx, nonce)

	// TODO check if we can extract the messages from ctx

	return contractAddress, wasmEvents, cosmwasm.StdError{}
}

func TestCallbackSanity(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
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

	data, execEvents, err := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":2,"y":3}}`, contractAddress.String(), codeHash), true, defaultGasForTests, 0)
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
	ctx, keeper, tempDir, codeID, _, walletA, walletB := setupTest(t, "./testdata/erc20.wasm")
	defer os.RemoveAll(tempDir)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, initMsg, true, defaultGasForTests)
	require.Empty(t, err)
	require.Empty(t, initEvents)

	// check state after init
	qRes, qErr := queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()), true, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"108"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()), true, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"53"}`, qRes)

	// transfer 10 from A to B
	data, wasmEvents, err := execHelper(t, keeper, ctx, contractAddress, walletA,
		fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String()), true, defaultGasForTests, 0)

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
	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()), true, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"98"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()), true, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"63"}`, qRes)
}

func TestInitLogs(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
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
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `{"empty_log_key_value":{}}`, true, defaultGasForTests, 0)

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
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	data, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"empty_data":{}}`, true, defaultGasForTests, 0)

	require.Empty(t, err)
	require.Empty(t, data)
}

func TestNoData(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	data, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"no_data":{}}`, true, defaultGasForTests, 0)

	require.Empty(t, err)
	require.Empty(t, data)
}

func TestExecuteIllegalInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `bad input`, true, defaultGasForTests, 0)

	require.NotNil(t, execErr.ParseErr)
}

func TestInitIllegalInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `bad input`, true, defaultGasForTests)

	require.NotNil(t, initErr.ParseErr)
}

func TestCallbackFromInitAndCallbackEvents(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init first contract so we'd have someone to callback
	firstContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
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
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback":{"contract_addr":"%s", "code_hash": "%s"}}`, firstContractAddress.String(), codeHash), true, defaultGasForTests)
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
	ctx, keeper, tempDir, codeID, _, walletA, walletB := setupTest(t, "./testdata/erc20.wasm")
	defer os.RemoveAll(tempDir)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, initMsg, true, defaultGasForTests)
	require.Empty(t, err)
	require.Empty(t, initEvents)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"address":"blabla"}}`, true, defaultGasForTests)

	require.NotNil(t, qErr.GenericErr)
	require.Equal(t, "canonicalize_address returned error", qErr.GenericErr.Msg)
}

func TestUnicodeData(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	data, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"unicode_data":{}}`, true, defaultGasForTests, 0)

	require.Empty(t, err)
	require.Equal(t, "🍆🥑🍄", string(data))
}

func TestInitContractError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	t.Run("generic_err", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"generic_err"}}`, true, defaultGasForTests)

		require.NotNil(t, err.GenericErr)
		require.Equal(t, "la la 🤯", err.GenericErr.Msg)
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, defaultGasForTests)

		require.NotNil(t, err.InvalidBase64)
		require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, defaultGasForTests)

		require.NotNil(t, err.InvalidUtf8)
		require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
	})
	t.Run("not_found", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"not_found"}}`, true, defaultGasForTests)

		require.NotNil(t, err.NotFound)
		require.Equal(t, "za za 🤯", err.NotFound.Kind)
	})
	t.Run("null_pointer", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"null_pointer"}}`, true, defaultGasForTests)

		require.NotNil(t, err.NullPointer)
	})
	t.Run("parse_err", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"parse_err"}}`, true, defaultGasForTests)

		require.NotNil(t, err.ParseErr)
		require.Equal(t, "na na 🤯", err.ParseErr.Target)
		require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
	})
	t.Run("serialize_err", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"serialize_err"}}`, true, defaultGasForTests)

		require.NotNil(t, err.SerializeErr)
		require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
		require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
	})
	t.Run("unauthorized", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"unauthorized"}}`, true, defaultGasForTests)

		require.NotNil(t, err.Unauthorized)
	})
	t.Run("underflow", func(t *testing.T) {
		_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"contract_error":{"error_type":"underflow"}}`, true, defaultGasForTests)

		require.NotNil(t, err.Underflow)
		require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
		require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
	})
}

func TestExecContractError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	t.Run("generic_err", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"generic_err"}}`, true, defaultGasForTests, 0)

		require.NotNil(t, err.GenericErr)
		require.Equal(t, "la la 🤯", err.GenericErr.Msg)
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, defaultGasForTests, 0)

		require.NotNil(t, err.InvalidBase64)
		require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, defaultGasForTests, 0)

		require.NotNil(t, err.InvalidUtf8)
		require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
	})
	t.Run("not_found", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"not_found"}}`, true, defaultGasForTests, 0)

		require.NotNil(t, err.NotFound)
		require.Equal(t, "za za 🤯", err.NotFound.Kind)
	})
	t.Run("null_pointer", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"null_pointer"}}`, true, defaultGasForTests, 0)

		require.NotNil(t, err.NullPointer)
	})
	t.Run("parse_err", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"parse_err"}}`, true, defaultGasForTests, 0)

		require.NotNil(t, err.ParseErr)
		require.Equal(t, "na na 🤯", err.ParseErr.Target)
		require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
	})
	t.Run("serialize_err", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"serialize_err"}}`, true, defaultGasForTests, 0)

		require.NotNil(t, err.SerializeErr)
		require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
		require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
	})
	t.Run("unauthorized", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"unauthorized"}}`, true, defaultGasForTests, 0)

		require.NotNil(t, err.Unauthorized)
	})
	t.Run("underflow", func(t *testing.T) {
		_, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, `{"contract_error":{"error_type":"underflow"}}`, true, defaultGasForTests, 0)

		require.NotNil(t, err.Underflow)
		require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
		require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
	})
}

func TestQueryContractError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	t.Run("generic_err", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"generic_err"}}`, true, defaultGasForTests)

		require.NotNil(t, err.GenericErr)
		require.Equal(t, "la la 🤯", err.GenericErr.Msg)
	})
	t.Run("invalid_base64", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"invalid_base64"}}`, true, defaultGasForTests)

		require.NotNil(t, err.InvalidBase64)
		require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, defaultGasForTests)

		require.NotNil(t, err.InvalidUtf8)
		require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
	})
	t.Run("not_found", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"not_found"}}`, true, defaultGasForTests)

		require.NotNil(t, err.NotFound)
		require.Equal(t, "za za 🤯", err.NotFound.Kind)
	})
	t.Run("null_pointer", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"null_pointer"}}`, true, defaultGasForTests)

		require.NotNil(t, err.NullPointer)
	})
	t.Run("parse_err", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"parse_err"}}`, true, defaultGasForTests)

		require.NotNil(t, err.ParseErr)
		require.Equal(t, "na na 🤯", err.ParseErr.Target)
		require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
	})
	t.Run("serialize_err", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"serialize_err"}}`, true, defaultGasForTests)

		require.NotNil(t, err.SerializeErr)
		require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
		require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
	})
	t.Run("unauthorized", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"unauthorized"}}`, true, defaultGasForTests)

		require.NotNil(t, err.Unauthorized)
	})
	t.Run("underflow", func(t *testing.T) {
		_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"underflow"}}`, true, defaultGasForTests)

		require.NotNil(t, err.Underflow)
		require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
		require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
	})
}

func TestInitParamError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	codeHash := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	msg := fmt.Sprintf(`{"callback":{"contract_addr":"notanaddress", "code_hash":"%s"}}`, codeHash)

	_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, msg, false, defaultGasForTests)

	require.Contains(t, initErr.Error(), "invalid address")
}

func TestCallbackExecuteParamError(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	msg := fmt.Sprintf(`{"a":{"code_hash":"%s","contract_addr":"notanaddress","x":2,"y":3}}`, codeHash)

	_, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, msg, false, defaultGasForTests, 0)

	require.Contains(t, err.Error(), "invalid address")
}

func TestQueryInputStructureError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, walletB := setupTest(t, "./testdata/erc20.wasm")
	defer os.RemoveAll(tempDir)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, initMsg, true, defaultGasForTests)
	require.Empty(t, err)
	require.Empty(t, initEvents)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"invalidkey":"invalidval"}}`, true, defaultGasForTests)

	require.NotNil(t, qErr.ParseErr)
	require.Contains(t, qErr.ParseErr.Msg, "missing field `address`")
}

func TestInitNotEncryptedInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := []byte(`{"nop":{}`)

	// init
	_, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsg, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)

	require.Contains(t, err.Error(), "failed to decrypt data")
}

func TestExecuteNotEncryptedInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, err := keeper.Execute(ctx, contractAddress, walletA, []byte(`{"empty_log_key_value":{}}`), sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)

	require.Contains(t, err.Error(), "failed to decrypt data")
}

func TestQueryNotEncryptedInputError(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, err := keeper.QuerySmart(ctx, contractAddress, []byte(`{"owner":{}}`), false)
	require.Error(t, err)

	require.Contains(t, err.Error(), "failed to decrypt data")
}

func TestInitNoLogs(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	_, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"no_logs":{}}`, true, defaultGasForTests)

	require.Empty(t, initErr)
	require.Empty(t, initEvents)
}

func TestExecNoLogs(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, execEvents, err := execHelper(t, keeper, ctx, contractAddress, walletA, `{"no_logs":{}}`, true, defaultGasForTests, 0)

	require.Empty(t, err)
	require.Empty(t, execEvents)
}

func TestExecCallbackToInit(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init first contract
	contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	// init second contract and callback to the first contract
	execData, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d,"code_hash":"%s"}}`, codeID, codeHash), true, defaultGasForTests, 0)
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

	data, execEvents, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, `{"unicode_data":{}}`, true, defaultGasForTests, 0)

	require.Empty(t, err)
	require.Empty(t, execEvents)
	require.Equal(t, "🍆🥑🍄", string(data))
}

func TestInitCallbackToInit(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, defaultGasForTests)
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

	data, execEvents, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, `{"unicode_data":{}}`, true, defaultGasForTests, 0)

	require.Empty(t, err)
	require.Empty(t, execEvents)
	require.Equal(t, "🍆🥑🍄", string(data))
}

func TestInitCallbackContractError(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	secondContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s", "code_hash":"%s"}}`, contractAddress, codeHash), true, defaultGasForTests)

	require.NotNil(t, initErr.GenericErr)
	require.Equal(t, "la la 🤯", initErr.GenericErr.Msg)
	require.Empty(t, secondContractAddress)
	require.Empty(t, initEvents)
}

func TestExecCallbackContractError(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	data, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s","code_hash":"%s"}}`, contractAddress, codeHash), true, defaultGasForTests, 0)

	require.NotNil(t, execErr.GenericErr)
	require.Equal(t, "la la 🤯", execErr.GenericErr.Msg)
	require.Empty(t, execEvents)
	require.Empty(t, data)
}

func TestExecCallbackBadParam(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	data, execEvents, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, defaultGasForTests, 0)

	require.NotNil(t, execErr.ParseErr)
	require.Equal(t, "test_contract::contract::HandleMsg", execErr.ParseErr.Target)
	require.Contains(t, execErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
	require.Empty(t, execEvents)
	require.Empty(t, data)
}

func TestInitCallbackBadParam(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init first
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	secondContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, defaultGasForTests)
	require.Empty(t, secondContractAddress)
	require.Empty(t, initEvents)

	require.NotNil(t, initErr.ParseErr)
	require.Equal(t, "test_contract::contract::InitMsg", initErr.ParseErr.Target)
	require.Contains(t, initErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
}

func TestState(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	// init
	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	data, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `{"get_state":{"key":"banana"}}`, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, data)

	_, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, defaultGasForTests, 0)
	require.Empty(t, execErr)

	data, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, `{"get_state":{"key":"banana"}}`, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Equal(t, "🍌", string(data))

	_, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, `{"remove_state":{"key":"banana"}}`, true, defaultGasForTests, 0)
	require.Empty(t, execErr)

	data, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, `{"get_state":{"key":"banana"}}`, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, data)
}

func TestCanonicalizeAddressErrors(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)
	require.Equal(t, 1, len(initEvents))

	// this function should handle errors internally and return gracefully
	data, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, `{"test_canonicalize_address_errors":{}}`, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Equal(t, "🤟", string(data))
}

func TestInitPanic(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"panic":{}}`, false, defaultGasForTests)

	require.NotNil(t, initErr.GenericErr)
	require.Equal(t, "instantiate contract failed: Execution error: Enclave: the contract panicked", initErr.GenericErr.Msg)
}

func TestExecPanic(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"panic":{}}`, false, defaultGasForTests, 0)

	require.NotNil(t, execErr.GenericErr)
	require.Equal(t, "execute contract failed: Execution error: Enclave: the contract panicked", execErr.GenericErr.Msg)
}

func TestQueryPanic(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, queryErr := queryHelper(t, keeper, ctx, addr, `{"panic":{}}`, false, defaultGasForTests)
	require.NotNil(t, queryErr.GenericErr)
	require.Equal(t, "query contract failed: Execution error: Enclave: the contract panicked", queryErr.GenericErr.Msg)
}

func TestAllocateOnHeapFailBecauseMemoryLimit(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	data, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"allocate_on_heap":{"bytes":13631488}}`, false, defaultGasForTests, 0)

	// this should fail with memory error because 13MiB is more than the allowed 12MiB

	require.Empty(t, data)

	require.NotNil(t, execErr.GenericErr)
	require.Equal(t, "execute contract failed: Execution error: Enclave: the contract panicked", execErr.GenericErr.Msg)
}

func TestAllocateOnHeapFailBecauseGasLimit(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	// ensure we get an out of gas panic
	defer func() {
		r := recover()
		require.NotNil(t, r)
		_, ok := r.(sdk.ErrorOutOfGas)
		require.True(t, ok, "%v", r)
	}()

	_, _, _ = execHelper(t, keeper, ctx, addr, walletA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, defaultGasForTests, 0)

	// this should fail with out of gas because 1GiB will ask for
	// 134,217,728 gas units (8192 per page times 16,384 pages)
	// the default gas limit in ctx is 200,000 which translates into
	// 20,000,000 WASM gas units, so before the memory_grow opcode is reached
	// the gas metering sees a request that'll cost 134mn and the limit
	// is 20mn, so it throws an out of gas exception

	require.True(t, false)
}

func TestAllocateOnHeapMoreThanSGXHasFailBecauseMemoryLimit(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	data, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, 9_000_000, 0)

	// this should fail with memory error because 1GiB is more
	// than the allowed 12MiB, gas is 9mn so WASM gas is 900mn
	// which is bigger than the 134mn from the previous test

	require.Empty(t, data)

	require.NotNil(t, execErr.GenericErr)
	require.Equal(t, "execute contract failed: Execution error: Enclave: the contract panicked", execErr.GenericErr.Msg)
}

func TestPassNullPointerToImports(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
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
			_, _, execErr := execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"pass_null_pointer_to_imports_should_throw":{"pass_type":"%s"}}`, passType), false, defaultGasForTests, 0)

			require.NotNil(t, execErr.GenericErr)
			require.Equal(t, "execute contract failed: Execution error: Enclave: failed to read memory", execErr.GenericErr.Msg)
		})
	}
}

func TestExternalQueryWorks(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	data, _, execErr := execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_external_query":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests, 0)

	require.Empty(t, execErr)
	require.Equal(t, []byte{3}, data)
}

func TestExternalQueryCalleePanic(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, err = execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_external_query_panic":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests, 0)

	require.NotNil(t, err.GenericErr)
	require.Equal(t, "query contract failed: Execution error: Enclave: the contract panicked", err.GenericErr.Msg)
}

func TestExternalQueryCalleeStdError(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, err = execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_external_query_error":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests, 0)

	require.NotNil(t, err.GenericErr)
	require.Equal(t, "la la 🤯", err.GenericErr.Msg)
}

func TestExternalQueryCalleeDoesntExist(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, err = execHelper(t, keeper, ctx, addr, walletA, `{"send_external_query_error":{"to":"secret13l72vhjngmg55ykajxdnlalktwglyqjqv9pkq4","code_hash":"bla bla"}}`, true, defaultGasForTests, 0)

	require.NotNil(t, err.GenericErr)
	require.Equal(t, "not found: contract", err.GenericErr.Msg)
}

func TestExternalQueryBadSenderABI(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, err = execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_external_query_bad_abi":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests, 0)

	require.NotNil(t, err.ParseErr)
	require.Equal(t, "test_contract::contract::QueryMsg", err.ParseErr.Target)
	require.Equal(t, "Invalid type", err.ParseErr.Msg)
}

func TestExternalQueryBadReceiverABI(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, err = execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_external_query_bad_abi_receiver":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests, 0)

	require.NotNil(t, err.ParseErr)
	require.Equal(t, "alloc::string::String", err.ParseErr.Target)
	require.Equal(t, "Invalid type", err.ParseErr.Msg)
}

func TestMsgSenderInCallback(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	_, events, err := execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"callback_to_log_msg_sender":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests, 0)

	require.Empty(t, err)
	require.Equal(t, []ContractEvent{
		{
			{Key: "contract_address", Value: addr.String()},
			{Key: "hi", Value: "hey"}},
		{
			{Key: "contract_address", Value: addr.String()},
			{Key: "msg.sender", Value: addr.String()},
		},
	}, events)
}

func TestInfiniteQueryLoopKilledGracefullyByOOM(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	data, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"send_external_query_infinite_loop":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests)

	require.Empty(t, data)
	require.NotNil(t, err.GenericErr)
	require.Equal(t, err.GenericErr.Msg, "query contract failed: Execution error: Enclave: enclave ran out of heap memory")
}

func TestWriteToStorageDuringQuery(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, queryErr := queryHelper(t, keeper, ctx, addr, `{"write_to_storage": {}}`, false, defaultGasForTests)
	require.NotNil(t, queryErr.GenericErr)
	require.Equal(t, "query contract failed: Execution error: Enclave: contract tried to write to storage during a query", queryErr.GenericErr.Msg)
}

func TestRemoveFromStorageDuringQuery(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, queryErr := queryHelper(t, keeper, ctx, addr, `{"remove_from_storage": {}}`, false, defaultGasForTests)
	require.NotNil(t, queryErr.GenericErr)
	require.Equal(t, "query contract failed: Execution error: Enclave: contract tried to write to storage during a query", queryErr.GenericErr.Msg)
}

func TestDepositToContract(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	contractCoinsBefore := keeper.bankKeeper.GetCoins(ctx, addr)
	walletCointsBefore := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "", contractCoinsBefore.String())
	require.Equal(t, "200000denom", walletCointsBefore.String())

	data, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"deposit_to_contract":{}}`, false, defaultGasForTests, 17)

	require.Empty(t, execErr)

	contractCoinsAfter := keeper.bankKeeper.GetCoins(ctx, addr)
	walletCointsAfter := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "17denom", contractCoinsAfter.String())
	require.Equal(t, "199983denom", walletCointsAfter.String())

	require.Equal(t, `[{"denom":"denom","amount":"17"}]`, string(data))
}

func TestContractSendFunds(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"deposit_to_contract":{}}`, false, defaultGasForTests, 17)

	require.Empty(t, execErr)

	contractCoinsBefore := keeper.bankKeeper.GetCoins(ctx, addr)
	walletCointsBefore := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "17denom", contractCoinsBefore.String())
	require.Equal(t, "199983denom", walletCointsBefore.String())

	_, _, execErr = execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, addr.String(), walletA.String(), "denom", 17), false, defaultGasForTests, 0)

	contractCoinsAfter := keeper.bankKeeper.GetCoins(ctx, addr)
	walletCointsAfter := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "", contractCoinsAfter.String())
	require.Equal(t, "200000denom", walletCointsAfter.String())

	require.Empty(t, execErr)
}

func TestContractTryToSendFundsFromSomeoneElse(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"deposit_to_contract":{}}`, false, defaultGasForTests, 17)

	require.Empty(t, execErr)

	contractCoinsBefore := keeper.bankKeeper.GetCoins(ctx, addr)
	walletCointsBefore := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "17denom", contractCoinsBefore.String())
	require.Equal(t, "199983denom", walletCointsBefore.String())

	_, _, execErr = execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, walletA.String(), addr.String(), "denom", 17), false, defaultGasForTests, 0)

	require.NotNil(t, execErr.GenericErr)
	require.Equal(t, "unauthorized: contract doesn't have permission", execErr.GenericErr.Msg)
}

func TestContractSendFundsToInitCallback(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	contractCoinsBefore := keeper.bankKeeper.GetCoins(ctx, addr)
	walletCointsBefore := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "", contractCoinsBefore.String())
	require.Equal(t, "200000denom", walletCointsBefore.String())

	_, execEvents, execErr := execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_funds_to_init_callback":{"code_id":%d,"denom":"%s","amount":%d,"code_hash":"%s"}}`, codeID, "denom", 17, codeHash), true, defaultGasForTests, 17)

	require.Empty(t, execErr)
	require.NotEmpty(t, execEvents)

	contractCoinsAfter := keeper.bankKeeper.GetCoins(ctx, addr)
	walletCointsAfter := keeper.bankKeeper.GetCoins(ctx, walletA)

	newContract, err := sdk.AccAddressFromBech32(execEvents[0][0].Value)
	require.NoError(t, err)
	newContractCoins := keeper.bankKeeper.GetCoins(ctx, newContract)

	require.Equal(t, "", contractCoinsAfter.String())
	require.Equal(t, "199983denom", walletCointsAfter.String())
	require.Equal(t, "17denom", newContractCoins.String())
}

func TestContractSendFundsToInitCallbackNotEnough(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	contractCoinsBefore := keeper.bankKeeper.GetCoins(ctx, addr)
	walletCointsBefore := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "", contractCoinsBefore.String())
	require.Equal(t, "200000denom", walletCointsBefore.String())

	_, execEvents, execErr := execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_funds_to_init_callback":{"code_id":%d,"denom":"%s","amount":%d,"code_hash":"%s"}}`, codeID, "denom", 18, codeHash), false, defaultGasForTests, 17)

	require.Empty(t, execEvents)

	require.NotNil(t, execErr.GenericErr)
	require.Equal(t, "insufficient funds: insufficient account funds; 17denom < 18denom", execErr.GenericErr.Msg)

	contractCoinsAfter := keeper.bankKeeper.GetCoins(ctx, addr)
	walletCointsAfter := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "17denom", contractCoinsAfter.String())
	require.Equal(t, "199983denom", walletCointsAfter.String())
}

func TestContractSendFundsToExecCallback(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	contractCoinsBefore := keeper.bankKeeper.GetCoins(ctx, addr)
	contract2CoinsBefore := keeper.bankKeeper.GetCoins(ctx, addr2)
	walletCointsBefore := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "", contractCoinsBefore.String())
	require.Equal(t, "", contract2CoinsBefore.String())
	require.Equal(t, "200000denom", walletCointsBefore.String())

	_, _, execErr := execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 17, codeHash), true, defaultGasForTests, 17)

	require.Empty(t, execErr)

	contractCoinsAfter := keeper.bankKeeper.GetCoins(ctx, addr)
	contract2CoinsAfter := keeper.bankKeeper.GetCoins(ctx, addr2)
	walletCointsAfter := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "", contractCoinsAfter.String())
	require.Equal(t, "17denom", contract2CoinsAfter.String())
	require.Equal(t, "199983denom", walletCointsAfter.String())
}

func TestContractSendFundsToExecCallbackNotEnough(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	contractCoinsBefore := keeper.bankKeeper.GetCoins(ctx, addr)
	contract2CoinsBefore := keeper.bankKeeper.GetCoins(ctx, addr2)
	walletCointsBefore := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "", contractCoinsBefore.String())
	require.Equal(t, "", contract2CoinsBefore.String())
	require.Equal(t, "200000denom", walletCointsBefore.String())

	_, _, execErr := execHelper(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 19, codeHash), false, defaultGasForTests, 17)

	require.NotNil(t, execErr.GenericErr)
	require.Equal(t, "insufficient funds: insufficient account funds; 17denom < 19denom", execErr.GenericErr.Msg)

	contractCoinsAfter := keeper.bankKeeper.GetCoins(ctx, addr)
	contract2CoinsAfter := keeper.bankKeeper.GetCoins(ctx, addr2)
	walletCointsAfter := keeper.bankKeeper.GetCoins(ctx, walletA)

	require.Equal(t, "17denom", contractCoinsAfter.String())
	require.Equal(t, "", contract2CoinsAfter.String())
	require.Equal(t, "199983denom", walletCointsAfter.String())
}

func TestSleep(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, execErr := execHelper(t, keeper, ctx, addr, walletA, `{"sleep":{"ms":3000}}`, false, defaultGasForTests, 0)

	require.Error(t, execErr)
	require.Error(t, execErr.GenericErr)
	require.Equal(t, "execute contract failed: Execution error: Enclave: the contract panicked", execErr.GenericErr.Msg)
}

func TestGasIsChargedForInitCallbackToInit(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d,"code_hash":"%s"}}`, codeID, codeHash), true, defaultGasForTests, 2)
	require.Empty(t, err)
}

func TestGasIsChargedForInitCallbackToExec(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"callback":{"contract_addr":"%s","code_hash":"%s"}}`, addr, codeHash), true, defaultGasForTests, 2)
	require.Empty(t, err)
}

func TestGasIsChargedForExecCallbackToInit(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	// exec callback to init
	_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d,"code_hash":"%s"}}`, codeID, codeHash), true, defaultGasForTests, 0, 2)
	require.Empty(t, err)
}

func TestGasIsChargedForExecCallbackToExec(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	// exec callback to exec
	_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":1,"y":2}}`, addr, codeHash), true, defaultGasForTests, 0, 3)
	require.Empty(t, err)
}

func TestGasIsChargedForExecExternalQuery(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"send_external_query_depth_counter":{"to":"%s","depth":2,"code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests, 0, 3)
	require.Empty(t, err)
}

func TestGasIsChargedForInitExternalQuery(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"send_external_query_depth_counter":{"to":"%s","depth":2,"code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests, 3)
	require.Empty(t, err)
}

func TestGasIsChargedForQueryExternalQuery(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, err := queryHelperImpl(t, keeper, ctx, addr, fmt.Sprintf(`{"send_external_query_depth_counter":{"to":"%s","depth":2,"code_hash":"%s"}}`, addr.String(), codeHash), true, defaultGasForTests, 3)
	require.Empty(t, err)
}

func TestWasmTooHighInitialMemoryRuntimeFail(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/too-high-initial-memory.wasm")
	defer os.RemoveAll(tempDir)

	_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, false, defaultGasForTests)
	require.NotNil(t, err.GenericErr)
	require.Equal(t, "instantiate contract failed: Execution error: Enclave: failed to initialize wasm memory", err.GenericErr.Msg)
}

func TestWasmTooHighInitialMemoryStaticFail(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	defer os.RemoveAll(tempDir)
	require.NoError(t, err)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	walletA := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 1)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/static-too-high-initial-memory.wasm")
	require.NoError(t, err)

	_, err = keeper.Create(ctx, walletA, wasmCode, "", "")
	require.Error(t, err)
	require.Equal(t, "create contract failed: Execution error: Error during static Wasm validation: Wasm contract memory's minimum must not exceed 512 pages.", err.Error())
}

func TestWasmWithFloatingPoints(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract_with_floats.wasm")
	defer os.RemoveAll(tempDir)

	_, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, false, defaultGasForTests)
	require.NotNil(t, err.GenericErr)
	require.Equal(t, "instantiate contract failed: Execution error: Enclave: found floating point operation in module code", err.GenericErr.Msg)
}

func TestCodeHashInvalid(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)
	initMsg := []byte(`AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA{"nop":{}`)

	enc, _ := wasmCtx.Encrypt(initMsg)

	_, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to validate transaction")
}

func TestCodeHashEmpty(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)
	initMsg := []byte(`{"nop":{}`)

	enc, _ := wasmCtx.Encrypt(initMsg)

	_, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to validate transaction")
}

func TestCodeHashNotHex(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)
	initMsg := []byte(`🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉{"nop":{}}`)

	enc, _ := wasmCtx.Encrypt(initMsg)

	_, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to validate transaction")
}

func TestCodeHashTooSmall(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := []byte(codeHash[0:63] + `{"nop":{}`)

	enc, _ := wasmCtx.Encrypt(initMsg)

	_, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to validate transaction")
}

func TestCodeHashTooBig(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := []byte(codeHash + "a" + `{"nop":{}`)

	enc, _ := wasmCtx.Encrypt(initMsg)

	_, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)

	initErr := extractInnerError(t, err, enc[0:32], true)
	require.NotEmpty(t, initErr)
	require.NotNil(t, initErr.ParseErr)
	require.Equal(t, "test_contract::contract::InitMsg", initErr.ParseErr.Target)
	require.Equal(t, "Expected to parse either a `true`, `false`, or a `null`.", initErr.ParseErr.Msg)
}

func TestCodeHashWrong(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := []byte(`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855{"nop":{}`)

	enc, _ := wasmCtx.Encrypt(initMsg)

	_, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to validate transaction")
}

func TestCodeHashInitCallInit(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	t.Run("GoodCodeHash", func(t *testing.T) {
		addr, events, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, defaultGasForTests, 2)

		require.Empty(t, err)
		require.Equal(t,
			[]ContractEvent{
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "a", Value: "a"},
				},
				{
					{Key: "contract_address", Value: events[1][0].Value},
					{Key: "init", Value: "🌈"},
				},
			},
			events,
		)
	})
	t.Run("EmptyCodeHash", func(t *testing.T) {
		_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"","msg":"%s","label":"2"}}`, codeID, `{\"nop\":{}}`), false, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"instantiate contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("TooBigCodeHash", func(t *testing.T) {
		_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%sa","msg":"%s","label":"3"}}`, codeID, codeHash, `{\"nop\":{}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"parsing test_contract::contract::InitMsg: Expected to parse either a `true`, `false`, or a `null`.",
		)
	})
	t.Run("TooSmallCodeHash", func(t *testing.T) {
		_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"4"}}`, codeID, codeHash[0:63], `{\"nop\":{}}`), false, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"instantiate contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("WrongCodeHash", func(t *testing.T) {
		_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s","label":"5"}}`, codeID, `{\"nop\":{}}`), false, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"instantiate contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
}

func TestCodeHashInitCallExec(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests, 1)
	require.Empty(t, err)

	t.Run("GoodCodeHash", func(t *testing.T) {
		addr2, events, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, defaultGasForTests, 2)

		require.Empty(t, err)
		require.Equal(t,
			[]ContractEvent{
				{
					{Key: "contract_address", Value: addr2.String()},
					{Key: "b", Value: "b"},
				},
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "watermelon", Value: "🍉"},
				},
			},
			events,
		)
	})
	t.Run("EmptyCodeHash", func(t *testing.T) {
		_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"c\":{\"x\":1,\"y\":1}}`), false, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"execute contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("TooBigCodeHash", func(t *testing.T) {
		_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"parsing test_contract::contract::HandleMsg: Expected to parse either a `true`, `false`, or a `null`.",
		)
	})
	t.Run("TooSmallCodeHash", func(t *testing.T) {
		_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"execute contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("WrongCodeHash", func(t *testing.T) {
		_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"c\":{\"x\":1,\"y\":1}}`), false, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"execute contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
}

func TestCodeHashInitCallQuery(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	t.Run("GoodCodeHash", func(t *testing.T) {
		addr2, events, err := initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.Empty(t, err)
		require.Equal(t,
			[]ContractEvent{
				{
					{Key: "contract_address", Value: addr2.String()},
					{Key: "c", Value: "2"},
				},
			},
			events,
		)
	})
	t.Run("EmptyCodeHash", func(t *testing.T) {
		_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"query contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("TooBigCodeHash", func(t *testing.T) {
		_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"Got an error from query: ParseErr { target: \"test_contract::contract::QueryMsg\", msg: \"Expected to parse either a `true`, `false`, or a `null`.\", backtrace: None }",
		)
	})
	t.Run("TooSmallCodeHash", func(t *testing.T) {
		_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"query contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("WrongCodeHash", func(t *testing.T) {
		_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"query contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
}

func TestCodeHashExecCallInit(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	t.Run("GoodCodeHash", func(t *testing.T) {
		_, events, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, defaultGasForTests, 0, 2)

		require.Empty(t, err)
		require.Equal(t,
			[]ContractEvent{
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "a", Value: "a"},
				},
				{
					{Key: "contract_address", Value: events[1][0].Value},
					{Key: "init", Value: "🌈"},
				},
			},
			events,
		)
	})
	t.Run("EmptyCodeHash", func(t *testing.T) {
		_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"","msg":"%s","label":"2"}}`, codeID, `{\"nop\":{}}`), false, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"instantiate contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("TooBigCodeHash", func(t *testing.T) {
		_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%sa","msg":"%s","label":"3"}}`, codeID, codeHash, `{\"nop\":{}}`), true, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"parsing test_contract::contract::InitMsg: Expected to parse either a `true`, `false`, or a `null`.",
		)
	})
	t.Run("TooSmallCodeHash", func(t *testing.T) {
		_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"4"}}`, codeID, codeHash[0:63], `{\"nop\":{}}`), false, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"instantiate contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("WrongCodeHash", func(t *testing.T) {
		_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s","label":"5"}}`, codeID, `{\"nop\":{}}`), false, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"instantiate contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
}

func TestLabelCollisionWhenMultipleCallbacksToInitFromSameContract(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, defaultGasForTests, 0, 2)
	require.Empty(t, err)

	_, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), false, defaultGasForTests, 0, 1)
	require.NotEmpty(t, err)
	require.NotNil(t, err.GenericErr)
	require.Equal(t, "contract account already exists: 1", err.GenericErr.Msg)
}

func TestCodeHashExecCallExec(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	t.Run("GoodCodeHash", func(t *testing.T) {
		_, events, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, defaultGasForTests, 0, 2)

		require.Empty(t, err)
		require.Equal(t,
			[]ContractEvent{
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "b", Value: "b"},
				},
				{
					{Key: "contract_address", Value: events[1][0].Value},
					{Key: "watermelon", Value: "🍉"},
				},
			},
			events,
		)
	})
	t.Run("EmptyCodeHash", func(t *testing.T) {
		_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"execute contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("TooBigCodeHash", func(t *testing.T) {
		_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr, codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"parsing test_contract::contract::HandleMsg: Expected to parse either a `true`, `false`, or a `null`.",
		)
	})
	t.Run("TooSmallCodeHash", func(t *testing.T) {
		_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, codeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"execute contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("WrongCodeHash", func(t *testing.T) {
		_, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"execute contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
}

func TestCodeHashExecCallQuery(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	t.Run("GoodCodeHash", func(t *testing.T) {
		_, events, err := execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 0, 2)

		require.Empty(t, err)
		require.Equal(t,
			[]ContractEvent{
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "c", Value: "2"},
				},
			},
			events,
		)
	})
	t.Run("EmptyCodeHash", func(t *testing.T) {
		_, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"query contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("TooBigCodeHash", func(t *testing.T) {
		_, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"Got an error from query: ParseErr { target: \"test_contract::contract::QueryMsg\", msg: \"Expected to parse either a `true`, `false`, or a `null`.\", backtrace: None }",
		)
	})
	t.Run("TooSmallCodeHash", func(t *testing.T) {
		_, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"query contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("WrongCodeHash", func(t *testing.T) {
		_, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 0, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"query contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
}

func TestCodeHashQueryCallQuery(t *testing.T) {
	ctx, keeper, tempDir, codeID, codeHash, walletA, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, err)

	t.Run("GoodCodeHash", func(t *testing.T) {
		output, err := queryHelperImpl(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.Empty(t, err)
		require.Equal(t, "2", output)
	})
	t.Run("EmptyCodeHash", func(t *testing.T) {
		_, err := queryHelperImpl(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"query contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("TooBigCodeHash", func(t *testing.T) {
		_, err := queryHelperImpl(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"Got an error from query: ParseErr { target: \"test_contract::contract::QueryMsg\", msg: \"Expected to parse either a `true`, `false`, or a `null`.\", backtrace: None }",
		)
	})
	t.Run("TooSmallCodeHash", func(t *testing.T) {
		_, err := queryHelperImpl(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"query contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
	t.Run("WrongCodeHash", func(t *testing.T) {
		_, err := queryHelperImpl(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, defaultGasForTests, 2)

		require.NotEmpty(t, err)
		require.Contains(t,
			err.Error(),
			"query contract failed: Execution error: Enclave: failed to validate transaction",
		)
	})
}
