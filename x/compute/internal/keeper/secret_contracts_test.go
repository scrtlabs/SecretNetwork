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
	abci "github.com/tendermint/tendermint/abci/types"

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
	CosmWasmVersion string
	IsCosmWasmV1    bool
	WasmFilePath    string
}

var testContracts = []TestContract{
	{
		CosmWasmVersion: "v0.10",
		IsCosmWasmV1:    false,
		WasmFilePath:    "./testdata/test-contract/contract.wasm",
	}, {
		CosmWasmVersion: "v1",
		IsCosmWasmV1:    true,
		WasmFilePath:    "./testdata/v1-sanity-contract/contract.wasm",
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

func setupTest(t *testing.T, wasmPath string, additionalCoinsInWallets sdk.Coins) (sdk.Context, Keeper, uint64, string, sdk.AccAddress, crypto.PrivKey, sdk.AccAddress, crypto.PrivKey) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	walletA, privKeyA := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 200000)).Add(additionalCoinsInWallets...))
	walletB, privKeyB := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 5000)).Add(additionalCoinsInWallets...))

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

func decryptAttribute(attr v010cosmwasm.LogAttribute, nonce []byte) (v010cosmwasm.LogAttribute, error) {
	var newAttr v010cosmwasm.LogAttribute

	keyCipherBz, err := base64.StdEncoding.DecodeString(attr.Key)
	if err != nil {
		return v010cosmwasm.LogAttribute{}, fmt.Errorf("Failed DecodeString for key %+v", attr.Key)
	}
	keyPlainBz, err := wasmCtx.Decrypt(keyCipherBz, nonce)
	if err != nil {
		return v010cosmwasm.LogAttribute{}, fmt.Errorf("Failed Decrypt for key %+v", keyPlainBz)
	}
	newAttr.Key = string(keyPlainBz)

	// value
	valueCipherBz, err := base64.StdEncoding.DecodeString(attr.Value)
	if err != nil {
		return v010cosmwasm.LogAttribute{}, fmt.Errorf("Failed DecodeString for value %+v", attr.Value)
	}
	valuePlainBz, err := wasmCtx.Decrypt(valueCipherBz, nonce)
	if err != nil {
		return v010cosmwasm.LogAttribute{}, fmt.Errorf("Failed Decrypt for value %+v", valuePlainBz)
	}
	newAttr.Value = string(valuePlainBz)

	return newAttr, nil
}

func parseAndDecryptAttributes(attrs []abci.EventAttribute, nonce []byte, shouldDecrypt bool) ([]v010cosmwasm.LogAttribute, error) {
	var newAttrs []v010cosmwasm.LogAttribute
	for _, a := range attrs {
		var attr v010cosmwasm.LogAttribute
		attr.Key = string(a.Key)
		attr.Value = string(a.Value)

		if attr.Key == "contract_address" {
			newAttrs = append(newAttrs, attr)
			continue
		}

		if shouldDecrypt {
			newAttr, err := decryptAttribute(attr, nonce)
			if err != nil {
				return nil, err
			}

			newAttrs = append(newAttrs, newAttr)
		} else {
			newAttrs = append(newAttrs, attr)
		}

	}

	return newAttrs, nil
}

// tryDecryptWasmEvents gets all "wasm" events and try to decrypt what it can.
// Returns all "wasm" events, including from contract callbacks.
// The difference between this and getDecryptedWasmEvents is that it is aware of plaintext logs.
func tryDecryptWasmEvents(ctx sdk.Context, nonce []byte, shouldSkipAttributes ...bool) []ContractEvent {
	events := ctx.EventManager().Events()
	shouldSkip := (len(shouldSkipAttributes) > 0 && shouldSkipAttributes[0])
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

				if !shouldSkip && newLog.Key != "contract_address" {
					// key
					newAttr, err := decryptAttribute(newLog, nonce)
					if err != nil {
						continue
					}

					newEvent[len(newEvent)-1] = newAttr
				}
			}
			res = append(res, newEvent)
		}
	}
	return res
}

// getDecryptedData decrypts the output of the first function to be called
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

// wrap the default gas meter with a counter of wasm calls
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
		// default, just check that at least 1 call happened
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
	isErrorEncrypted bool, isV1Contract bool, gas uint64, coin int64, shouldSkipAttributes ...bool,
) ([]byte, sdk.Context, []byte, []ContractEvent, uint64, cosmwasm.StdError) {
	return execHelperImpl(t, keeper, ctx, contractAddress, txSender, senderPrivKey, execMsg, isErrorEncrypted, isV1Contract, gas, coin, -1, shouldSkipAttributes...)
}

func execHelperImpl(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddress sdk.AccAddress, txSender sdk.AccAddress, senderPrivKey crypto.PrivKey, execMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, coin int64, wasmCallCount int64, shouldSkipAttributes ...bool,
) ([]byte, sdk.Context, []byte, []ContractEvent, uint64, cosmwasm.StdError) {
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
		// default, just check that at least 1 call happened
		require.NotZero(t, gasMeter.GetWasmCounter(), err)
	} else {
		require.Equal(t, uint64(wasmCallCount), gasMeter.GetWasmCounter(), err)
	}

	if err != nil {
		return nil, ctx, nil, nil, 0, extractInnerError(t, err, nonce, isErrorEncrypted, isV1Contract)
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := tryDecryptWasmEvents(ctx, nonce, shouldSkipAttributes...)

	// TODO check if we can extract the messages from ctx

	// Data is the output of only the first call
	data := getDecryptedData(t, execResult.Data, nonce)

	return nonce, ctx, data, wasmEvents, gasUsed, cosmwasm.StdError{}
}

func initHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	codeID uint64, creator sdk.AccAddress, creatorPrivKey crypto.PrivKey, initMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, shouldSkipAttributes ...bool,
) ([]byte, sdk.Context, sdk.AccAddress, []ContractEvent, cosmwasm.StdError) {
	return initHelperImpl(t, keeper, ctx, codeID, creator, creatorPrivKey, initMsg, isErrorEncrypted, isV1Contract, gas, -1, sdk.NewCoins(), shouldSkipAttributes...)
}

func initHelperImpl(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	codeID uint64, creator sdk.AccAddress, creatorPrivKey crypto.PrivKey, initMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, wasmCallCount int64, sentFunds sdk.Coins, shouldSkipAttributes ...bool,
) ([]byte, sdk.Context, sdk.AccAddress, []ContractEvent, cosmwasm.StdError) {
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

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, creatorPrivKey, initMsgBz, codeID, sentFunds)
	// make the label a random base64 string, because why not?
	contractAddress, _, err := keeper.Instantiate(ctx, codeID, creator /* nil,*/, initMsgBz, base64.RawURLEncoding.EncodeToString(nonce), sentFunds, nil)

	if wasmCallCount < 0 {
		// default, just check that at least 1 call happened
		require.NotZero(t, gasMeter.GetWasmCounter(), err)
	} else {
		require.Equal(t, uint64(wasmCallCount), gasMeter.GetWasmCounter(), err)
	}

	if err != nil {
		return nil, ctx, nil, nil, extractInnerError(t, err, nonce, isErrorEncrypted, isV1Contract)
	}

	// wasmEvents comes from all the callbacks as well
	wasmEvents := tryDecryptWasmEvents(ctx, nonce, shouldSkipAttributes...)

	// TODO check if we can extract the messages from ctx

	return nonce, ctx, contractAddress, wasmEvents, cosmwasm.StdError{}
}

func TestCallbackSanity(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
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

			_, _, data, execEvents, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":2,"y":3}}`, contractAddress.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
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
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, "./testdata/erc20.wasm", sdk.NewCoins())

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, initMsg, true, false, defaultGasForTests)
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
	_, _, data, wasmEvents, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA,
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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"empty_log_key_value":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"empty_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Empty(t, data)
		})
	}
}

func TestNoData(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"no_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Empty(t, data)
		})
	}
}

func TestExecuteIllegalInputError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `bad input`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, execErr.GenericErr)
				require.Contains(t, execErr.GenericErr.Msg, "Error parsing")
			} else {
				require.NotNil(t, execErr.ParseErr)
			}
		})
	}
}

func TestInitIllegalInputError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, _, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `bad input`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, initErr.GenericErr)
				require.Contains(t, initErr.GenericErr.Msg, "Error parsing")
			} else {
				require.NotNil(t, initErr.ParseErr)
			}
		})
	}
}

func TestCallbackFromInitAndCallbackEvents(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init first contract so we'd have someone to callback
			_, _, firstContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
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
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback":{"contract_addr":"%s", "code_hash": "%s"}}`, firstContractAddress.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests)
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
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, "./testdata/erc20.wasm", sdk.NewCoins())

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, initMsg, true, false, defaultGasForTests)
	require.Empty(t, err)
	// require.Empty(t, initEvents)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"address":"blabla"}}`, true, false, defaultGasForTests)

	require.NotNil(t, qErr.GenericErr)
	require.Equal(t, "canonicalize_address errored: invalid length", qErr.GenericErr.Msg)
}

func TestUnicodeData(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestInitContractError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			t.Run("generic_err", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"generic_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotNil(t, err.GenericErr)
				require.Contains(t, err.GenericErr.Msg, "la la 🤯")
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ra ra 🤯")
				} else {
					require.NotNil(t, err.InvalidBase64)
					require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
				}
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ka ka 🤯")
				} else {
					require.NotNil(t, err.InvalidUtf8)
					require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
				}
			})
			t.Run("not_found", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"not_found"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "za za 🤯")
				} else {
					require.NotNil(t, err.NotFound)
					require.Equal(t, "za za 🤯", err.NotFound.Kind)
				}
			})
			t.Run("parse_err", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"parse_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "na na 🤯")
					require.Contains(t, err.GenericErr.Msg, "pa pa 🤯")
				} else {
					require.NotNil(t, err.ParseErr)
					require.Equal(t, "na na 🤯", err.ParseErr.Target)
					require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
				}
			})
			t.Run("serialize_err", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"serialize_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ba ba 🤯")
					require.Contains(t, err.GenericErr.Msg, "ga ga 🤯")
				} else {
					require.NotNil(t, err.SerializeErr)
					require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
					require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
				}
			})
			t.Run("unauthorized", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"unauthorized"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Unauthorized)
				}
			})
			t.Run("underflow", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"underflow"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Underflow)
					require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
					require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
				}
			})
		})
	}
}

func TestExecContractError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			t.Run("generic_err", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"generic_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotNil(t, err.GenericErr)
				require.Contains(t, err.GenericErr.Msg, "la la 🤯")
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ra ra 🤯")
				} else {
					require.NotNil(t, err.InvalidBase64)
					require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
				}
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ka ka 🤯")
				} else {
					require.NotNil(t, err.InvalidUtf8)
					require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
				}
			})
			t.Run("not_found", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"not_found"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "za za 🤯")
				} else {
					require.NotNil(t, err.NotFound)
					require.Equal(t, "za za 🤯", err.NotFound.Kind)
				}
			})
			t.Run("parse_err", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"parse_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "na na 🤯")
					require.Contains(t, err.GenericErr.Msg, "pa pa 🤯")
				} else {
					require.NotNil(t, err.ParseErr)
					require.Equal(t, "na na 🤯", err.ParseErr.Target)
					require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
				}
			})
			t.Run("serialize_err", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"serialize_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ba ba 🤯")
					require.Contains(t, err.GenericErr.Msg, "ga ga 🤯")
				} else {
					require.NotNil(t, err.SerializeErr)
					require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
					require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
				}
			})
			t.Run("unauthorized", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"unauthorized"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Unauthorized)
				}
			})
			t.Run("underflow", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"underflow"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Underflow)
					require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
					require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
				}
			})
		})
	}
}

func TestQueryContractError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			t.Run("generic_err", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"generic_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotNil(t, err.GenericErr)
				require.Contains(t, err.GenericErr.Msg, "la la 🤯")
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"invalid_base64"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ra ra 🤯")
				} else {
					require.NotNil(t, err.InvalidBase64)
					require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
				}
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ka ka 🤯")
				} else {
					require.NotNil(t, err.InvalidUtf8)
					require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
				}
			})
			t.Run("not_found", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"not_found"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "za za 🤯")
				} else {
					require.NotNil(t, err.NotFound)
					require.Equal(t, "za za 🤯", err.NotFound.Kind)
				}
			})
			t.Run("parse_err", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"parse_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "na na 🤯")
					require.Contains(t, err.GenericErr.Msg, "pa pa 🤯")
				} else {
					require.NotNil(t, err.ParseErr)
					require.Equal(t, "na na 🤯", err.ParseErr.Target)
					require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
				}
			})
			t.Run("serialize_err", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"serialize_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ba ba 🤯")
					require.Contains(t, err.GenericErr.Msg, "ga ga 🤯")
				} else {
					require.NotNil(t, err.SerializeErr)
					require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
					require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
				}
			})
			t.Run("unauthorized", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"unauthorized"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Unauthorized)
				}
			})
			t.Run("underflow", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddr, `{"contract_error":{"error_type":"underflow"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Underflow)
					require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
					require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
				}
			})
		})
	}
}

func TestInitParamError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			codeHash := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
			msg := fmt.Sprintf(`{"callback":{"contract_addr":"notanaddress", "code_hash":"%s"}}`, codeHash)

			_, _, _, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, msg, false, testContract.IsCosmWasmV1, defaultGasForTests)

			require.Contains(t, initErr.Error(), "invalid address")
		})
	}
}

func TestCallbackExecuteParamError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			msg := fmt.Sprintf(`{"a":{"code_hash":"%s","contract_addr":"notanaddress","x":2,"y":3}}`, codeHash)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Contains(t, err.Error(), "invalid address")
		})
	}
}

func TestQueryInputStructureError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, "./testdata/erc20.wasm", sdk.NewCoins())

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, initMsg, true, false, defaultGasForTests)
	require.Empty(t, err)
	// require.Empty(t, initEvents)

	_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"balance":{"invalidkey":"invalidval"}}`, true, false, defaultGasForTests)

	require.NotNil(t, qErr.ParseErr)
	require.Contains(t, qErr.ParseErr.Msg, "missing field `address`")
}

func TestInitNotEncryptedInputError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKey, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, err := keeper.QuerySmart(ctx, contractAddress, []byte(`{"owner":{}}`), false)
			require.Error(t, err)

			require.Contains(t, err.Error(), "failed to decrypt data")
		})
	}
}

func TestInitNoLogs(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, _, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"no_logs":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			require.Empty(t, initErr)
			////require.Empty(t, initEvents)
		})
	}
}

func TestExecNoLogs(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"no_logs":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			// require.Empty(t, execEvents)
		})
	}
}

func TestExecCallbackToInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init first contract
			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// init second contract and callback to the first contract
			_, _, execData, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
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

			_, _, data, _, _, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			// require.Empty(t, execEvents)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestInitCallbackToInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests)
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

			_, _, data, _, _, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			// require.Empty(t, execEvents)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestInitCallbackContractError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, secondContractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s", "code_hash":"%s"}}`, contractAddress, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests)

			require.NotNil(t, initErr.GenericErr)
			require.Contains(t, initErr.GenericErr.Msg, "la la 🤯")
			require.Empty(t, secondContractAddress)
			// require.Empty(t, initEvents)
		})
	}
}

func TestExecCallbackContractError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s","code_hash":"%s"}}`, contractAddress, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "la la 🤯")
			// require.Empty(t, execEvents)
			require.Empty(t, data)
		})
	}
}

func TestExecCallbackBadParam(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, execErr.GenericErr)
				require.Contains(t, execErr.GenericErr.Msg, "v1_sanity_contract::msg::ExecuteMsg")
				require.Contains(t, execErr.GenericErr.Msg, "unknown variant `callback_contract_bad_param`")
			} else {
				require.NotNil(t, execErr.ParseErr)
				require.Equal(t, "test_contract::contract::HandleMsg", execErr.ParseErr.Target)
				require.Contains(t, execErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
			}
			// require.Empty(t, execEvents)
			require.Empty(t, data)
		})
	}
}

func TestInitCallbackBadParam(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init first
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, secondContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, secondContractAddress)
			// require.Empty(t, initEvents)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, initErr.GenericErr)
				require.Contains(t, initErr.GenericErr.Msg, "v1_sanity_contract::msg::InstantiateMsg")
				require.Contains(t, initErr.GenericErr.Msg, "unknown variant `callback_contract_bad_param`")
			} else {
				require.NotNil(t, initErr.ParseErr)
				require.Equal(t, "test_contract::contract::InitMsg", initErr.ParseErr.Target)
				require.Contains(t, initErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
			}
		})
	}
}

func TestState(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Empty(t, data)

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(data))

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"remove_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Empty(t, data)
		})
	}
}

func TestCanonicalizeAddressErrors(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			// this function should handle errors internally and return gracefully
			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"test_canonicalize_address_errors":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🤟", string(data))
		})
	}
}

func TestInitPanic(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, _, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"panic":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests)

			require.NotNil(t, initErr.GenericErr)
			require.Contains(t, initErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestExecPanic(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"panic":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestQueryPanic(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, queryErr := queryHelper(t, keeper, ctx, addr, `{"panic":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests)
			require.NotNil(t, queryErr.GenericErr)
			require.Contains(t, queryErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseMemoryLimit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":13631488}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			// this should fail with memory error because 13MiB is more than the allowed 12MiB

			require.Empty(t, data)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseGasLimit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// ensure we get an out of gas panic
			defer func() {
				r := recover()
				require.NotNil(t, r)
				_, ok := r.(sdk.ErrorOutOfGas)
				require.True(t, ok, "%+v", r)
			}()

			_, _, _, _, _, _ = execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, testContract.IsCosmWasmV1, 9_000_000, 0)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
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
					_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"pass_null_pointer_to_imports_should_throw":{"pass_type":"%s"}}`, passType), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

					require.NotNil(t, execErr.GenericErr)
					if testContract.IsCosmWasmV1 {
						require.Contains(t, execErr.GenericErr.Msg, "execute contract failed")
					} else {
						require.Contains(t, execErr.GenericErr.Msg, "failed to read memory")
					}
				})
			}
		})
	}
}

func TestExternalQueryWorks(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, execErr)
			require.Equal(t, []byte{3}, data)
		})
	}
}

func TestExternalQueryCalleePanic(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_panic":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestExternalQueryCalleeStdError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_error":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "la la 🤯")
		})
	}
}

func TestExternalQueryCalleeDoesntExist(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"send_external_query_error":{"to":"secret13l72vhjngmg55ykajxdnlalktwglyqjqv9pkq4","code_hash":"bla bla"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "not found")
		})
	}
}

func TestExternalQueryBadSenderABI(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_bad_abi":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, err.GenericErr)
				require.Contains(t, err.GenericErr.Msg, "v1_sanity_contract::msg::QueryMsg")
				require.Contains(t, err.GenericErr.Msg, "Invalid type")
			} else {
				require.NotNil(t, err.ParseErr)
				require.Equal(t, "test_contract::contract::QueryMsg", err.ParseErr.Target)
				require.Equal(t, "Invalid type", err.ParseErr.Msg)
			}
		})
	}
}

func TestExternalQueryBadReceiverABI(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_bad_abi_receiver":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, err.GenericErr)
				require.Contains(t, err.GenericErr.Msg, "alloc::string::String")
				require.Contains(t, err.GenericErr.Msg, "Invalid type")
			} else {
				require.NotNil(t, err.ParseErr)
				require.Equal(t, "alloc::string::String", err.ParseErr.Target)
				require.Equal(t, "Invalid type", err.ParseErr.Msg)
			}
		})
	}
}

func TestMsgSenderInCallback(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"callback_to_log_msg_sender":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			data, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"send_external_query_infinite_loop":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests)

			require.Empty(t, data)
			require.NotNil(t, err.GenericErr)
			require.Equal(t, err.GenericErr.Msg, "query contract failed: Execution error: Enclave: enclave ran out of heap memory")
		})
	}
}

func TestQueryRecursionLimitEnforcedInQueries(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			data, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, 2*defaultGasForTests)

			require.NotEmpty(t, data)
			require.Equal(t, data, "\"Recursion limit was correctly enforced\"")

			require.Nil(t, err.GenericErr)
		})
	}
}

func TestQueryRecursionLimitEnforcedInHandles(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, 2*defaultGasForTests, 0)

			require.NotEmpty(t, data)
			require.Equal(t, string(data), "\"Recursion limit was correctly enforced\"")

			require.Nil(t, err.GenericErr)
		})
	}
}

func TestQueryRecursionLimitEnforcedInInits(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// Initialize a contract that we will be querying
			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			// Initialize the contract that will be running the test
			_, _, addr, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, 2*defaultGasForTests)
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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, queryErr := queryHelper(t, keeper, ctx, addr, `{"write_to_storage": {}}`, false, testContract.IsCosmWasmV1, defaultGasForTests)
			require.NotNil(t, queryErr.GenericErr)
			require.Contains(t, queryErr.GenericErr.Msg, "contract tried to write to storage during a query")
		})
	}
}

func TestRemoveFromStorageDuringQuery(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, queryErr := queryHelper(t, keeper, ctx, addr, `{"remove_from_storage": {}}`, false, testContract.IsCosmWasmV1, defaultGasForTests)
			require.NotNil(t, queryErr.GenericErr)
			require.Contains(t, queryErr.GenericErr.Msg, "contract tried to write to storage during a query")
		})
	}
}

func TestDepositToContract(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())

			require.Equal(t, `[{"denom":"denom","amount":"17"}]`, string(data))
		})
	}
}

func TestContractSendFunds(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsBefore.String())
			require.Equal(t, "199983denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, addr.String(), walletA.String(), "denom", 17), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "200000denom", walletCoinsAfter.String())

			require.Empty(t, execErr)
		})
	}
}

// In V1 there is no "from" field in Bank message functionality which means it shouldn't be tested
func TestContractTryToSendFundsFromSomeoneElse(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm", sdk.NewCoins())

	_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, false, defaultGasForTests, 17)

	require.Empty(t, execErr)

	contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
	walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

	require.Equal(t, "17denom", contractCoinsBefore.String())
	require.Equal(t, "199983denom", walletCoinsBefore.String())

	_, _, _, _, _, execErr = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, walletA.String(), addr.String(), "denom", 17), false, false, defaultGasForTests, 0)

	require.NotNil(t, execErr.GenericErr)
	require.Contains(t, execErr.GenericErr.Msg, "contract doesn't have permission")
}

func TestContractSendFundsToInitCallback(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, execEvents, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_init_callback":{"code_id":%d,"denom":"%s","amount":%d,"code_hash":"%s"}}`, codeID, "denom", 17, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.Empty(t, execErr)
			require.NotEmpty(t, execEvents)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			newContract, err := sdk.AccAddressFromBech32(execEvents[1][0].Value)
			require.NoError(t, err)
			newContractCoins := keeper.bankKeeper.GetAllBalances(ctx, newContract)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
			require.Equal(t, "17denom", newContractCoins.String())
		})
	}
}

func TestContractSendFundsToInitCallbackNotEnough(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_init_callback":{"code_id":%d,"denom":"%s","amount":%d,"code_hash":"%s"}}`, codeID, "denom", 18, codeHash), false, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			// require.Empty(t, execEvents)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "insufficient funds")

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
		})
	}
}

func TestContractSendFundsToExecCallback(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 17, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "17denom", contract2CoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
		})
	}
}

func TestContractSendFundsToExecCallbackNotEnough(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 19, codeHash), false, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "insufficient funds")

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "", contract2CoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
		})
	}
}

func TestSleep(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"sleep":{"ms":3000}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Error(t, execErr)
			require.Error(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestGasIsChargedForInitCallbackToInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d,"code_hash":"%s"}}`, codeID, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForInitCallbackToExec(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback":{"contract_addr":"%s","code_hash":"%s"}}`, addr, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForExecCallbackToInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// exec callback to init
			_, _, _, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d,"code_hash":"%s"}}`, codeID, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForExecCallbackToExec(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// exec callback to exec
			_, _, _, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":1,"y":2}}`, addr, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 3)
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForExecExternalQuery(t *testing.T) {
	t.SkipNow() // as of v0.10 CosmWasm are overriding the default gas meter

	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_depth_counter":{"to":"%s","depth":2,"code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 3)
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForInitExternalQuery(t *testing.T) {
	t.SkipNow() // as of v0.10 CosmWasm are overriding the default gas meter

	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_depth_counter":{"to":"%s","depth":2,"code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 3, sdk.NewCoins())
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForQueryExternalQuery(t *testing.T) {
	t.SkipNow() // as of v0.10 CosmWasm are overriding the default gas meter

	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, err := queryHelperImpl(t, keeper, ctx, addr, fmt.Sprintf(`{"send_external_query_depth_counter":{"to":"%s","depth":2,"code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 3)
			require.Empty(t, err)
		})
	}
}

func TestWasmTooHighInitialMemoryRuntimeFail(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/test-contract/too-high-initial-memory.wasm", sdk.NewCoins())

	_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, false, defaultGasForTests)
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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/test-contract/contract_with_floats.wasm", sdk.NewCoins())

			_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests)
			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "found floating point operation in module code")
		})
	}
}

func TestCodeHashInvalid(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())
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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())
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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())
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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			initMsg := []byte(codeHash + "a" + `{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA /* nil, */, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)

			initErr := extractInnerError(t, err, enc[0:32], true, testContract.IsCosmWasmV1)
			require.NotEmpty(t, initErr)
			require.Contains(t, initErr.Error(), "Expected to parse either a `true`, `false`, or a `null`.")
		})
	}
}

func TestCodeHashWrong(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, addr, events, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

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
				_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"","msg":"%s","label":"2"}}`, codeID, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%sa","msg":"%s","label":"3"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"4"}}`, codeID, codeHash[0:63], `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s","label":"5"}}`, codeID, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 1, sdk.NewCoins())
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, addr2, events, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

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
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, addr2, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

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
				_, _, _, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, _, events, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

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
				_, _, _, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"","msg":"%s","label":"2"}}`, codeID, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%sa","msg":"%s","label":"3"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				if testContract.IsCosmWasmV1 {
					require.Contains(t,
						err.Error(),
						"v1_sanity_contract::msg::InstantiateMsg: Expected to parse either a `true`, `false`, or a `null`.",
					)
				} else {
					require.Contains(t,
						err.Error(),
						"parsing test_contract::contract::InitMsg: Expected to parse either a `true`, `false`, or a `null`.",
					)
				}
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"4"}}`, codeID, codeHash[0:63], `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s","label":"5"}}`, codeID, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0, 1)
			require.NotEmpty(t, err)
			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "contract account already exists")
		})
	}
}

func TestCodeHashExecCallExec(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr, codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				if testContract.IsCosmWasmV1 {
					require.Contains(t,
						err.Error(),
						"v1_sanity_contract::msg::ExecuteMsg: Expected to parse either a `true`, `false`, or a `null`.",
					)
				} else {
					require.Contains(t,
						err.Error(),
						"parsing test_contract::contract::HandleMsg: Expected to parse either a `true`, `false`, or a `null`.",
					)
				}
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, codeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestQueryGasPrice(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			t.Run("Query to Self Gas Price", func(t *testing.T) {
				_, _, _, _, gasUsed, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
				require.Empty(t, err)
				// require that more gas was used than the base 20K (10K for execute, another 10K for query)
				require.Greater(t, gasUsed, uint64(20_000))
			})
		})
	}
}

func TestCodeHashExecCallQuery(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				if testContract.IsCosmWasmV1 {
					require.Contains(t,
						err.Error(),
						"Expected to parse either a `true`, `false`, or a `null`",
					)
				} else {
					require.Contains(t,
						err.Error(),
						"Got an error from query: ParseErr { target: \"test_contract::contract::QueryMsg\", msg: \"Expected to parse either a `true`, `false`, or a `null`.\", backtrace: None }",
					)
				}
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				output, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.Empty(t, err)
				require.Equal(t, "2", output)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				if testContract.IsCosmWasmV1 {
					require.Contains(t,
						err.Error(),
						"Expected to parse either a `true`, `false`, or a `null`",
					)
				} else {
					require.Contains(t,
						err.Error(),
						"Got an error from query: ParseErr { target: \"test_contract::contract::QueryMsg\", msg: \"Expected to parse either a `true`, `false`, or a `null`.\", backtrace: None }",
					)
				}
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/plaintext_logs.wasm", sdk.NewCoins())

	_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	_, _, _, events, _, err := execHelperImpl(t, keeper, ctx, addr, walletA, privKeyA, "{}", true, false, defaultGasForTests, 0, 1)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/

			t.Run("CorrectCompactPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"AoSdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXc","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BISdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXcFWl11YCgu65hzvNDQE2Qo1hwTMQ/42Xif8O/MrxzvxI=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	// and not testing.B and I just wanted to quickly get a feel for the perf improvements
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			start := time.Now()
			// https://paulmillr.com/noble/
			execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":10,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			elapsed := time.Since(start)
			fmt.Printf("TestBenchmarkSecp256k1VerifyAPI took %s\n", elapsed)
		})
	}
}

func TestBenchmarkSecp256k1VerifyCrate(t *testing.T) {
	t.SkipNow()
	// Assaf: I wrote the benchmark like this because the init functions take testing.T
	// and not testing.B and I just wanted to quickly get a feel for the perf improvements
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			start := time.Now()
			// https://paulmillr.com/noble/
			execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify_from_crate":{"iterations":10,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, 100_000_000, 0)
			elapsed := time.Since(start)
			fmt.Printf("TestBenchmarkSecp256k1VerifyCrate took %s\n", elapsed)
		})
	}
}

func TestEd25519Verify(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmUK"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":[],"msgs":[]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":[]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg==","uuNxLEzAYDbuJ+BiYN94pTqhD7UhvCJNbxAbnWz0B9DivkPXmqIULko0DddP2/tVXPtjJ90J20faiWCEC3QkDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","cGVhY2Ugb3V0"]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":0,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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

			_, _, _, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":1,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// priv iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao=
			// pub ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, err)

			signature := events[0][1].Value

			_, _, _, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"secp256k1_verify":{"iterations":1,"pubkey":"ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS","sig":"%s","msg_hash":"K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA="}}`, signature), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// priv z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo=
			// pub jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, err)

			signature := events[0][1].Value

			_, _, _, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"ed25519_verify":{"iterations":1,"pubkey":"jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=","sig":"%s","msg":"d2VuIG1vb24="}}`, signature), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

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
	// and not testing.B and I just wanted to quickly get a feel for the performance improvements
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			start := time.Now()
			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1000,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, math.MaxUint64, 0)

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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment":{"addition": 13}}`, true, true, math.MaxUint64, 0)

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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":0}}`, true, true, defaultGasForTests)
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment":{"addition": 13}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(23), binary.BigEndian.Uint32(data))

	_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"transfer_money":{"amount": 10213}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(23), binary.BigEndian.Uint32(data))

	_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"recursive_reply":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(25), binary.BigEndian.Uint32(data))

	_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"recursive_reply_fail":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(10), binary.BigEndian.Uint32(data))

	_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"init_new_contract":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(150), binary.BigEndian.Uint32(data))

	_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"init_new_contract_with_error":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(1337), binary.BigEndian.Uint32(data))

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(1337), resp.Get.Count)
}

func TestInitCreateNewContract(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, ev, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
			_, _, _, ev, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"init_new_contract":{}}`, true, testContract.IsCosmWasmV1, math.MaxUint64, 0)

			require.Empty(t, err)
			newContractAddress, Aerr := sdk.AccAddressFromBech32(ev[1][0].Value)
			require.Empty(t, Aerr)
			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
			require.Empty(t, qErr)

			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(10), resp.Get.Count)

			queryRes, qErr = queryHelper(t, keeper, ctx, newContractAddress, `{"get":{}}`, true, true, math.MaxUint64)
			require.Empty(t, qErr)

			e = json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(150), resp.Get.Count)
		})
	}
}

func TestV1ReplyLoop(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"sub_msg_loop":{"iter": 10}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(20), binary.BigEndian.Uint32(data))
}

func TestV010BankMsgSendFrom(t *testing.T) {
	for _, callType := range []string{"init", "exec"} {
		t.Run(callType, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, "./testdata/test-contract/contract.wasm", sdk.NewCoins())

			var err cosmwasm.StdError
			var contractAddress sdk.AccAddress

			if callType == "init" {
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","from":"%s","amount":[{"amount":"1","denom":"denom"}]}}`, walletB.String(), walletA.String()), false, false, defaultGasForTests, -1, sdk.NewCoins())
			} else {
				_, _, contractAddress, _, _ = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, false, defaultGasForTests, -1, sdk.NewCoins())

				_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","from":"%s","amount":[{"amount":"1","denom":"denom"}]}}`, walletB.String(), walletA.String()), false, false, math.MaxUint64, 0)
			}

			require.NotEmpty(t, err)
			require.Contains(t, err.Error(), "contract doesn't have permission to send funds from another account")
		})
	}
}

func TestBankMsgSend(t *testing.T) {
	for _, contract := range testContracts {
		t.Run(contract.CosmWasmVersion, func(t *testing.T) {
			for _, callType := range []string{"init", "exec"} {
				t.Run(callType, func(t *testing.T) {
					for _, test := range []struct {
						description    string
						input          string
						isSuccuss      bool
						errorMsg       string
						balancesBefore string
						balancesAfter  string
					}{
						{
							description:    "regular",
							input:          `[{"amount":"2","denom":"denom"}]`,
							isSuccuss:      true,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5002denom",
						},
						{
							description:    "multi-coin",
							input:          `[{"amount":"1","denom":"assaf"},{"amount":"1","denom":"denom"}]`,
							isSuccuss:      true,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5001assaf,5001denom",
						},
						{
							description:    "zero",
							input:          `[{"amount":"0","denom":"denom"}]`,
							isSuccuss:      false,
							errorMsg:       "encrypted: dispatch: submessages: 0denom: invalid coins",
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5000denom",
						},
						{
							description:    "insufficient funds",
							input:          `[{"amount":"3","denom":"denom"}]`,
							isSuccuss:      false,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5000denom",
							errorMsg:       "encrypted: dispatch: submessages: 2denom is smaller than 3denom: insufficient funds",
						},
						{
							description:    "non-existing denom",
							input:          `[{"amount":"1","denom":"blabla"}]`,
							isSuccuss:      false,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5000denom",
							errorMsg:       "encrypted: dispatch: submessages: 0blabla is smaller than 1blabla: insufficient funds",
						},
						{
							description:    "none",
							input:          `[]`,
							isSuccuss:      true,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5000denom",
						},
					} {
						t.Run(test.description, func(t *testing.T) {
							ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, contract.WasmFilePath, sdk.NewCoins(sdk.NewInt64Coin("assaf", 5000)))

							walletACoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)
							walletBCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletB)

							require.Equal(t, test.balancesBefore, walletACoinsBefore.String()+" "+walletBCoinsBefore.String())

							var err cosmwasm.StdError
							var contractAddress sdk.AccAddress

							if callType == "init" {
								_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","amount":%s}}`, walletB.String(), test.input), false, contract.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins(sdk.NewInt64Coin("denom", 2), sdk.NewInt64Coin("assaf", 2)))
							} else {
								_, _, contractAddress, _, _ = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, contract.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins(sdk.NewInt64Coin("denom", 2), sdk.NewInt64Coin("assaf", 2)))

								_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","amount":%s}}`, walletB.String(), test.input), false, contract.IsCosmWasmV1, math.MaxUint64, 0)
							}

							if test.isSuccuss {
								require.Empty(t, err)
							} else {
								require.NotEmpty(t, err)
								require.Equal(t, err.Error(), test.errorMsg)
							}

							walletACoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)
							walletBCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletB)

							require.Equal(t, test.balancesAfter, walletACoinsAfter.String()+" "+walletBCoinsAfter.String())
						})
					}
				})
			}
		})
	}
}

func TestWasmMsgStructure(t *testing.T) {
	for _, from := range testContracts {
		t.Run(fmt.Sprintf("from %s", from.CosmWasmVersion), func(t *testing.T) {
			for _, to := range testContracts {
				t.Run(fmt.Sprintf("to %s", to.CosmWasmVersion), func(t *testing.T) {
					for _, firstCallType := range []string{"init", "exec"} {
						t.Run(fmt.Sprintf("first call %s", firstCallType), func(t *testing.T) {
							for _, secondCallType := range []string{"init", "exec"} {
								t.Run(fmt.Sprintf("second call %s", secondCallType), func(t *testing.T) {
									for _, test := range []struct {
										description      string
										msg              string
										isSuccess        bool
										isErrorEncrypted bool
										expectedError    string
									}{
										{
											description:      "Send invalid input",
											msg:              `{\"blabla\":{}}`,
											isSuccess:        false,
											isErrorEncrypted: true,
											expectedError:    "unknown variant",
										},
										{
											description:      "Success",
											msg:              `{\"wasm_msg\":{\"ty\": \"success\"}}`,
											isSuccess:        true,
											isErrorEncrypted: true,
											expectedError:    "",
										},
										{
											description:      "StdError",
											msg:              `{\"wasm_msg\":{\"ty\": \"err\"}}`,
											isSuccess:        false,
											isErrorEncrypted: true,
											expectedError:    "custom error",
										},
										{
											description:      "Panic",
											msg:              `{\"wasm_msg\":{\"ty\": \"panic\"}}`,
											isSuccess:        false,
											isErrorEncrypted: false,
											expectedError:    "panicked",
										},
									} {
										t.Run(test.description, func(t *testing.T) {
											ctx, keeper, fromCodeID, _, walletA, privKeyA, _, _ := setupTest(t, from.WasmFilePath, sdk.NewCoins())

											wasmCode, err := ioutil.ReadFile(to.WasmFilePath)
											require.NoError(t, err)

											toCodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
											toCodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, toCodeID).CodeHash)
											require.NoError(t, err)

											toAddress := sdk.AccAddress{}
											if secondCallType != "init" {
												_, _, toAddress, _, err = initHelper(t, keeper, ctx, toCodeID, walletA, privKeyA, `{"nop":{}}`, true, to.IsCosmWasmV1, defaultGasForTests)
												require.Empty(t, err)
											}

											fromAddress := sdk.AccAddress{}
											if firstCallType == "init" {
												_, _, _, _, err = initHelper(t, keeper, ctx, fromCodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_%s":{"code_id": %d, "addr": "%s", "code_hash": "%s", "label": "%s", "msg": "%s"}}`, secondCallType, toCodeID, toAddress, toCodeHash, "blabla", test.msg), test.isErrorEncrypted, true, defaultGasForTests)
											} else if firstCallType == "exec" {
												_, _, fromAddress, _, err = initHelper(t, keeper, ctx, fromCodeID, walletA, privKeyA, `{"nop":{}}`, true, from.IsCosmWasmV1, defaultGasForTests)
												require.Empty(t, err)
												_, _, _, _, _, err = execHelper(t, keeper, ctx, fromAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_%s":{"code_id": %d, "addr": "%s", "code_hash": "%s", "label": "%s", "msg": "%s"}}`, secondCallType, toCodeID, toAddress, toCodeHash, "blabla", test.msg), test.isErrorEncrypted, true, math.MaxUint64, 0)
											}

											if test.isSuccess {
												require.Empty(t, err)
											} else {
												require.NotEmpty(t, err)
												require.Contains(t, fmt.Sprintf("%+v", err), test.expectedError)
											}
										})
									}
								})
							}
						})
					}
				})
			}
		})
	}
}

func TestBankMsgBurn(t *testing.T) {
	t.Run("v1", func(t *testing.T) {
		for _, callType := range []string{"init", "exec"} {
			t.Run(callType, func(t *testing.T) {
				for _, test := range []struct {
					description string
					sentFunds   sdk.Coins
				}{
					{
						description: "try to burn coins it has",
						sentFunds:   sdk.NewCoins(sdk.NewInt64Coin("denom", 1)),
					},
					{
						description: "try to burn coins it doesnt have",
						sentFunds:   sdk.NewCoins(),
					},
				} {
					t.Run(test.description, func(t *testing.T) {
						ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

						var err cosmwasm.StdError
						var contractAddress sdk.AccAddress

						if callType == "init" {
							_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_burn":{"amount":[{"amount":"1","denom":"denom"}]}}`), false, false, defaultGasForTests, -1, test.sentFunds)
						} else {
							_, _, contractAddress, _, _ = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, false, defaultGasForTests, -1, test.sentFunds)

							_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_burn":{"amount":[{"amount":"1","denom":"denom"}]}}`), false, false, math.MaxUint64, 0)
						}

						require.NotEmpty(t, err)
						require.Contains(t, err.Error(), "Unknown variant of Bank: invalid CosmosMsg from the contract")
					})
				}
			})
		}
	})
}

func TestCosmosMsgCustom(t *testing.T) {
	for _, contract := range testContracts {
		t.Run(contract.CosmWasmVersion, func(t *testing.T) {
			for _, callType := range []string{"init", "exec"} {
				t.Run(callType, func(t *testing.T) {
					ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, contract.WasmFilePath, sdk.NewCoins())

					var err cosmwasm.StdError
					var contractAddress sdk.AccAddress

					if callType == "init" {
						_, _, contractAddress, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"cosmos_msg_custom":{}}`), false, contract.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins())
					} else {
						_, _, contractAddress, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, contract.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins())

						_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"cosmos_msg_custom":{}}`), false, contract.IsCosmWasmV1, math.MaxUint64, 0)
					}

					require.NotEmpty(t, err)
					// if contract.IsCosmWasmV1 {
					require.Contains(t, err.Error(), "Custom variant not supported: invalid CosmosMsg from the contract")
					// } else {
					// 	require.Equal(t, err.Error(), "Custom variant not supported: invalid CosmosMsg from the contract")

					// }
				})
			}
		})
	}
}

func TestV1SendsFundsWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, true, defaultGasForTests, 200)
	require.Empty(t, err)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"send_funds_with_reply":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
}

func TestV1SendsFundsWithErrorWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)

	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"send_funds_with_error_with_reply":{}}`, false, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)
	require.Contains(t, fmt.Sprintf("%+v", err), "an sdk error occoured while sending a sub-message")
}

func TestV1ReplyOnMultipleSubmessages(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"multiple_sub_messages":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(102), binary.BigEndian.Uint32(data))
}

func TestV1MultipleSubmessagesNoReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)

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

func TestV1InitV010ContractWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	msg := fmt.Sprintf(`{"init_v10":{"counter":80, "code_id":%d, "code_hash":"%s"}}`, v010CodeID, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	accAddress, err := sdk.AccAddressFromBech32(string(data))
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, accAddress, `{"get_count_from_v1":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(80), resp.Get.Count)
}

func TestV1ExecuteV010ContractWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"exec_v10":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(20), binary.BigEndian.Uint32(data))
}

func TestV1InitV010ContractNoReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	msg := fmt.Sprintf(`{"init_v10_no_reply":{"counter":180, "code_id":%d, "code_hash":"%s"}}`, v010CodeID, v010CodeHash)

	_, _, _, ev, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	accAddress, err := sdk.AccAddressFromBech32(ev[1][0].Value)
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, accAddress, `{"get_count_from_v1":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(180), resp.Get.Count)
}

func TestV1ExecuteV010ContractNoReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"exec_v10_no_reply":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	queryRes, qErr := queryHelper(t, keeper, ctx, v010ContractAddress, `{"get_count_from_v1":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(20), resp.Get.Count)
}

func TestV1QueryV010Contract(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"init_from_v1":{"counter":190}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"query_v10":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(190), binary.BigEndian.Uint32(data))
}

func TestV1InitV010ContractWithReplyWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	msg := fmt.Sprintf(`{"init_v10_with_error":{"code_id":%d, "code_hash":"%s"}}`, v010CodeID, v010CodeHash)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
}

func TestV1ExecuteV010ContractWithReplyWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"exec_v10_with_error":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
}

func TestV1InitV010ContractNoReplyWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	msg := fmt.Sprintf(`{"init_v10_no_reply_with_error":{"code_id":%d, "code_hash":"%s"}}`, v010CodeID, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)
	require.Nil(t, data)
}

func TestV1ExecuteV010ContractNoReplyWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"exec_v10_no_reply_with_error":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)
	require.Nil(t, data)
}

func TestV1QueryV010ContractWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"query_v10_with_error":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)
	require.Nil(t, data)
}

func TestV010InitV1ContractFromInitWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, initEvents, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, true, defaultGasForTests)
	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "10")

	require.Empty(t, err)
	accAddress, err := sdk.AccAddressFromBech32(initEvents[1][0].Value)
	require.Empty(t, err)

	queryRes, qErr = queryHelper(t, keeper, ctx, accAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "1")
}

func TestV010InitV1ContractFromExecuteWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "10")

	_, _, execData, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, execData)

	accAddress, err := sdk.AccAddressFromBech32(execEvents[1][0].Value)
	require.Empty(t, err)

	queryRes, qErr = queryHelper(t, keeper, ctx, accAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "1")
}

func TestV010ExecuteV1ContractFromInitWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":199, "expires":100}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, _, _, err = initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"increment\":{\"addition\": 1}}`), true, true, defaultGasForTests)
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(200), resp.Get.Count)
}

func TestV010ExecuteV1ContractFromExecuteWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":299, "expires":100}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"increment\":{\"addition\": 1}}`), true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(300), resp.Get.Count)
}

func TestV010QueryV1ContractFromInitWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, events, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"receive_external_query_v1\":{\"num\":1}}`), true, true, defaultGasForTests)
	require.Empty(t, err)
	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "c", Value: "2"},
			},
		},
		events,
	)
}

func TestV010QueryV1ContractFromExecuteWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	_, _, _, events, _, err := execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"receive_external_query_v1\":{\"num\":1}}`), true, true, defaultGasForTests, 0)
	require.Empty(t, err)
	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "c", Value: "2"},
			},
		},
		events,
	)
}

func TestV010InitV1ContractFromInitWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, _, _, err = initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d, "code_hash":"%s","label":"blabla", "msg":"%s"}}`, codeID, codeHash, `{\"counter\":{\"counter\":0, \"expires\":100}}`), true, true, defaultGasForTests)
	require.Contains(t, fmt.Sprintf("%+v", err), "got wrong counter on init")
}

func TestV010InitV1ContractFromExecuteWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "10")

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d, "code_hash":"%s","label":"blabla", "msg":"%s"}}`, codeID, codeHash, `{\"counter\":{\"counter\":0, \"expires\":100}}`), true, true, defaultGasForTests, 0)
	require.Contains(t, fmt.Sprintf("%+v", err), "got wrong counter on init")
}

func TestV010ExecuteV1ContractFromInitWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":199, "expires":100}}`, true, true, defaultGasForTests)
	_, _, _, _, err = initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"increment\":{\"addition\": 0}}`), true, true, defaultGasForTests)

	require.Contains(t, fmt.Sprintf("%+v", err), "got wrong counter on increment")
}

func TestV010ExecuteV1ContractFromExecuteWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":299, "expires":100}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"increment\":{\"addition\": 0}}`), true, true, defaultGasForTests, 0)
	require.Contains(t, fmt.Sprintf("%+v", err), "got wrong counter on increment")
}

func TestV010QueryV1ContractFromInitWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, _, _, err = initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"contract_error\":{\"error_type\":\"generic_err\"}}`), true, true, defaultGasForTests)
	require.Contains(t, fmt.Sprintf("%+v", err), "la la 🤯")
}

func TestV010QueryV1ContractFromExecuteWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"contract_error\":{\"error_type\":\"generic_err\"}}`), true, true, defaultGasForTests, 0)
	require.Contains(t, fmt.Sprintf("%+v", err), "la la 🤯")
}

func TestSendEncryptedAttributesFromInitWithoutSubmessageWithoutReply(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_attributes":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			require.Equal(t,
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

			_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":2200}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	require.Equal(t,
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

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attributes":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, err)

			require.Equal(t,
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

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, err)

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":2200}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	require.Equal(t,
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

			_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_plaintext_attributes":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, true)
			require.Empty(t, err)

			require.Equal(t,
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

			_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, true)
			require.Empty(t, err)

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":2300}}`, true, true, defaultGasForTests, true)
	require.Empty(t, err)

	require.Equal(t,
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

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0, true)
			require.Empty(t, err)

			require.Equal(t,
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

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0, true)
			require.Empty(t, err)

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":2300}}`, true, true, defaultGasForTests, 0, true)
	require.Empty(t, err)

	require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	nonce, ctx, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_events":{}}`, true, true, defaultGasForTests)

	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false
	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			require.Equal(t,
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

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	nonce, ctx, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_events_with_submessage":{"id":0}}`, true, true, defaultGasForTests)
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	nonce, ctx, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_events_with_submessage":{"id":2400}}`, true, true, defaultGasForTests)
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
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

			require.Equal(t,
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

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	nonce, ctx, contractAddress, logs, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_mixed_attributes_and_events":{}}`, true, true, defaultGasForTests, true)

	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, false)
			require.Empty(t, err)

			require.Equal(t,
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

	require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	nonce, ctx, contractAddress, logs, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":0}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber2 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			require.Equal(t,
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

			require.Equal(t,
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

	require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	nonce, ctx, contractAddress, logs, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":2500}}`, true, true, defaultGasForTests)
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

	require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
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

			require.Equal(t,
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

	require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
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

			require.Equal(t,
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

			require.Equal(t,
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

	require.Equal(t,
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
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
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

			require.Equal(t,
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

			require.Equal(t,
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

			require.Equal(t,
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

	require.Equal(t,
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

func TestV1SendsLogsMixedWithV010WithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)
	_, _, v1ContractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, v1ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"add_attributes_from_v010":{"addr":"%s","code_hash":"%s", "id":0}}`, v010ContractAddress, v010CodeHash), true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			require.Equal(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: v1ContractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}
	}

	require.True(t, hadCyber1)

	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "attr3", Value: "🍉"},
				{Key: "attr4", Value: "🥝"},
			},
		},
		logs,
	)
}

func TestV1SendsLogsMixedWithV010WithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	v010CodeHash := hex.EncodeToString(keeper.GetCodeInfo(ctx, v010CodeID).CodeHash)

	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)
	_, _, v1ContractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, v1ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"add_attributes_from_v010":{"addr":"%s","code_hash":"%s", "id":2500}}`, v010ContractAddress, v010CodeHash), true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber3 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			require.Equal(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: v1ContractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber3" {
			require.False(t, hadCyber3)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			require.Equal(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: v1ContractAddress.String()},
					{Key: "attr9", Value: "🤯"},
					{Key: "attr10", Value: "🤟"},
				},
				attrs,
			)

			hadCyber3 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber3)

	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "attr3", Value: "🍉"},
				{Key: "attr4", Value: "🥝"},
			},
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr11", Value: "😉"},
				{Key: "attr12", Value: "😊"},
			},
		},
		logs,
	)
}

func TestV010SendsLogsMixedWithV1(t *testing.T) {
	ctx, keeper, codeID, v1CodeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)
	_, _, v1ContractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"add_mixed_events_and_attributes_from_v1":{"addr":"%s","code_hash":"%s"}}`, v1ContractAddress, v1CodeHash), true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber2 := false

	for _, e := range events {
		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			require.Equal(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: v1ContractAddress.String()},
					{Key: "attr5", Value: "🐙"},
					{Key: "attr6", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}
	}

	require.True(t, hadCyber2)

	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "attr1", Value: "🦄"},
				{Key: "attr2", Value: "🌈"},
			},
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr7", Value: "😅"},
				{Key: "attr8", Value: "🦄"},
			},
		},
		logs,
	)
}

func TestSubmessageGasExceedingMessageGas(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	defer func() {
		r := recover()
		require.NotNil(t, r)
		_, ok := r.(sdk.ErrorOutOfGas)
		require.True(t, ok, "%+v", r)
	}()
	_, _, _, _, _ = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"measure_gas_for_submessage":{"id":0}}`, false, true, defaultGasForTests)
}

func TestReplyGasExceedingMessageGas(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, "./testdata/v1-sanity-contract/contract.wasm", sdk.NewCoins())

	defer func() {
		r := recover()
		require.NotNil(t, r)
		_, ok := r.(sdk.ErrorOutOfGas)
		require.True(t, ok, "%+v", r)
	}()
	_, _, _, _, _ = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"measure_gas_for_submessage":{"id":2600}}`, false, true, defaultGasForTests)
}
