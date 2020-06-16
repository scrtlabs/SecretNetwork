package keeper

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestSanity(t *testing.T) {
	// Assaf: maybe port the bash sanity test and the callback test into here?

	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	fred := createFakeFundedAccount(ctx, accKeeper, topUp)

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	_, _, bob := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	initMsgBz, err = wasmCtx.Encrypt(initMsgBz)
	require.NoError(t, err)

	addr, err := keeper.Instantiate(ctx, contractID, creator, initMsgBz, "demo contract 4", deposit)
	require.NoError(t, err)

	execMsgBz, err := wasmCtx.Encrypt([]byte(`{"panic":{}}`))
	require.NoError(t, err)

	// let's make sure we get a reasonable error, no panic/crash
	_, err = keeper.Execute(ctx, addr, fred, execMsgBz, topUp)
	require.Error(t, err)
}
