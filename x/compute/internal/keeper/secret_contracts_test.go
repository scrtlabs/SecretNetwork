package keeper

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"regexp"
	"testing"
	"time"

	stypes "github.com/cosmos/cosmos-sdk/store/types"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"

	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cosmwasm "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	v010cosmwasm "github.com/enigmampc/SecretNetwork/go-cosmwasm/types/v010"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

type ContractEvent []v010cosmwasm.LogAttribute

type TestContract struct {
	Version string
	Wasm    string
}

var testContracts = []TestContract{
	{
		Version: "v0.10",
		Wasm:    "./testdata/test-contract/contract.wasm",
	}, {
		Version: "v1",
		Wasm:    "./testdata/v1-sanity-contract/contract.wasm",
	},
}

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

func setupTest(t *testing.T, wasmPath string) (sdk.Context, Keeper, uint64, string, sdk.AccAddress, crypto.PrivKey, sdk.AccAddress, crypto.PrivKey) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	walletA, privKeyA := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))
	walletB, privKeyB := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, topUp)

	wasmCode, err := ioutil.ReadFile(wasmPath)
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, codeID).CodeHash)

	return ctx, keeper, codeID, codeHash, walletA, privKeyA, walletB, privKeyB
}

// getDecryptedWasmEvents gets all "wasm" events and decrypt what's necessary
// Returns all "wasm" events, including from contract callbacks
func getDecryptedWasmEvents(t *testing.T, ctx sdk.Context, nonce []byte) []ContractEvent {
	events := ctx.EventManager().Events()
	var res []ContractEvent
	for _, e := range events {
		if e.Type == "wasm" {
			newEvent := []v010cosmwasm.LogAttribute{}
			for _, oldLog := range e.Attributes {
				newLog := v010cosmwasm.LogAttribute{
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

// tryDecryptWasmEvents gets all "wasm" events and try to decrypt what it can.
// Returns all "wasm" events, including from contract callbacks.
// The difference between this and getDecryptedWasmEvents is that it is aware of plaintext logs.
func tryDecryptWasmEvents(ctx sdk.Context, nonce []byte) []ContractEvent {
	events := ctx.EventManager().Events()
	var res []ContractEvent
	for _, e := range events {
		if e.Type == "wasm" {
			newEvent := []v010cosmwasm.LogAttribute{}
			for _, oldLog := range e.Attributes {
				newLog := v010cosmwasm.LogAttribute{
					Key:   string(oldLog.Key),
					Value: string(oldLog.Value),
				}
				newEvent = append(newEvent, newLog)

				if newLog.Key != "contract_address" {
					// key
					keyCipherBz, err := base64.StdEncoding.DecodeString(newLog.Key)
					if err != nil {
						continue
					}
					keyPlainBz, err := wasmCtx.Decrypt(keyCipherBz, nonce)
					if err != nil {
						continue
					}
					newEvent[len(newEvent)-1].Key = string(keyPlainBz)

					// value
					valueCipherBz, err := base64.StdEncoding.DecodeString(newLog.Value)
					if err != nil {
						continue
					}
					valuePlainBz, err := wasmCtx.Decrypt(valueCipherBz, nonce)
					if err != nil {
						continue
					}
					newEvent[len(newEvent)-1].Value = string(valuePlainBz)
				}
			}
			res = append(res, newEvent)
		}
	}
	return res
}

// getDecryptedData decrytes the output of the first function to be called
// Only returns the data, logs and messages from the first function call
func getDecryptedData(t *testing.T, data []byte, nonce []byte) []byte {
	if len(data) == 0 {
		return data
	}

	dataPlaintextBase64, err := wasmCtx.Decrypt(data, nonce)
	require.NoError(t, err)

	dataPlaintext, err := base64.StdEncoding.DecodeString(string(dataPlaintextBase64))
	require.NoError(t, err)

	return dataPlaintext
}

var contractErrorRegex = regexp.MustCompile(`.*encrypted: (.+): (?:instantiate|execute|query|reply to) contract failed`)

func extractInnerError(t *testing.T, err error, nonce []byte, isEncrypted bool, isV1Contract bool) cosmwasm.StdError {
	match := contractErrorRegex.FindAllStringSubmatch(err.Error(), -1)
	if match == nil {
		require.True(t, !isEncrypted, fmt.Sprintf("Error message should be plaintext but was: %v", err))
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
	if !isV1Contract {
		err = json.Unmarshal(errorPlainBz, &innerErr)
		require.NoError(t, err)
	} else {
		innerErr = cosmwasm.StdError{GenericErr: &cosmwasm.GenericErr{Msg: string(errorPlainBz)}}
	}

	return innerErr
}

const defaultGasForTests uint64 = 100_000

// wrap the defualt gas meter with a counter of wasm calls
// in order to verify that every wasm call consumes gas
type WasmCounterGasMeter struct {
	wasmCounter uint64
	gasMeter    sdk.GasMeter
}

func (wasmGasMeter *WasmCounterGasMeter) RefundGas(amount stypes.Gas, descriptor string) {}

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
	if (descriptor == "wasm contract" || descriptor == "contract sub-query") && amount > 0 {
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

func (wasmGasMeter *WasmCounterGasMeter) String() string {
	return fmt.Sprintf("WasmCounterGasMeter: %+v %+v", wasmGasMeter.wasmCounter, wasmGasMeter.gasMeter)
}

func (wasmGasMeter *WasmCounterGasMeter) GetWasmCounter() uint64 {
	return wasmGasMeter.wasmCounter
}

var _ sdk.GasMeter = (*WasmCounterGasMeter)(nil) // check interface

func queryHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, input string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64,
) (string, cosmwasm.StdError) {
	return queryHelperImpl(t, keeper, ctx, contractAddr, input, isErrorEncrypted, isV1Contract, gas, -1)
}

func queryHelperImpl(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddr sdk.AccAddress, input string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, wasmCallCount int64,
) (string, cosmwasm.StdError) {
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

	resultCipherBz, err := keeper.QuerySmart(ctx, contractAddr, queryBz, false)

	if wasmCallCount < 0 {
		// default, just check that at least 1 call happend
		require.NotZero(t, gasMeter.GetWasmCounter(), err)
	} else {
		require.Equal(t, uint64(wasmCallCount), gasMeter.GetWasmCounter(), err)
	}

	if err != nil {
		return "", extractInnerError(t, err, nonce, isErrorEncrypted, isV1Contract)
	}

	resultPlainBz, err := wasmCtx.Decrypt(resultCipherBz, nonce)
	require.NoError(t, err)

	resultBz, err := base64.StdEncoding.DecodeString(string(resultPlainBz))
	require.NoError(t, err)

	return string(resultBz), cosmwasm.StdError{}
}

func execHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddress sdk.AccAddress, txSender sdk.AccAddress, senderPrivKey crypto.PrivKey, execMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, coin int64,
) ([]byte, []ContractEvent, uint64, cosmwasm.StdError) {
	return execHelperImpl(t, keeper, ctx, contractAddress, txSender, senderPrivKey, execMsg, isErrorEncrypted, isV1Contract, gas, coin, -1)
}

func execHelperImpl(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddress sdk.AccAddress, txSender sdk.AccAddress, senderPrivKey crypto.PrivKey, execMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, coin int64, wasmCallCount int64,
) ([]byte, []ContractEvent, uint64, cosmwasm.StdError) {
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

	ctx = PrepareExecSignedTx(t, keeper, ctx, txSender, senderPrivKey, execMsgBz, contractAddress, sdk.NewCoins(sdk.NewInt64Coin("denom", coin)))

	gasBefore := ctx.GasMeter().GasConsumed()
	execResult, err := keeper.Execute(ctx, contractAddress, txSender, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", coin)), nil)
	gasAfter := ctx.GasMeter().GasConsumed()
	gasUsed := gasAfter - gasBefore

	if wasmCallCount < 0 {
		// default, just check that at least 1 call happend
		require.NotZero(t, gasMeter.GetWasmCounter(), err)
	} else {
		require.Equal(t, uint64(wasmCallCount), gasMeter.GetWasmCounter(), err)
	}

	if err != nil {
		return nil, nil, 0, extractInnerError(t, err, nonce, isErrorEncrypted, isV1Contract)
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := tryDecryptWasmEvents(ctx, nonce)

	// TODO check if we can extract the messages from ctx

	// Data is the output of only the first call
	data := getDecryptedData(t, execResult.Data, nonce)

	return data, wasmEvents, gasUsed, cosmwasm.StdError{}
}

func initHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	codeID uint64, creator sdk.AccAddress, creatorPrivKey crypto.PrivKey, initMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64,
) (sdk.AccAddress, []ContractEvent, cosmwasm.StdError) {
	return initHelperImpl(t, keeper, ctx, codeID, creator, creatorPrivKey, initMsg, isErrorEncrypted, isV1Contract, gas, -1, 0)
}

func initHelperImpl(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	codeID uint64, creator sdk.AccAddress, creatorPrivKey crypto.PrivKey, initMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, wasmCallCount int64, coin int64,
) (sdk.AccAddress, []ContractEvent, cosmwasm.StdError) {
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

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, creatorPrivKey, initMsgBz, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", coin)))
	// make the label a random base64 string, because why not?
	contractAddress, _, err := keeper.Instantiate(ctx, codeID, creator /* nil,*/, initMsgBz, base64.RawURLEncoding.EncodeToString(nonce), sdk.NewCoins(sdk.NewInt64Coin("denom", coin)), nil)

	if wasmCallCount < 0 {
		// default, just check that at least 1 call happend
		require.NotZero(t, gasMeter.GetWasmCounter(), err)
	} else {
		require.Equal(t, uint64(wasmCallCount), gasMeter.GetWasmCounter(), err)
	}

	if err != nil {
		return nil, nil, extractInnerError(t, err, nonce, isErrorEncrypted, isV1Contract)
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := tryDecryptWasmEvents(ctx, nonce)

	// TODO check if we can extract the messages from ctx

	return contractAddress, wasmEvents, cosmwasm.StdError{}
}

func TestCallbackSanity(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// init
			contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
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

			data, execEvents, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":2,"y":3}}`, contractAddress.String(), codeHash), true, false, defaultGasForTests, 0)
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
		})
	}
}

func TestSanity(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, "./testdata/erc20.wasm")

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, initMsg, true, false, defaultGasForTests)
	require.Empty(t, err)
	// require.Empty(t, initEvents)

	// check state after init
	qRes, qErr := queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"108"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"53"}`, qRes)

	// transfer 10 from A to B
	data, wasmEvents, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA,
		fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String()), true, false, defaultGasForTests, 0)

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
	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"98"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"63"}`, qRes)
}

func TestInitLogs(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
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
		})
	}
}

func TestEmptyLogKeyValue(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"empty_log_key_value":{}}`, true, false, defaultGasForTests, 0)

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
		})
	}
}

func TestEmptyData(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"empty_data":{}}`, true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Empty(t, data)
		})
	}
}

func TestNoData(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"no_data":{}}`, true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Empty(t, data)
		})
	}
}

func TestExecuteIllegalInputError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `bad input`, true, false, defaultGasForTests, 0)

			require.NotNil(t, execErr.ParseErr)
		})
	}
}

func TestInitIllegalInputError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `bad input`, true, false, defaultGasForTests)

			require.NotNil(t, initErr.ParseErr)
		})
	}
}

func TestCallbackFromInitAndCallbackEvents(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// init first contract so we'd have someone to callback
			firstContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
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
			contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback":{"contract_addr":"%s", "code_hash": "%s"}}`, firstContractAddress.String(), codeHash), true, false, defaultGasForTests)
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
		})
	}
}

func TestQueryInputParamError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, "./testdata/erc20.wasm")

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, initMsg, true, false, defaultGasForTests)
	require.Empty(t, err)
	// require.Empty(t, initEvents)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"address":"blabla"}}`, true, false, defaultGasForTests)

	require.NotNil(t, qErr.GenericErr)
	require.Equal(t, "canonicalize_address errored: invalid length", qErr.GenericErr.Msg)
}

func TestUnicodeData(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestInitContractError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			t.Run("generic_err", func(t *testing.T) {
				_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"generic_err"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.GenericErr)
				require.Equal(t, "la la 🤯", err.GenericErr.Msg)
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.InvalidBase64)
				require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.InvalidUtf8)
				require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
			})
			t.Run("not_found", func(t *testing.T) {
				_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"not_found"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.NotFound)
				require.Equal(t, "za za 🤯", err.NotFound.Kind)
			})
			t.Run("parse_err", func(t *testing.T) {
				_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"parse_err"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.ParseErr)
				require.Equal(t, "na na 🤯", err.ParseErr.Target)
				require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
			})
			t.Run("serialize_err", func(t *testing.T) {
				_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"serialize_err"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.SerializeErr)
				require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
				require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
			})
			t.Run("unauthorized", func(t *testing.T) {
				_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"unauthorized"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.Unauthorized)
			})
			t.Run("underflow", func(t *testing.T) {
				_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"underflow"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.Underflow)
				require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
				require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
			})
		})
	}
}

func TestExecContractError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			t.Run("generic_err", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"generic_err"}}`, true, false, defaultGasForTests, 0)

				require.NotNil(t, err.GenericErr)
				require.Equal(t, "la la 🤯", err.GenericErr.Msg)
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, false, defaultGasForTests, 0)

				require.NotNil(t, err.InvalidBase64)
				require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, false, defaultGasForTests, 0)

				require.NotNil(t, err.InvalidUtf8)
				require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
			})
			t.Run("not_found", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"not_found"}}`, true, false, defaultGasForTests, 0)

				require.NotNil(t, err.NotFound)
				require.Equal(t, "za za 🤯", err.NotFound.Kind)
			})
			t.Run("parse_err", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"parse_err"}}`, true, false, defaultGasForTests, 0)

				require.NotNil(t, err.ParseErr)
				require.Equal(t, "na na 🤯", err.ParseErr.Target)
				require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
			})
			t.Run("serialize_err", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"serialize_err"}}`, true, false, defaultGasForTests, 0)

				require.NotNil(t, err.SerializeErr)
				require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
				require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
			})
			t.Run("unauthorized", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"unauthorized"}}`, true, false, defaultGasForTests, 0)

				require.NotNil(t, err.Unauthorized)
			})
			t.Run("underflow", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"underflow"}}`, true, false, defaultGasForTests, 0)

				require.NotNil(t, err.Underflow)
				require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
				require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
			})
		})
	}
}

func TestQueryContractError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			t.Run("generic_err", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"generic_err"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.GenericErr)
				require.Equal(t, "la la 🤯", err.GenericErr.Msg)
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"invalid_base64"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.InvalidBase64)
				require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.InvalidUtf8)
				require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
			})
			t.Run("not_found", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"not_found"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.NotFound)
				require.Equal(t, "za za 🤯", err.NotFound.Kind)
			})
			t.Run("parse_err", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"parse_err"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.ParseErr)
				require.Equal(t, "na na 🤯", err.ParseErr.Target)
				require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
			})
			t.Run("serialize_err", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"serialize_err"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.SerializeErr)
				require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
				require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
			})
			t.Run("unauthorized", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"unauthorized"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.Unauthorized)
			})
			t.Run("underflow", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"underflow"}}`, true, false, defaultGasForTests)

				require.NotNil(t, err.Underflow)
				require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
				require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
			})
		})
	}
}

func TestInitParamError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			codeHash := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
			msg := fmt.Sprintf(`{"callback":{"contract_addr":"notanaddress", "code_hash":"%s"}}`, codeHash)

			_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, msg, false, false, defaultGasForTests)

			require.Contains(t, initErr.Error(), "invalid address")
		})
	}
}

func TestCallbackExecuteParamError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			msg := fmt.Sprintf(`{"a":{"code_hash":"%s","contract_addr":"notanaddress","x":2,"y":3}}`, codeHash)

			_, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, false, false, defaultGasForTests, 0)

			require.Contains(t, err.Error(), "invalid address")
		})
	}
}

func TestQueryInputStructureError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, "./testdata/erc20.wasm")

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, initMsg, true, false, defaultGasForTests)
	require.Empty(t, err)
	// require.Empty(t, initEvents)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"invalidkey":"invalidval"}}`, true, false, defaultGasForTests)

	require.NotNil(t, qErr.ParseErr)
	require.Contains(t, qErr.ParseErr.Msg, "missing field `address`")
}

func TestInitNotEncryptedInputError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKey, _, _ := setupTest(t, tc.Wasm)

			//ctx = sdk.NewContext(
			//	ctx.MultiStore(),
			//	ctx.BlockHeader(),
			//	ctx.IsCheckTx(),
			//	log.NewNopLogger(),
			//).WithGasMeter(sdk.NewGasMeter(defaultGas))

			initMsg := []byte(`{"nop":{}`)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privKey, initMsg, codeID, nil)

			// init
			_, _, err := keeper.Instantiate(ctx, codeID, walletA /* nil, */, initMsg, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)

			require.Contains(t, err.Error(), "failed to decrypt data")
		})
	}
}

func TestExecuteNotEncryptedInputError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			//ctx = sdk.NewContext(
			//	ctx.MultiStore(),
			//	ctx.BlockHeader(),
			//	ctx.IsCheckTx(),
			//	log.NewNopLogger(),
			//).WithGasMeter(sdk.NewGasMeter(defaultGas))

			execMsg := []byte(`{"empty_log_key_value":{}}`)

			ctx = PrepareExecSignedTx(t, keeper, ctx, walletA, privKeyA, execMsg, contractAddress, nil)

			_, err := keeper.Execute(ctx, contractAddress, walletA, execMsg, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)

			require.Contains(t, err.Error(), "failed to decrypt data")
		})
	}
}

func TestQueryNotEncryptedInputError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, err := keeper.QuerySmart(ctx, contractAddress, []byte(`{"owner":{}}`), false)
			require.Error(t, err)

			require.Contains(t, err.Error(), "failed to decrypt data")
		})
	}
}

func TestInitNoLogs(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// init
			_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"no_logs":{}}`, true, false, defaultGasForTests)

			require.Empty(t, initErr)
			////require.Empty(t, initEvents)
		})
	}
}

func TestExecNoLogs(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// init
			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"no_logs":{}}`, true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			// require.Empty(t, execEvents)
		})
	}
}

func TestExecCallbackToInit(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// init first contract
			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// init second contract and callback to the first contract
			execData, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, false, defaultGasForTests, 0)
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
				v010cosmwasm.LogAttribute{Key: "init", Value: "🌈"},
				execEvents[1][1],
			)
			require.Equal(t, "contract_address", execEvents[1][0].Key)

			secondContractAddressBech32 := execEvents[1][0].Value
			secondContractAddress, err := sdk.AccAddressFromBech32(secondContractAddressBech32)
			require.NoError(t, err)

			data, _, _, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			// require.Empty(t, execEvents)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestInitCallbackToInit(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, false, defaultGasForTests)
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
				v010cosmwasm.LogAttribute{Key: "init", Value: "🌈"},
				initEvents[1][1],
			)
			require.Equal(t, "contract_address", initEvents[1][0].Key)

			secondContractAddressBech32 := initEvents[1][0].Value
			secondContractAddress, err := sdk.AccAddressFromBech32(secondContractAddressBech32)
			require.NoError(t, err)

			data, _, _, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			// require.Empty(t, execEvents)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestInitCallbackContractError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			secondContractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s", "code_hash":"%s"}}`, contractAddress, codeHash), true, false, defaultGasForTests)

			require.NotNil(t, initErr.GenericErr)
			require.Equal(t, "la la 🤯", initErr.GenericErr.Msg)
			require.Empty(t, secondContractAddress)
			// require.Empty(t, initEvents)
		})
	}
}

func TestExecCallbackContractError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// init
			contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s","code_hash":"%s"}}`, contractAddress, codeHash), true, false, defaultGasForTests, 0)

			require.NotNil(t, execErr.GenericErr)
			require.Equal(t, "la la 🤯", execErr.GenericErr.Msg)
			// require.Empty(t, execEvents)
			require.Empty(t, data)
		})
	}
}

func TestExecCallbackBadParam(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// init
			contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, false, defaultGasForTests, 0)

			require.NotNil(t, execErr.ParseErr)
			require.Equal(t, "test_contract::contract::HandleMsg", execErr.ParseErr.Target)
			require.Contains(t, execErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
			// require.Empty(t, execEvents)
			require.Empty(t, data)
		})
	}
}

func TestInitCallbackBadParam(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// init first
			contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			secondContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, false, defaultGasForTests)
			require.Empty(t, secondContractAddress)
			// require.Empty(t, initEvents)

			require.NotNil(t, initErr.ParseErr)
			require.Equal(t, "test_contract::contract::InitMsg", initErr.ParseErr.Target)
			require.Contains(t, initErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
		})
	}
}

func TestState(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// init
			contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, false, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Empty(t, data)

			_, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, false, defaultGasForTests, 0)
			require.Empty(t, execErr)

			data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, false, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(data))

			_, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"remove_state":{"key":"banana"}}`, true, false, defaultGasForTests, 0)
			require.Empty(t, execErr)

			data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, false, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Empty(t, data)
		})
	}
}

func TestCanonicalizeAddressErrors(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			// this function should handle errors internally and return gracefully
			data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"test_canonicalize_address_errors":{}}`, true, false, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🤟", string(data))
		})
	}
}

func TestInitPanic(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			_, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"panic":{}}`, false, false, defaultGasForTests)

			require.NotNil(t, initErr.GenericErr)
			require.Contains(t, initErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestExecPanic(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"panic":{}}`, false, false, defaultGasForTests, 0)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestQueryPanic(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, queryErr := queryHelper(t, keeper, ctx, addr, `{"panic":{}}`, false, false, defaultGasForTests)
			require.NotNil(t, queryErr.GenericErr)
			require.Contains(t, queryErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseMemoryLimit(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":13631488}}`, false, false, defaultGasForTests, 0)

			// this should fail with memory error because 13MiB is more than the allowed 12MiB

			require.Empty(t, data)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseGasLimit(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// ensure we get an out of gas panic
			defer func() {
				r := recover()
				require.NotNil(t, r)
				_, ok := r.(sdk.ErrorOutOfGas)
				require.True(t, ok, "%+v", r)
			}()

			_, _, _, _ = execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, false, defaultGasForTests, 0)

			// this should fail with out of gas because 1GiB will ask for
			// 134,217,728 gas units (8192 per page times 16,384 pages)
			// the default gas limit in ctx is 200,000 which translates into
			// 20,000,000 WASM gas units, so before the memory_grow opcode is reached
			// the gas metering sees a request that'll cost 134mn and the limit
			// is 20mn, so it throws an out of gas exception

			require.True(t, false)
		})
	}
}

func TestAllocateOnHeapMoreThanSGXHasFailBecauseMemoryLimit(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, false, 9_000_000, 0)

			// this should fail with memory error because 1GiB is more
			// than the allowed 12MiB, gas is 9mn so WASM gas is 900mn
			// which is bigger than the 134mn from the previous test

			require.Empty(t, data)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestPassNullPointerToImports(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
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
					_, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"pass_null_pointer_to_imports_should_throw":{"pass_type":"%s"}}`, passType), false, false, defaultGasForTests, 0)

					require.NotNil(t, execErr.GenericErr)
					require.Contains(t, execErr.GenericErr.Msg, "failed to read memory")
				})
			}
		})
	}
}

func TestExternalQueryWorks(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests, 0)

			require.Empty(t, execErr)
			require.Equal(t, []byte{3}, data)
		})
	}
}

func TestExternalQueryCalleePanic(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_panic":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests, 0)

			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestExternalQueryCalleeStdError(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_error":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests, 0)

			require.NotNil(t, err.GenericErr)
			require.Equal(t, "la la 🤯", err.GenericErr.Msg)
		})
	}
}

func TestExternalQueryCalleeDoesntExist(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"send_external_query_error":{"to":"secret13l72vhjngmg55ykajxdnlalktwglyqjqv9pkq4","code_hash":"bla bla"}}`, true, false, defaultGasForTests, 0)

			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "not found")
		})
	}
}

func TestExternalQueryBadSenderABI(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_bad_abi":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests, 0)

			require.NotNil(t, err.ParseErr)
			require.Equal(t, "test_contract::contract::QueryMsg", err.ParseErr.Target)
			require.Equal(t, "Invalid type", err.ParseErr.Msg)
		})
	}
}

func TestExternalQueryBadReceiverABI(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_bad_abi_receiver":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests, 0)

			require.NotNil(t, err.ParseErr)
			require.Equal(t, "alloc::string::String", err.ParseErr.Target)
			require.Equal(t, "Invalid type", err.ParseErr.Msg)
		})
	}
}

func TestMsgSenderInCallback(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			_, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"callback_to_log_msg_sender":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t, []ContractEvent{
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "hi", Value: "hey"},
				},
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "msg.sender", Value: addr.String()},
				},
			}, events)
		})
	}
}

func TestInfiniteQueryLoopKilledGracefullyByOOM(t *testing.T) {
	t.SkipNow() // We no longer expect to hit OOM trivially
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			data, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"send_external_query_infinite_loop":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests)

			require.Empty(t, data)
			require.NotNil(t, err.GenericErr)
			require.Equal(t, err.GenericErr.Msg, "query contract failed: Execution error: Enclave: enclave ran out of heap memory")
		})
	}
}

func TestQueryRecursionLimitEnforcedInQueries(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			data, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), codeHash), true, false, defaultGasForTests)

			require.NotEmpty(t, data)
			require.Equal(t, data, "\"Recursion limit was correctly enforced\"")

			require.Nil(t, err.GenericErr)
		})
	}
}

func TestQueryRecursionLimitEnforcedInHandles(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			data, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), codeHash), true, false, defaultGasForTests, 0)

			require.NotEmpty(t, data)
			require.Equal(t, string(data), "\"Recursion limit was correctly enforced\"")

			require.Nil(t, err.GenericErr)
		})
	}
}

func TestQueryRecursionLimitEnforcedInInits(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			// Initialize a contract that we will be querying
			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			// Initialize the contract that will be running the test
			addr, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), codeHash), true, false, defaultGasForTests)
			require.Empty(t, err)

			require.Nil(t, err.GenericErr)

			require.Equal(t, []ContractEvent{
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "message", Value: "Recursion limit was correctly enforced"},
				},
			}, events)
		})
	}
}

func TestWriteToStorageDuringQuery(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, queryErr := queryHelper(t, keeper, ctx, addr, `{"write_to_storage": {}}`, false, false, defaultGasForTests)
			require.NotNil(t, queryErr.GenericErr)
			require.Contains(t, queryErr.GenericErr.Msg, "contract tried to write to storage during a query")
		})
	}
}

func TestRemoveFromStorageDuringQuery(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, queryErr := queryHelper(t, keeper, ctx, addr, `{"remove_from_storage": {}}`, false, false, defaultGasForTests)
			require.NotNil(t, queryErr.GenericErr)
			require.Contains(t, queryErr.GenericErr.Msg, "contract tried to write to storage during a query")
		})
	}
}

func TestDepositToContract(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCointsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCointsBefore.String())

			data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, false, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCointsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCointsAfter.String())

			require.Equal(t, `[{"denom":"denom","amount":"17"}]`, string(data))
		})
	}
}

func TestContractSendFunds(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, false, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCointsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsBefore.String())
			require.Equal(t, "199983denom", walletCointsBefore.String())

			_, _, _, execErr = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, addr.String(), walletA.String(), "denom", 17), false, false, defaultGasForTests, 0)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCointsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "200000denom", walletCointsAfter.String())

			require.Empty(t, execErr)
		})
	}
}

func TestContractTryToSendFundsFromSomeoneElse(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, false, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsBefore.String())
			require.Equal(t, "199983denom", walletCoinsBefore.String())

			_, _, _, execErr = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, walletA.String(), addr.String(), "denom", 17), false, false, defaultGasForTests, 0)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "contract doesn't have permission")
		})
	}
}

func TestContractSendFundsToInitCallback(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCointsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCointsBefore.String())

			_, execEvents, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_init_callback":{"code_id":%d,"denom":"%s","amount":%d,"code_hash":"%s"}}`, codeID, "denom", 17, codeHash), true, false, defaultGasForTests, 17)

			require.Empty(t, execErr)
			require.NotEmpty(t, execEvents)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCointsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			newContract, err := sdk.AccAddressFromBech32(execEvents[1][0].Value)
			require.NoError(t, err)
			newContractCoins := keeper.bankKeeper.GetAllBalances(ctx, newContract)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCointsAfter.String())
			require.Equal(t, "17denom", newContractCoins.String())
		})
	}
}

func TestContractSendFundsToInitCallbackNotEnough(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCointsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCointsBefore.String())

			_, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_init_callback":{"code_id":%d,"denom":"%s","amount":%d,"code_hash":"%s"}}`, codeID, "denom", 18, codeHash), false, false, defaultGasForTests, 17)

			// require.Empty(t, execEvents)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "insufficient funds")

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCointsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCointsAfter.String())
		})
	}
}

func TestContractSendFundsToExecCallback(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCointsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCointsBefore.String())

			_, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 17, codeHash), true, false, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCointsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "17denom", contract2CoinsAfter.String())
			require.Equal(t, "199983denom", walletCointsAfter.String())
		})
	}
}

func TestContractSendFundsToExecCallbackNotEnough(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCointsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCointsBefore.String())

			_, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 19, codeHash), false, false, defaultGasForTests, 17)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "insufficient funds")

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCointsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "", contract2CoinsAfter.String())
			require.Equal(t, "199983denom", walletCointsAfter.String())
		})
	}
}

func TestSleep(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"sleep":{"ms":3000}}`, false, false, defaultGasForTests, 0)

			require.Error(t, execErr)
			require.Error(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestGasIsChargedForInitCallbackToInit(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d,"code_hash":"%s"}}`, codeID, codeHash), true, false, defaultGasForTests, 2, 0)
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForInitCallbackToExec(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback":{"contract_addr":"%s","code_hash":"%s"}}`, addr, codeHash), true, false, defaultGasForTests, 2, 0)
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForExecCallbackToInit(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// exec callback to init
			_, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d,"code_hash":"%s"}}`, codeID, codeHash), true, false, defaultGasForTests, 0, 2)
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForExecCallbackToExec(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// exec callback to exec
			_, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":1,"y":2}}`, addr, codeHash), true, false, defaultGasForTests, 0, 3)
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForExecExternalQuery(t *testing.T) {
	t.SkipNow() // as of v0.10 CowmWasm are overriding the default gas meter

	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_depth_counter":{"to":"%s","depth":2,"code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests, 0, 3)
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForInitExternalQuery(t *testing.T) {
	t.SkipNow() // as of v0.10 CowmWasm are overriding the default gas meter

	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_depth_counter":{"to":"%s","depth":2,"code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests, 3, 0)
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForQueryExternalQuery(t *testing.T) {
	t.SkipNow() // as of v0.10 CowmWasm are overriding the default gas meter

	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			_, err := queryHelperImpl(t, keeper, ctx, addr, fmt.Sprintf(`{"send_external_query_depth_counter":{"to":"%s","depth":2,"code_hash":"%s"}}`, addr.String(), codeHash), true, false, defaultGasForTests, 3)
			require.Empty(t, err)
		})
	}
}

func TestWasmTooHighInitialMemoryRuntimeFail(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/test-contract/too-high-initial-memory.wasm")

	_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, false, defaultGasForTests)
	require.NotNil(t, err.GenericErr)
	require.Contains(t, err.GenericErr.Msg, "failed to initialize wasm memory")
}

func TestWasmTooHighInitialMemoryStaticFail(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	walletA, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 1)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/static-too-high-initial-memory.wasm")
	require.NoError(t, err)

	_, err = keeper.Create(ctx, walletA, wasmCode, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Error during static Wasm validation: Wasm contract memory's minimum must not exceed 512 pages")
}

func TestWasmWithFloatingPoints(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/test-contract/contract_with_floats.wasm")

			_, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, false, defaultGasForTests)
			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "found floating point operation in module code")
		})
	}
}

func TestCodeHashInvalid(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, tc.Wasm)
			initMsg := []byte(`AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA /* nil, */, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashEmpty(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, tc.Wasm)
			initMsg := []byte(`{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA /* nil, */, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashNotHex(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, tc.Wasm)
			initMsg := []byte(`🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉{"nop":{}}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA /* nil, */, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashTooSmall(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privWalletA, _, _ := setupTest(t, tc.Wasm)

			initMsg := []byte(codeHash[0:63] + `{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA /* nil, */, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashTooBig(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privWalletA, _, _ := setupTest(t, tc.Wasm)

			initMsg := []byte(codeHash + "a" + `{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA /* nil, */, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)

			initErr := extractInnerError(t, err, enc[0:32], true, false)
			require.NotEmpty(t, initErr)
			require.NotNil(t, initErr.ParseErr)
			require.Equal(t, "test_contract::contract::InitMsg", initErr.ParseErr.Target)
			require.Equal(t, "Expected to parse either a `true`, `false`, or a `null`.", initErr.ParseErr.Msg)
		})
	}
}

func TestCodeHashWrong(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, tc.Wasm)

			initMsg := []byte(`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA /* nil, */, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashInitCallInit(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			t.Run("GoodCodeHash", func(t *testing.T) {
				addr, events, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, false, defaultGasForTests, 2, 0)

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
				_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"","msg":"%s","label":"2"}}`, codeID, `{\"nop\":{}}`), false, false, defaultGasForTests, 2, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%sa","msg":"%s","label":"3"}}`, codeID, codeHash, `{\"nop\":{}}`), true, false, defaultGasForTests, 2, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"parsing test_contract::contract::InitMsg: Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"4"}}`, codeID, codeHash[0:63], `{\"nop\":{}}`), false, false, defaultGasForTests, 2, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s","label":"5"}}`, codeID, `{\"nop\":{}}`), false, false, defaultGasForTests, 2, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestCodeHashInitCallExec(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests, 1, 0)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				addr2, events, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, false, defaultGasForTests, 2, 0)

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
				_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"c\":{\"x\":1,\"y\":1}}`), false, false, defaultGasForTests, 2, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, false, defaultGasForTests, 2, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"parsing test_contract::contract::HandleMsg: Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, false, defaultGasForTests, 2, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"c\":{\"x\":1,\"y\":1}}`), false, false, defaultGasForTests, 2, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestCodeHashInitCallQuery(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				addr2, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

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
				_, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Got an error from query: ParseErr { target: \"test_contract::contract::QueryMsg\", msg: \"Expected to parse either a `true`, `false`, or a `null`.\", backtrace: None }",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestCodeHashExecCallInit(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, events, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, false, defaultGasForTests, 0, 2)

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
				_, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"","msg":"%s","label":"2"}}`, codeID, `{\"nop\":{}}`), false, false, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%sa","msg":"%s","label":"3"}}`, codeID, codeHash, `{\"nop\":{}}`), true, false, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"parsing test_contract::contract::InitMsg: Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"4"}}`, codeID, codeHash[0:63], `{\"nop\":{}}`), false, false, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s","label":"5"}}`, codeID, `{\"nop\":{}}`), false, false, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestLabelCollisionWhenMultipleCallbacksToInitFromSameContract(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, false, defaultGasForTests, 0, 2)
			require.Empty(t, err)

			_, _, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), false, false, defaultGasForTests, 0, 1)
			require.NotEmpty(t, err)
			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "contract account already exists")
		})
	}
}

func TestCodeHashExecCallExec(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, false, defaultGasForTests, 0)

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
				_, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, false, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr, codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, false, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"parsing test_contract::contract::HandleMsg: Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, codeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, false, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, false, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

// todo: enable after the upgrade to sdk 0.45x
//func TestGasUsageForStoreKey(t *testing.T) {
//	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)
//
//	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
//	require.Empty(t, err)
//
//	t.Run("StoreDifferentKeySizes", func(t *testing.T) {
//		_, _, gasUsedLong, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"store_really_long_key":{}}`, true, false, defaultGasForTests, 0)
//		require.Empty(t, err)
//
//		_, _, gasUsedShort, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"store_really_short_key":{}}`, true, false, defaultGasForTests, 0)
//		require.Empty(t, err)
//
//		_, _, gasUsedLongValue, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"store_really_long_value":{}}`, true, false, defaultGasForTests, 0)
//		require.Empty(t, err)
//
//		require.Greater(t, gasUsedLong, gasUsedShort)
//		println("Gas used value-key", gasUsedLongValue, gasUsedLong)
//
//		require.Equal(t, gasUsedLongValue, gasUsedLong)
//
//		println("Gas used long-short", gasUsedLong, gasUsedShort)
//
//	})
//}

func TestQueryGasPrice(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			t.Run("Query to Self Gas Price", func(t *testing.T) {
				_, _, gasUsed, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests, 0)
				require.Empty(t, err)
				// require that more gas was used than the base 20K (10K for execute, another 10K for query)
				require.Greater(t, gasUsed, uint64(20_000))
			})
		})
	}
}

func TestCodeHashExecCallQuery(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests, 0)

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
				_, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Got an error from query: ParseErr { target: \"test_contract::contract::QueryMsg\", msg: \"Expected to parse either a `true`, `false`, or a `null`.\", backtrace: None }",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestCodeHashQueryCallQuery(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				output, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

				require.Empty(t, err)
				require.Equal(t, "2", output)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Got an error from query: ParseErr { target: \"test_contract::contract::QueryMsg\", msg: \"Expected to parse either a `true`, `false`, or a `null`.\", backtrace: None }",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, false, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestEncryptedAndPlaintextLogs(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/plaintext_logs.wasm")

	addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	_, events, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, "{}", true, false, defaultGasForTests, 0, 1)

	require.Empty(t, err)
	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: addr.String()},
				{Key: "encrypted log", Value: "encrypted value"},
				{Key: "ZW5jb2RlZCBsb2cK", Value: "ZW5jb2RlZCB2YWx1ZQo="},
				{Key: "plaintext log", Value: "plaintext value"},
			},
		},
		events,
	)
}

func TestSecp256k1Verify(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/

			t.Run("CorrectCompactPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectLongPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectMsgHashCompactPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectMsgHashLongPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectSigCompactPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectSigLongPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectCompactPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"AoSdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXc","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectLongPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BISdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXcFWl11YCgu65hzvNDQE2Qo1hwTMQ/42Xif8O/MrxzvxI=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
		})
	}
}

func TestBenchmarkSecp256k1VerifyAPI(t *testing.T) {
	t.SkipNow()
	// Assaf: I wrote the benchmark like this because the init functions take testing.T
	// and not testing.B and I just wanted to quickly get a feel for the perf improvments
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

			start := time.Now()
			// https://paulmillr.com/noble/
			execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":10,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, defaultGasForTests, 0)
			elapsed := time.Since(start)
			fmt.Printf("TestBenchmarkSecp256k1VerifyAPI took %s\n", elapsed)
		})
	}
}

func TestBenchmarkSecp256k1VerifyCrate(t *testing.T) {
	t.SkipNow()
	// Assaf: I wrote the benchmark like this because the init functions take testing.T
	// and not testing.B and I just wanted to quickly get a feel for the perf improvments
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

			start := time.Now()
			// https://paulmillr.com/noble/
			execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify_from_crate":{"iterations":10,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, 100_000_000, 0)
			elapsed := time.Since(start)
			fmt.Printf("TestBenchmarkSecp256k1VerifyCrate took %s\n", elapsed)
		})
	}
}

func TestEd25519Verify(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectMsg", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmUK"}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectSig", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
		})
	}
}

func TestEd25519BatchVerify(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("100Correct", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU="]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectPubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectMsg", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectSig", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("CorrectEmptySigsEmptyMsgsOnePubkey", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":[],"msgs":[]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectEmpty", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":[]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectEmptyPubkeysEmptySigsOneMsg", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectMultisig", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectMultiMsgOneSigner", func(t *testing.T) {
				_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg==","uuNxLEzAYDbuJ+BiYN94pTqhD7UhvCJNbxAbnWz0B9DivkPXmqIULko0DddP2/tVXPtjJ90J20faiWCEC3QkDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","cGVhY2Ugb3V0"]}}`, true, false, defaultGasForTests, 0)

				require.Empty(t, err)
				require.Equal(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
		})
	}
}

func TestSecp256k1RecoverPubkey(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/
			_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":0,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//"},
					},
				},
				events,
			)

			_, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":1,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "Ams198xOCEVnc/ESvxF2nxnE3AVFO8ahB22S1ZgX2vSR"},
					},
				},
				events,
			)
		})
	}
}

func TestSecp256k1Sign(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// priv iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao=
			// pub ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao="}}`, true, false, defaultGasForTests, 0)
			require.Empty(t, err)

			signature := events[0][1].Value

			_, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"secp256k1_verify":{"iterations":1,"pubkey":"ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS","sig":"%s","msg_hash":"K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA="}}`, signature), true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "true"},
					},
				},
				events,
			)
		})
	}
}

func TestEd25519Sign(t *testing.T) {
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
			require.Empty(t, initErr)

			// priv z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo=
			// pub jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			_, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo="}}`, true, false, defaultGasForTests, 0)
			require.Empty(t, err)

			signature := events[0][1].Value

			_, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"ed25519_verify":{"iterations":1,"pubkey":"jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=","sig":"%s","msg":"d2VuIG1vb24="}}`, signature), true, false, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "true"},
					},
				},
				events,
			)
		})
	}
}

func TestBenchmarkEd25519BatchVerifyAPI(t *testing.T) {
	t.SkipNow()
	// Assaf: I wrote the benchmark like this because the init functions take testing.T
	// and not testing.B and I just wanted to quickly get a feel for the performance improvments
	for _, tc := range testContracts {
		t.Run(tc.Version, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, tc.Wasm)

			contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

			start := time.Now()
			_, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1000,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU="]}}`, true, false, math.MaxUint64, 0)

			require.Empty(t, err)

			elapsed := time.Since(start)
			fmt.Printf("TestBenchmarkEd25519BatchVerifyAPI took %s\n", elapsed)
		})
	}
}

type GetResponse struct {
	Count uint32 `json:"count"`
}
type v1QueryResponse struct {
	Get GetResponse `json:"get"`
}

func TestV1EndpointsSanity(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract.wasm")

	contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)

	data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment":{"addition": 13}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(23), binary.BigEndian.Uint32(data))

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(23), resp.Get.Count)
}

func TestV1QueryWorksWithEnv(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract.wasm")

	contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":0}}`, true, true, defaultGasForTests)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 10)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(0), resp.Get.Count)
}

func TestV1ReplySanity(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract.wasm")

	contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	fmt.Printf("LIORRR %s", string(contractAddress))

	// data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment":{"addition": 13}}`, true, true, math.MaxUint64, 0)

	// require.Empty(t, err)
	// require.Equal(t, uint32(23), binary.BigEndian.Uint32(data))

	// data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"transfer_money":{"amount": 10213}}`, true, true, math.MaxUint64, 0)

	// require.Empty(t, err)
	// require.Equal(t, uint32(23), binary.BigEndian.Uint32(data))

	// data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"recursive_reply":{}}`, true, true, math.MaxUint64, 0)

	// require.Empty(t, err)
	// require.Equal(t, uint32(25), binary.BigEndian.Uint32(data))

	// data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"recursive_reply_fail":{}}`, true, true, math.MaxUint64, 0)

	// require.Empty(t, err)
	// require.Equal(t, uint32(10), binary.BigEndian.Uint32(data))

	data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"init_new_contract":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(150), binary.BigEndian.Uint32(data))

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(10), resp.Get.Count)
}
