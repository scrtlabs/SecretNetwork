package keeper

import (
	"fmt"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	"github.com/enigmampc/cosmos-sdk/x/auth"
	"github.com/enigmampc/cosmos-sdk/x/auth/exported"
	authtypes "github.com/enigmampc/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/multisig"
	"os"
	"testing"
)

func multisigTxCreator(t *testing.T, ctx *sdk.Context, keeper Keeper, n int, threshold int, actualSigners int, sdkMsg sdk.Msg) sdk.AccAddress {
	privKeys, pubKeys, multisigPubKey := generateMultisigAddr(n, threshold, *ctx, keeper)

	switch msg := sdkMsg.(type) {
	case types.MsgInstantiateContract:
		msg.Sender = sdk.AccAddress(multisigPubKey.Address())
	case types.MsgExecuteContract:
		msg.Sender = sdk.AccAddress(multisigPubKey.Address())
	}

	tx := authtypes.StdTx{
		Msgs:       []sdk.Msg{sdkMsg},
		Fee:        authtypes.StdFee{},
		Signatures: []authtypes.StdSignature{},
		Memo:       "",
	}

	multiSignature := generateSignatures(t, *ctx, keeper, privKeys, pubKeys, multisigPubKey.Address().Bytes(), tx, actualSigners)

	stdSig := authtypes.StdSignature{
		PubKey:    multisigPubKey,
		Signature: multiSignature.Marshal(),
	}

	tx.Signatures = []authtypes.StdSignature{stdSig}
	txBytes, err := keeper.cdc.MarshalBinaryLengthPrefixed(tx)
	require.NoError(t, err)

	*ctx = ctx.WithTxBytes(txBytes)

	return sdk.AccAddress(multisigPubKey.Address())
}

// GetSignBytes returns the signBytes of the tx for a given signer
// This is a copy of cosmos-sdk function (cosmos-sdk/x/auth/types/StdTx.GetSignBytes()
// This is because the original `GetSignBytes` was probably meant to be used before the transaction gets processed, and the
// sequence that gets returned is an increment of what we need.
// This is why we use `acc.GetSequence() - 1`
func getSignBytes(ctx sdk.Context, acc exported.Account, tx auth.StdTx) []byte {
	genesis := ctx.BlockHeight() == 0
	chainID := ctx.ChainID()
	var accNum uint64
	if !genesis {
		accNum = acc.GetAccountNumber()
	}

	return authtypes.StdSignBytes(
		chainID, accNum, acc.GetSequence()-1, tx.Fee, tx.Msgs, tx.Memo,
	)
}

func generateSignatures(t *testing.T, ctx sdk.Context, keeper Keeper, privKeys []crypto.PrivKey, pubKeys []crypto.PubKey, accAddress sdk.AccAddress, tx authtypes.StdTx, actualSigners int) *multisig.Multisignature {
	multisigAcc := keeper.accountKeeper.GetAccount(ctx, accAddress)
	signBytes := getSignBytes(ctx, multisigAcc, tx)
	multiSig := multisig.NewMultisig(len(privKeys))

	var signDoc authtypes.StdSignDoc
	keeper.cdc.MustUnmarshalJSON(signBytes, &signDoc)
	fmt.Printf("Sign Doc is %+v\n", signDoc)
	fmt.Printf("Sign Bytes is %v\n", signBytes)

	for i := 0; i < actualSigners; i++ {
		signature, _ := privKeys[i].Sign(signBytes)

		fmt.Printf("Signature %d  is %v\n", i, signature)
		fmt.Printf("Signer is %v\n", pubKeys[i].Bytes())

		err := multiSig.AddSignatureFromPubKey(signature, pubKeys[i], pubKeys)
		require.NoError(t, err)
	}

	fmt.Printf("Multisig is %v\n", multiSig)

	return multiSig
}

func generateMultisigAddr(n int, threshold int, ctx sdk.Context, keeper Keeper) ([]crypto.PrivKey, []crypto.PubKey, multisig.PubKeyMultisigThreshold) {
	privkeys := make([]crypto.PrivKey, n)
	pubkeys := make([]crypto.PubKey, n)

	for i := 0; i < n; i++ {
		deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
		_, privKey := createFakeFundedAccount(ctx, keeper.accountKeeper, deposit.Add(deposit...))
		privkeys[i] = privKey
		pubkeys[i] = privKey.PubKey()
	}

	multisigPubkey, _ := multisig.NewPubKeyMultisigThreshold(threshold, pubkeys).(multisig.PubKeyMultisigThreshold)

	// Register to keeper
	addr := multisigPubkey.Address().Bytes()
	baseAcct := auth.NewBaseAccountWithAddress(addr)
	coins := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	_ = baseAcct.SetCoins(coins)
	_ = baseAcct.SetPubKey(multisigPubkey)
	keeper.accountKeeper.SetAccount(ctx, &baseAcct)

	return privkeys, pubkeys, multisigPubkey
}

func prepareInitSignedTxMultipleMsgs(t *testing.T, keeper Keeper, ctx sdk.Context, creators []sdk.AccAddress, privKeys []crypto.PrivKey, initMsgs []sdk.Msg, codeID uint64) sdk.Context {
	accountNums := make([]uint64, len(creators))
	accountSeqs := make([]uint64, len(creators))
	for i, acc := range creators {
		account, err := auth.GetSignerAcc(ctx, keeper.accountKeeper, acc)
		require.NoError(t, err)

		accountNums[i] = account.GetAccountNumber()
		accountSeqs[i] = account.GetSequence() - 1
	}

	tx := authtypes.NewTestTx(ctx, initMsgs, privKeys, accountNums, accountSeqs, authtypes.StdFee{
		Amount: nil,
		Gas:    0,
	})

	txBytes, err := keeper.cdc.MarshalBinaryLengthPrefixed(tx)
	require.NoError(t, err)

	return ctx.WithTxBytes(txBytes)
}

func TestMultipleSigners(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, privKeyA, walletB, privKeyB := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := `{"nop":{}}`

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsgA := types.MsgInstantiateContract{
		Sender:    walletA,
		Admin:     nil,
		Code:      codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}
	sdkMsgB := types.MsgInstantiateContract{
		Sender:    walletB,
		Admin:     nil,
		Code:      codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}

	ctx = prepareInitSignedTxMultipleMsgs(t, keeper, ctx, []sdk.AccAddress{walletA, walletB}, []crypto.PrivKey{privKeyA, privKeyB}, []sdk.Msg{sdkMsgA, sdkMsgB}, codeID)

	contractAddressA, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, true)
	}
	require.NoError(t, err)

	wasmEvents := getDecryptedWasmEvents(t, ctx, nonce)

	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddressA.String()},
				{Key: "init", Value: "🌈"},
			},
		},
		wasmEvents,
	)

	contractAddressB, err := keeper.Instantiate(ctx, codeID, walletB, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, true)
	}
	require.NoError(t, err)

	wasmEvents = getDecryptedWasmEvents(t, ctx, nonce)

	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddressA.String()},
				{Key: "init", Value: "🌈"},
			},
			{
				{Key: "contract_address", Value: contractAddressB.String()},
				{Key: "init", Value: "🌈"},
			},
		},
		wasmEvents,
	)
}

func TestWrongSigner(t *testing.T) {
	ctx, keeper, tempDir, codeID, walletA, _, walletB, privKeyB := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := `{"nop":{}}`

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsgA := types.MsgInstantiateContract{
		Sender:    walletB,
		Admin:     nil,
		Code:      codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}

	ctx = prepareInitSignedTxMultipleMsgs(t, keeper, ctx, []sdk.AccAddress{walletB}, []crypto.PrivKey{privKeyB}, []sdk.Msg{sdkMsgA}, codeID)

	_, err = keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
	}
	require.Contains(t, err.Error(), "is not found in the tx creator set")
}

func TestMultiSig(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, _, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := `{"nop":{}}`

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		Admin:     nil,
		Code:      codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	for i := 0; i < 5; i++ {
		for j := 0; j <= i; j++ {
			multisigAddr := multisigTxCreator(t, &ctx, keeper, i+1, j+1, i+1, sdkMsg)

			contractAddressA, err := keeper.Instantiate(ctx, codeID, multisigAddr, nil, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			if err != nil {
				err = extractInnerError(t, err, nonce, true)
			}
			require.NoError(t, err)

			wasmEvents := getDecryptedWasmEvents(t, ctx, nonce)

			require.Equal(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddressA.String()},
						{Key: "init", Value: "🌈"},
					},
				},
				wasmEvents,
			)

			// Reset wasm events
			ctx, keeper, tempDir, codeID, _, _, _, _ = setupTest(t, "./testdata/test-contract/contract.wasm")
		}
	}
}

func TestMultiSigThreshold(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, _, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := `{"nop":{}}`

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		Admin:     nil,
		Code:      codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	for i := 0; i < 5; i++ {
		for j := 0; j <= i; j++ {
			multisigAddr := multisigTxCreator(t, &ctx, keeper, i+1, j+1, j+1, sdkMsg)

			contractAddressA, err := keeper.Instantiate(ctx, codeID, multisigAddr, nil, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			if err != nil {
				err = extractInnerError(t, err, nonce, true)
			}
			require.NoError(t, err)

			wasmEvents := getDecryptedWasmEvents(t, ctx, nonce)

			require.Equal(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddressA.String()},
						{Key: "init", Value: "🌈"},
					},
				},
				wasmEvents,
			)

			// Reset wasm events
			ctx, keeper, tempDir, codeID, _, _, _, _ = setupTest(t, "./testdata/test-contract/contract.wasm")
		}
	}
}

func TestMultiSigThresholdNotMet(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, _, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := `{"nop":{}}`

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		Admin:     nil,
		Code:      codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	multisigAddr := multisigTxCreator(t, &ctx, keeper, 3, 2, 1, sdkMsg)

	_, err = keeper.Instantiate(ctx, codeID, multisigAddr, nil, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
	}
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestMultiSigExecute(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, _, walletB, privKeyB := setupTest(t, "./testdata/erc20.wasm")
	defer os.RemoveAll(tempDir)

	privKeys, pubKeys, multisigPubKey := generateMultisigAddr(5, 4, ctx, keeper)
	multisigAddr := sdk.AccAddress(multisigPubKey.Address())

	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, multisigAddr, walletB.String())

	contractAddress, _, error := initHelper(t, keeper, ctx, codeID, walletB, privKeyB, initMsg, true, defaultGas)
	require.Empty(t, error)

	execMsg := fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String())

	execMsgBz, err := wasmCtx.Encrypt([]byte(execMsg))
	require.NoError(t, err)
	nonce := execMsgBz[0:32]

	sdkMsg := types.MsgExecuteContract{
		Contract:          contractAddress,
		Msg:               execMsgBz,
		SentFunds:         sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		CallbackSignature: nil,
	}

	sdkMsg.Sender = sdk.AccAddress(multisigPubKey.Address())

	tx := authtypes.StdTx{
		Msgs:       []sdk.Msg{sdkMsg},
		Fee:        authtypes.StdFee{},
		Signatures: []authtypes.StdSignature{},
		Memo:       "",
	}

	multiSignature := generateSignatures(t, ctx, keeper, privKeys, pubKeys, multisigPubKey.Address().Bytes(), tx, 4)

	stdSig := authtypes.StdSignature{
		PubKey:    multisigPubKey,
		Signature: multiSignature.Marshal(),
	}

	tx.Signatures = []authtypes.StdSignature{stdSig}
	txBytes, err := keeper.cdc.MarshalBinaryLengthPrefixed(tx)
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)

	execRes, err := keeper.Execute(ctx, contractAddress, multisigAddr, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, true)
	}
	data := getDecryptedData(t, execRes.Data, nonce)
	wasmEvents := getDecryptedWasmEvents(t, ctx, nonce)

	require.Empty(t, err)
	require.Empty(t, data)
	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "action", Value: "transfer"},
				{Key: "sender", Value: multisigAddr.String()},
				{Key: "recipient", Value: walletB.String()},
			},
		},
		wasmEvents,
	)
}

func TestMultiSigCallbacks(t *testing.T) {

}
