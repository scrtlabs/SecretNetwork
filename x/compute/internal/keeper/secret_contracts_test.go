package keeper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	cosmwasm "github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/types"

	"github.com/stretchr/testify/require"
)

// getDecryptedWasmEvents gets all "wasm" events and decrypt what's necessary
// Returns all "wasm" events, including from contract callbacks
func getDecryptedWasmEvents(t *testing.T, ctx *sdk.Context, nonce []byte) []cosmwasm.LogAttribute {
	events := ctx.EventManager().Events()
	var res []cosmwasm.LogAttribute
	for _, e := range events {
		if e.Type == "wasm" {
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

				res = append(res, newLog)
			}
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

	// data
	dataCiphertextBz, err := base64.StdEncoding.DecodeString(res.Ok.Data)
	require.NoError(t, err)
	dataPlaintext, err := wasmCtx.Decrypt(dataCiphertextBz, nonce)
	require.NoError(t, err)

	res.Ok.Data = string(dataPlaintext)

	// logs
	for i, log := range res.Ok.Log {
		// key
		keyCipherBz, err := base64.StdEncoding.DecodeString(log.Key)
		require.NoError(t, err)
		keyPlainBz, err := wasmCtx.Decrypt(keyCipherBz, nonce)
		require.NoError(t, err)
		log.Key = string(keyPlainBz)

		// value
		valueCipherBz, err := base64.StdEncoding.DecodeString(log.Value)
		require.NoError(t, err)
		valuePlainBz, err := wasmCtx.Decrypt(valueCipherBz, nonce)
		require.NoError(t, err)
		log.Value = string(valuePlainBz)

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

func TestCallbackSanity(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)
	creator := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	fred := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	wasmCode, err := ioutil.ReadFile("./testdata/test-contract/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	initMsg := InitMsg{}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	initMsgBz, err = wasmCtx.Encrypt(initMsgBz)
	require.NoError(t, err)

	contractAddress, err := keeper.Instantiate(ctx, contractID, creator, initMsgBz, "demo contract 5", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	execMsg := fmt.Sprintf(`{"a":{"contract_addr":"%s","x":2,"y":3}}`, contractAddress.String())
	execMsgBz, err := wasmCtx.Encrypt([]byte(execMsg))
	require.NoError(t, err)

	// let's make sure we get a reasonable error, no panic/crash
	execResult, err := keeper.Execute(ctx, contractAddress, fred, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	nonce := execMsgBz[0:32]

	// Events is from all callbacks
	wasmEvents := getDecryptedWasmEvents(t, &ctx, nonce)
	require.Equal(t, 6, len(wasmEvents))
	require.Equal(t,
		[]cosmwasm.LogAttribute{
			{Key: "contract_address", Value: contractAddress.String()},
			{Key: "action", Value: "banana"},
			{Key: "contract_address", Value: contractAddress.String()},
			{Key: "action", Value: "papaya"},
			{Key: "contract_address", Value: contractAddress.String()},
			{Key: "action", Value: "watermelon"},
		},
		wasmEvents,
	)

	// Data is the output of only the first call
	data := getDecryptedData(t, execResult.Data, nonce)

	require.Empty(t, data.Err)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte{2, 3}), data.Ok.Data)
	require.Equal(t, []cosmwasm.LogAttribute{{Key: "action", Value: "banana"}}, data.Ok.Log)
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
	creator := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	fred := createFakeFundedAccount(ctx, accKeeper, topUp)

	// https://github.com/CosmWasm/cosmwasm-examples/blob/f5ea00a85247abae8f8cbcba301f94ef21c66087/erc20/src/contract.rs
	wasmCode, err := ioutil.ReadFile("./testdata/erc20-f5ea00a85247abae8f8cbcba301f94ef21c66087.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, creator.String(), fred.String())

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)

	contractAddr, err := keeper.Instantiate(ctx, contractID, creator, initMsgBz, "demo contract", deposit)
	require.NoError(t, err)

	// check state after init
	requireQueryResult(t,
		keeper, ctx, contractAddr,
		fmt.Sprintf(`{"balance":{"address":"%s"}}`, creator.String()),
		`{"balance":"108"}`,
	)
	requireQueryResult(t,
		keeper, ctx, contractAddr,
		fmt.Sprintf(`{"balance":{"address":"%s"}}`, fred.String()),
		`{"balance":"53"}`,
	)
}
