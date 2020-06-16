package keeper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

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
	res, err := keeper.Execute(ctx, addr, fred, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
	nonce := execMsgBz[:32]
	resDecrypted, err := wasmCtx.Decrypt(res, nonce)
	require.NoError(t, err)

	// TODO iterate over res.Events and decrypt with nonce to verify banana, papaya, watermelon
	// TODO hex.DecodeString(res.Data) -> json.Marshal -> {data: ... , messages: ..., log: [{key:, value:},{key:,value:}]}
	// output decryption example: https://github.com/enigmampc/SecretNetwork/blob/bedbe10e2e08bedc800f3ff6dac019824da18bd7/x/compute/client/cli/query.go#L327

	fmt.Println(resDecrypted)

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
