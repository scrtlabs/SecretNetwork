package keeper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/types"
	"github.com/stretchr/testify/require"
)

// filterMessageEvents returns the same events with all of type == EventTypeMessage removed.
// this is so only our top-level message event comes through
func getDecryptedWasmEvents(t *testing.T, ctx *sdk.Context, nonce []byte) sdk.Events {
	events := ctx.EventManager().Events()
	var res []sdk.Event
	for _, e := range events {
		if e.Type == "wasm" {
			for i, a := range e.Attributes {
				key := string(a.Key)
				value := string(a.Value)
				if key != "contract_address" {
					// key
					keyCiphertext, err := base64.StdEncoding.DecodeString(key)
					require.NoError(t, err)

					keyPlaintext, err := wasmCtx.Decrypt(keyCiphertext, nonce)
					require.NoError(t, err)

					a.Key = keyPlaintext

					// value
					valueCiphertext, err := base64.StdEncoding.DecodeString(value)
					require.NoError(t, err)

					valuePlaintext, err := wasmCtx.Decrypt(valueCiphertext, nonce)
					require.NoError(t, err)

					a.Value = valuePlaintext

					// override in parent
					e.Attributes[i] = a
				}
			}

			res = append(res, e)
		}
	}
	return res
}

func decryptDataJSON(t *testing.T, resp *types.CosmosResponse, nonce []byte) {
	// data
	dataCiphertext, err := base64.StdEncoding.DecodeString(resp.Ok.Data)
	require.NoError(t, err)

	dataPlaintext, err := wasmCtx.Decrypt(dataCiphertext, nonce)
	require.NoError(t, err)

	resp.Ok.Data = string(dataPlaintext)

	// logs
	for i, l := range resp.Ok.Log {
		// key
		keyCiphertext, err := base64.StdEncoding.DecodeString(l.Key)
		require.NoError(t, err)

		keyPlaintext, err := wasmCtx.Decrypt(keyCiphertext, nonce)
		require.NoError(t, err)

		l.Key = string(keyPlaintext)

		// value
		valueCiphertext, err := base64.StdEncoding.DecodeString(l.Value)
		require.NoError(t, err)

		valuePlaintext, err := wasmCtx.Decrypt(valueCiphertext, nonce)
		require.NoError(t, err)

		l.Value = string(valuePlaintext)

		resp.Ok.Log[i] = l
	}

	// messages
	for i, m := range resp.Ok.Messages {
		msgPlaintext, err := wasmCtx.Decrypt(m.Contract.Msg[64:], nonce)
		require.NoError(t, err)

		m.Contract.Msg = msgPlaintext

		resp.Ok.Messages[i] = m
	}
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

	addr, err := keeper.Instantiate(ctx, contractID, creator, initMsgBz, "demo contract 5", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	execMsg := fmt.Sprintf(`{"a":{"contract_addr":"%s","x":2,"y":3}}`, addr.String())
	execMsgBz, err := wasmCtx.Encrypt([]byte(execMsg))
	require.NoError(t, err)

	// let's make sure we get a reasonable error, no panic/crash
	execResult, err := keeper.Execute(ctx, addr, fred, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	require.NoError(t, err)

	nonce := execMsgBz[:32]

	// Getting wasm Events + get Data json
	resEvents := getDecryptedWasmEvents(t, &ctx, nonce)
	var resDataJSON types.CosmosResponse
	err = json.Unmarshal(execResult.Data, &resDataJSON)
	require.NoError(t, err)

	// Decrypt Datat json
	decryptDataJSON(t, &resDataJSON, nonce)
	fmt.Println(resEvents)

	// TODO iterate over res.Events and decrypt with nonce to verify banana, papaya, watermelon
	// TODO hex.DecodeString(res.Data) -> json.Marshal -> {data: ... , messages: ..., log: [{key:, value:},{key:,value:}]}
	// output decryption example: https://github.com/enigmampc/SecretNetwork/blob/bedbe10e2e08bedc800f3ff6dac019824da18bd7/x/compute/client/cli/query.go#L327

	require.NoError(t, err)
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
	query := fmt.Sprintf(`{"balance":{"address":"%s"}}`, creator.String())
	queryBz, err := wasmCtx.Encrypt([]byte(query))
	require.NoError(t, err)

	queryRes, err := keeper.QuerySmart(ctx, contractAddr, queryBz)
	require.NoError(t, err)

	nonce := queryBz[:32]
	resDecrypted, err := wasmCtx.Decrypt(queryRes, nonce)
	require.NoError(t, err)

	decodedResp, err := base64.StdEncoding.DecodeString(string(resDecrypted))
	require.NoError(t, err)
	x := string(decodedResp)
	fmt.Println(x)
}
