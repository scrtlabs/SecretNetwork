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

// GetSignBytes returns the signBytes of the tx for a given signer
// This is a copy of cosmos-sdk function (cosmos-sdk/x/auth/types/StdTx.GetSignBytes()
// This is because the original `GetSignBytes` was probably meant to be used before the transaction gets processed, and the
// sequence that gets returned is an increment of what we need.
// This is why we use `acc.GetSequence() - 1`
func GetTestSignBytes(ctx sdk.Context, acc exported.Account, tx auth.StdTx) []byte {
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

func generateSignatures(t *testing.T, ctx sdk.Context, keeper Keeper, privKeys []crypto.PrivKey, pubKeys []crypto.PubKey, accAddress sdk.AccAddress, tx authtypes.StdTx) *multisig.Multisignature {
	multisigAcc := keeper.accountKeeper.GetAccount(ctx, accAddress)
	signBytes := GetTestSignBytes(ctx, multisigAcc, tx)
	multiSig := multisig.NewMultisig(len(privKeys))

	var signDoc authtypes.StdSignDoc
	keeper.cdc.MustUnmarshalJSON(signBytes, &signDoc)
	fmt.Printf("Sign Doc is %+v\n", signDoc)
	fmt.Printf("Sign Bytes is %v\n", signBytes)

	for i := 0; i < len(privKeys); i++ {
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
	fmt.Println(err.Error())
	require.Contains(t, err.Error(), "is not found in the tx creator set")
}

func TestMultiSig(t *testing.T) {
	ctx, keeper, tempDir, codeID, _, _, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")
	defer os.RemoveAll(tempDir)

	initMsg := `{"nop":{}}`

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	privKeys, pubKeys, multisigPubKey := generateMultisigAddr(3, 2, ctx, keeper)

	sdkMsg := types.MsgInstantiateContract{
		Sender:    sdk.AccAddress(multisigPubKey.Address()),
		Admin:     nil,
		Code:      codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	tx := authtypes.StdTx{
		Msgs:       []sdk.Msg{sdkMsg},
		Fee:        authtypes.StdFee{},
		Signatures: []authtypes.StdSignature{},
		Memo:       "",
	}

	multiSignature := generateSignatures(t, ctx, keeper, privKeys, pubKeys, multisigPubKey.Address().Bytes(), tx)

	stdSig := authtypes.StdSignature{
		PubKey:    multisigPubKey,
		Signature: multiSignature.Marshal(),
	}

	tx.Signatures = []authtypes.StdSignature{stdSig}
	txBytes, err := keeper.cdc.MarshalBinaryLengthPrefixed(tx)
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	contractAddressA, err := keeper.Instantiate(ctx, codeID, sdk.AccAddress(multisigPubKey.Address()), nil, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
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
}
