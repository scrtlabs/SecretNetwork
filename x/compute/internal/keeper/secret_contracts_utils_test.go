package keeper

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"os"
	"regexp"
	"strings"
	"testing"

	stypes "github.com/cosmos/cosmos-sdk/store/types"
	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"

	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	v010cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

type TestContract struct {
	CosmWasmVersion string
	IsCosmWasmV1    bool
	WasmFilePath    string
}

var testContracts = []TestContract{
	{
		CosmWasmVersion: "v0.10",
		IsCosmWasmV1:    false,
		WasmFilePath:    TestContractPaths[v010Contract],
	}, {
		CosmWasmVersion: "v1",
		IsCosmWasmV1:    true,
		WasmFilePath:    TestContractPaths[v1Contract],
	},
}

// if codeID isn't 0, it will try to use that. Otherwise will take the contractAddress
func testEncrypt(t *testing.T, keeper Keeper, ctx sdk.Context, contractAddress sdk.AccAddress, codeId uint64, msg []byte) ([]byte, error) {
	var hash []byte
	var err error

	if codeId != 0 {
		codeInfo, err := keeper.GetCodeInfo(ctx, codeId)
		require.NoError(t, err)

		hash = codeInfo.CodeHash
	} else {
		hash, err = keeper.GetContractHash(ctx, contractAddress)
		require.NoError(t, err)

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

func uploadCode(ctx sdk.Context, t *testing.T, keeper Keeper, wasmPath string, walletA sdk.AccAddress) (uint64, string) {
	wasmCode, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, codeID)
	require.NoError(t, err)

	codeHash := hex.EncodeToString(codeInfo.CodeHash)

	return codeID, codeHash
}

func uploadChainCode(ctx sdk.Context, t *testing.T, keeper Keeper, wasmPath string, walletA sdk.AccAddress, bytesCount uint64) (uint64, string) {
	wasmCode, err := os.ReadFile(wasmPath)

	toBeReplaced := "Gas submessage"
	replaceBy := strings.Replace(toBeReplaced, "G", string(byte(bytesCount)), 1)
	wasmCode = []byte(strings.Replace(string(wasmCode), toBeReplaced, replaceBy, 1))

	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, codeID)
	require.NoError(t, err)

	codeHash := hex.EncodeToString(codeInfo.CodeHash)

	return codeID, codeHash
}

func setupBasicTest(t *testing.T, additionalCoinsInWallets sdk.Coins) (sdk.Context, Keeper, sdk.AccAddress, crypto.PrivKey, sdk.AccAddress, crypto.PrivKey) {
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

	return ctx, keeper, walletA, privKeyA, walletB, privKeyB
}

func setupTest(t *testing.T, wasmPath string, additionalCoinsInWallets sdk.Coins) (sdk.Context, Keeper, uint64, string, sdk.AccAddress, crypto.PrivKey, sdk.AccAddress, crypto.PrivKey) {
	ctx, keeper, walletA, privKeyA, walletB, privKeyB := setupBasicTest(t, additionalCoinsInWallets)

	codeID, codeHash := uploadCode(ctx, t, keeper, wasmPath, walletA)

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
		return v010cosmwasm.LogAttribute{}, fmt.Errorf("Failed Decrypt for key %+v", keyCipherBz)
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

const defaultGasForTests uint64 = 500_000

// wrap the default gas meter with a counter of wasm calls
// in order to verify that every wasm call consumes gas
type WasmCounterGasMeter struct {
	wasmCounter uint64
	gasMeter    sdk.GasMeter
}

func (wasmGasMeter *WasmCounterGasMeter) RefundGas(_ stypes.Gas, _ string) {}

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
	hash, err := keeper.GetContractHash(ctx, contractAddr)
	require.NoError(t, err)

	hashStr := hex.EncodeToString(hash)

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

//func execHelperImpl(
//	t *testing.T, keeper Keeper, ctx sdk.Context,
//	contractAddress sdk.AccAddress, txSender sdk.AccAddress, senderPrivKey crypto.PrivKey, execMsg string,
//	isErrorEncrypted bool, isV1Contract bool, gas uint64, coin int64, wasmCallCount int64, shouldSkipAttributes ...bool,
//) ([]byte, sdk.Context, []byte, []ContractEvent, uint64, cosmwasm.StdError) {
//	return execHelperMultipleCoinsImpl(t, keeper, ctx, contractAddress, txSender, senderPrivKey, execMsg, isErrorEncrypted, isV1Contract, gas, sdk.NewCoins(sdk.NewInt64Coin("denom", coin)), wasmCallCount, shouldSkipAttributes...)
//}

func execHelperCustomWasmCount(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddress sdk.AccAddress, txSender sdk.AccAddress, senderPrivKey crypto.PrivKey, execMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, coin int64, wasmCallCount int64, shouldSkipAttributes ...bool,
) ([]byte, sdk.Context, []byte, []ContractEvent, uint64, cosmwasm.StdError) {
	results, err := execTxBuilderImpl(t, keeper, ctx, contractAddress, txSender, senderPrivKey, []string{execMsg}, isErrorEncrypted, isV1Contract, gas, sdk.NewCoins(sdk.NewCoin("denom", sdk.NewInt(coin))), wasmCallCount, shouldSkipAttributes...)

	if len(results) != 1 {
		panic("Single msg test somehow returned multiple results")
	}

	if err != nil {
		return results[0].Nonce, results[0].Ctx, results[0].Data, results[0].WasmEvents, results[0].GasUsed, *err.CosmWasm
	}
	return results[0].Nonce, results[0].Ctx, results[0].Data, results[0].WasmEvents, results[0].GasUsed, cosmwasm.StdError{}
	// todo: lol refactor tests to use the struct

}

func execHelperMultipleCoins(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddress sdk.AccAddress, txSender sdk.AccAddress, senderPrivKey crypto.PrivKey, execMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, coins sdk.Coins, wasmCount int64, shouldSkipAttributes ...bool,
) ([]byte, sdk.Context, []byte, []ContractEvent, uint64, cosmwasm.StdError) {
	results, err := execTxBuilderImpl(t, keeper, ctx, contractAddress, txSender, senderPrivKey, []string{execMsg}, isErrorEncrypted, isV1Contract, gas, coins, wasmCount, shouldSkipAttributes...)

	if len(results) != 1 {
		panic("Single msg test somehow returned multiple results")
	}

	// todo: lol refactor tests to use the struct
	if err != nil {
		return results[0].Nonce, results[0].Ctx, results[0].Data, results[0].WasmEvents, results[0].GasUsed, *err.CosmWasm
	}
	return results[0].Nonce, results[0].Ctx, results[0].Data, results[0].WasmEvents, results[0].GasUsed, cosmwasm.StdError{}
}

func execHelper(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddress sdk.AccAddress, txSender sdk.AccAddress, senderPrivKey crypto.PrivKey, execMsg string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, coin int64, shouldSkipAttributes ...bool,
) ([]byte, sdk.Context, []byte, []ContractEvent, uint64, cosmwasm.StdError) {
	results, err := execTxBuilderImpl(t, keeper, ctx, contractAddress, txSender, senderPrivKey, []string{execMsg}, isErrorEncrypted, isV1Contract, gas, sdk.NewCoins(sdk.NewInt64Coin("denom", coin)), -1, shouldSkipAttributes...)

	if len(results) != 1 {
		panic(fmt.Sprintf("Single msg test somehow returned multiple results: %d", len(results)))
	}

	if err != nil {
		return results[0].Nonce, results[0].Ctx, results[0].Data, results[0].WasmEvents, results[0].GasUsed, *err.CosmWasm
	}
	return results[0].Nonce, results[0].Ctx, results[0].Data, results[0].WasmEvents, results[0].GasUsed, cosmwasm.StdError{}
}

func execHelperMultipleMsgs(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddress sdk.AccAddress, txSender sdk.AccAddress, senderPrivKey crypto.PrivKey, execMsg []string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, coin int64, shouldSkipAttributes ...bool,
) ([]ExecResult, *ErrorResult) {
	return execTxBuilderImpl(t, keeper, ctx, contractAddress, txSender, senderPrivKey, execMsg, isErrorEncrypted, isV1Contract, gas, sdk.NewCoins(sdk.NewInt64Coin("denom", coin)), -1, shouldSkipAttributes...)
}

func execTxBuilderImpl(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	contractAddress sdk.AccAddress, txSender sdk.AccAddress, senderPrivKey crypto.PrivKey, execMsgs []string,
	isErrorEncrypted bool, isV1Contract bool, gas uint64, coins sdk.Coins, wasmCallCount int64, shouldSkipAttributes ...bool,
) ([]ExecResult, *ErrorResult) {
	hash, err := keeper.GetContractHash(ctx, contractAddress)
	require.NoError(t, err)

	hashStr := hex.EncodeToString(hash)

	var secretMsgs []types.SecretMsg
	for _, execMsg := range execMsgs {
		secretMsg := types.SecretMsg{
			CodeHash: []byte(hashStr),
			Msg:      []byte(execMsg),
		}
		secretMsgs = append(secretMsgs, secretMsg)
	}

	var secretMsgsBz [][]byte
	for _, msg := range secretMsgs {
		execMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
		require.NoError(t, err)

		secretMsgsBz = append(secretMsgsBz, execMsgBz)
		// nonce := execMsgBz[0:32]
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

	ctx = PrepareExecSignedTxWithMultipleMsgs(t, keeper, ctx, txSender, senderPrivKey, secretMsgsBz, contractAddress, coins)

	// reset value before test
	keeper.LastMsgManager.SetMarker(false)

	var results []ExecResult
	for _, msg := range secretMsgsBz {

		// simulate the check in baseapp
		if keeper.LastMsgManager.GetMarker() {
			errResult := ErrorResult{
				Generic: sdkerrors.Wrap(sdkerrors.ErrLastTx, "Error"),
			}
			return results, &errResult
		}

		nonce := msg[0:32]

		gasBefore := ctx.GasMeter().GasConsumed()
		execResult, err := keeper.Execute(ctx, contractAddress, txSender, msg, coins, nil)
		gasAfter := ctx.GasMeter().GasConsumed()
		gasUsed := gasAfter - gasBefore

		if wasmCallCount < 0 {
			// default, just check that at least 1 call happened
			require.NotZero(t, gasMeter.GetWasmCounter(), err)
		} else {
			require.Equal(t, uint64(wasmCallCount), gasMeter.GetWasmCounter(), err)
		}

		if err != nil {
			results = append(results, ExecResult{
				Nonce:      nil,
				Ctx:        ctx,
				Data:       nil,
				WasmEvents: nil,
				GasUsed:    gasUsed,
			})

			errResult := ErrorResult{
				Generic: err,
			}
			cwErr := extractInnerError(t, err, nonce, isErrorEncrypted, isV1Contract)
			errResult.CosmWasm = &cwErr

			return results, &errResult
		}

		// wasmEvents comes from all the callbacks as well
		wasmEvents := tryDecryptWasmEvents(ctx, nonce, shouldSkipAttributes...)

		// Data is the output of only the first call
		data := getDecryptedData(t, execResult.Data, nonce)

		results = append(results, ExecResult{
			Nonce:      nonce,
			Ctx:        ctx,
			Data:       data,
			WasmEvents: wasmEvents,
			GasUsed:    gasUsed,
		})
	}

	return results, nil
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
	codeInfo, err := keeper.GetCodeInfo(ctx, codeID)
	require.NoError(t, err)

	hashStr := hex.EncodeToString(codeInfo.CodeHash)

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

// requireEvents checks events output
// Events are now sorted (since wasmd v0.28),
// but for us they're sorted while encrypted so when testing with random encryption keys
// this is non-deterministic
func requireEvents(t *testing.T, a, b []ContractEvent) {
	require.Equal(t, len(a), len(b))

	for i := range b {
		require.Equal(t, len(a[i]), len(b[i]))
		for j := range b[i] {
			require.Contains(t, a[i], b[i][j])
		}
	}
}

// requireEventsInclude checks that "b" "a" contains the log attributes specified in the respective  events,
// but "a"'s events may have additional log attributes that are not specified
func requireEventsInclude(t *testing.T, a, b []ContractEvent) {
	require.Equal(t, len(a), len(b))

	for i := range b {
		for j := range b[i] {
			require.Contains(t, a[i], b[i][j])
		}
	}
}

// requireLogAttributes checks events output
// Events are now sorted (since wasmd v0.28),
// but for us they're sorted while encrypted so when testing with random encryption keys
// this is non-deterministic
func requireLogAttributes(t *testing.T, a, b []v010cosmwasm.LogAttribute) {
	require.Equal(t, len(a), len(b))

	for i := range b {
		require.Contains(t, a, b[i])
	}
}

type GetResponse struct {
	Count uint32 `json:"count"`
}
type v1QueryResponse struct {
	Get GetResponse `json:"get"`
}
