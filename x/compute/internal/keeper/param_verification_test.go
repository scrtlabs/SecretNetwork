package keeper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	multisigkeys "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/crypto/types/multisig"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdksigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

// Create a multisig address of `n` new accounts and a threshold of `threshold`, then sign `sdkMsg` using
// `actualSigners` accounts from the `n` new accounts. Serialize the message and add it to the `ctx`.
func multisigTxCreator(
	t *testing.T, ctx *sdk.Context, keeper Keeper, n int, threshold int, actualSigners int, sdkMsg sdk.Msg,
) sdk.AccAddress {
	t.SkipNow() // skipping till multisig is fixed
	privKeys, pubKeys, multisigPubKey := generateMultisigAddr(*ctx, keeper, n, threshold)

	switch msg := sdkMsg.(type) {
	case *types.MsgInstantiateContract:
		msg.Sender = sdk.AccAddress(multisigPubKey.Address())
	case *types.MsgExecuteContract:
		msg.Sender = sdk.AccAddress(multisigPubKey.Address())
	}

	builder := authtx.NewTxConfig(nil, authtx.DefaultSignModes).NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(sdkMsg)

	multisigAcc := keeper.accountKeeper.GetAccount(*ctx, multisigPubKey.Address().Bytes())
	multiSignature := generateSignatures(t, privKeys, pubKeys, builder, actualSigners)
	signature := sdksigning.SignatureV2{
		PubKey:   multisigPubKey,
		Sequence: multisigAcc.GetSequence(),
		Data:     multiSignature,
	}
	err := builder.SetSignatures(signature)
	require.NoError(t, err)

	tx := builder.(protoTxProvider)
	txbytes, err := tx.GetProtoTx().Marshal()
	require.NoError(t, err)
	*ctx = ctx.WithTxBytes(txbytes)

	return multisigAcc.GetAddress()
}

// GetSignBytes returns the signBytes of the tx for a given signer
// This is a copy of cosmos-sdk function (cosmos-sdk/x/auth/types/StdTx.GetSignBytes()
// This is because the original `GetSignBytes` was probably meant to be used before the transaction gets processed, and the
// sequence that gets returned is an increment of what we need.
// This is why we use `acc.GetSequence() - 1`
func getSignBytes(builder client.TxBuilder) []byte {
	tx := builder.(protoTxProvider)
	data, err := tx.GetProtoTx().Body.Marshal()
	if err != nil {
		panic(err)
	}
	return data
}

func generateSignatures(
	t *testing.T,
	privKeys []crypto.PrivKey, pubKeys []crypto.PubKey,
	builder client.TxBuilder, actualSigners int,
) *sdksigning.MultiSignatureData {
	signBytes := getSignBytes(builder)
	multiSig := multisig.NewMultisig(len(privKeys))

	for i := 0; i < actualSigners; i++ {
		signature, _ := privKeys[i].Sign(signBytes)
		signatureData := sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: signature,
		}

		err := multisig.AddSignatureFromPubKey(multiSig, &signatureData, pubKeys[i], pubKeys)
		require.NoError(t, err)
	}

	fmt.Printf("Multisig is %v\n", multiSig)

	return multiSig
}

func generateMultisigAddr(ctx sdk.Context, keeper Keeper, n int, threshold int) ([]crypto.PrivKey, []crypto.PubKey, *multisigkeys.LegacyAminoPubKey) {
	privkeys := make([]crypto.PrivKey, n)
	pubkeys := make([]crypto.PubKey, n)

	for i := 0; i < n; i++ {
		deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
		_, privKey := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, deposit.Add(deposit...))
		privkeys[i] = privKey
		pubkeys[i] = privKey.PubKey()
	}

	multisigPubkey := generateMultisigAddrExisting(ctx, keeper, pubkeys, threshold)

	return privkeys, pubkeys, multisigPubkey
}

func generateMultisigAddrExisting(ctx sdk.Context, keeper Keeper, pubKeys []crypto.PubKey, threshold int) *multisigkeys.LegacyAminoPubKey {
	multisigPubkey := multisigkeys.NewLegacyAminoPubKey(threshold, pubKeys)

	// Register to keeper
	addr := multisigPubkey.Address().Bytes()
	baseAcct := authtypes.NewBaseAccountWithAddress(addr)
	coins := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	_ = baseAcct.SetPubKey(multisigPubkey)
	keeper.accountKeeper.SetAccount(ctx, baseAcct)

	if err := keeper.bankKeeper.MintCoins(ctx, faucetAccountName, coins); err != nil {
		panic(err)
	}

	_ = keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, faucetAccountName, addr, coins)

	return multisigPubkey
}

func prepareInitSignedTxMultipleMsgs(
	t *testing.T, keeper Keeper, ctx sdk.Context,
	creators []sdk.AccAddress, privKeys []crypto.PrivKey, initMsgs []sdk.Msg, codeID uint64,
) sdk.Context {
	t.SkipNow() // skipping till multisig is fixed
	accounts := make([]authtypes.AccountI, len(creators))
	for i, acc := range creators {
		account, err := authante.GetSignerAcc(ctx, keeper.accountKeeper, acc)
		require.NoError(t, err)
		accounts[i] = account
	}

	tx := NewTestTxMultiple(initMsgs, accounts, privKeys)
	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	return ctx.WithTxBytes(txBytes)
}

func TestMultipleSigners(t *testing.T) {
	//
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, walletA, privKeyA, walletB, privKeyB := setupTest(t, "./testdata/test-contract/contract.wasm")

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsgA := types.MsgInstantiateContract{
		Sender: walletA,
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}
	sdkMsgB := types.MsgInstantiateContract{
		Sender: walletB,
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 2",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}

	ctx = prepareInitSignedTxMultipleMsgs(
		t, keeper, ctx,
		[]sdk.AccAddress{walletA, walletB}, []crypto.PrivKey{privKeyA, privKeyB}, []sdk.Msg{&sdkMsgA, &sdkMsgB}, codeID,
	)

	contractAddressA, err := keeper.Instantiate(ctx, codeID, walletA /* nil,*/, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
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

	contractAddressB, err := keeper.Instantiate(ctx, codeID, walletB /* nil,*/, initMsgBz, "demo contract 2", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
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
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, _, walletA, _, walletB, privKeyB := setupTest(t, "./testdata/test-contract/contract.wasm")

	initMsg := `{"nop":{}}`

	initMsgBz, err := wasmCtx.Encrypt([]byte(initMsg))
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsgA := types.MsgInstantiateContract{
		Sender: walletB,
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}

	ctx = prepareInitSignedTxMultipleMsgs(t, keeper, ctx, []sdk.AccAddress{walletB}, []crypto.PrivKey{privKeyB}, []sdk.Msg{&sdkMsgA}, codeID)

	_, err = keeper.Instantiate(ctx, codeID, walletA /* nil,*/, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
	}
	require.Contains(t, err.Error(), "is not found in the tx signer set")
}

func TestMultiSig(t *testing.T) {
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, _, _, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	for i := 0; i < 5; i++ {
		for j := 0; j <= i; j++ {
			label := fmt.Sprintf("demo contract %d%d", i, j)
			sdkMsg := types.MsgInstantiateContract{
				// Admin:     nil,
				CodeID:    codeID,
				Label:     label,
				InitMsg:   initMsgBz,
				InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
			}

			multisigAddr := multisigTxCreator(t, &ctx, keeper, i+1, j+1, i+1, &sdkMsg)

			contractAddressA, err := keeper.Instantiate(ctx, codeID, multisigAddr /* nil, */, initMsgBz, label, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
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
			ctx, keeper, codeID, codeHash, _, _, _, _ = setupTest(t, "./testdata/test-contract/contract.wasm")
		}
	}
}

func TestMultiSigThreshold(t *testing.T) {
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, _, _, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	for i := 0; i < 5; i++ {
		for j := 0; j <= i; j++ {
			label := fmt.Sprintf("demo contract %d%d", i, j)
			sdkMsg := types.MsgInstantiateContract{
				// Admin:     nil,
				CodeID:    codeID,
				Label:     label,
				InitMsg:   initMsgBz,
				InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
			}

			multisigAddr := multisigTxCreator(t, &ctx, keeper, i+1, j+1, j+1, &sdkMsg)

			contractAddressA, err := keeper.Instantiate(ctx, codeID, multisigAddr /* nil,*/, initMsgBz, label, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
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
			ctx, keeper, codeID, _, _, _, _, _ = setupTest(t, "./testdata/test-contract/contract.wasm")
		}
	}
}

func TestMultiSigThresholdNotMet(t *testing.T) {
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, _, _, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	multisigAddr := multisigTxCreator(t, &ctx, keeper, 3, 2, 1, &sdkMsg)

	_, err = keeper.Instantiate(ctx, codeID, multisigAddr /* nil,*/, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
	}
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestMultiSigExecute(t *testing.T) {
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, _, _, walletB, privKeyB := setupTest(t, "./testdata/erc20.wasm")

	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletB.String())

	contractAddress, _, error := initHelper(t, keeper, ctx, codeID, walletB, privKeyB, initMsg, true, defaultGasForTests)
	require.Empty(t, error)

	execMsg := fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String())

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(execMsg),
	}

	execMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := execMsgBz[0:32]

	sdkMsg := types.MsgExecuteContract{
		Contract:    contractAddress,
		Msg:         execMsgBz,
		SentFunds:   sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		CallbackSig: nil,
	}

	multisigAddr := multisigTxCreator(t, &ctx, keeper, 5, 4, 4, &sdkMsg)

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
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")

	// init
	contractAddress, initEvents, error := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, defaultGasForTests)
	require.Empty(t, error)

	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "init", Value: "🌈"},
			},
		},
		initEvents,
	)

	execMsg := fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":2,"y":3}}`, contractAddress.String(), codeHash)

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(execMsg),
	}

	execMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := execMsgBz[0:32]

	sdkMsg := types.MsgExecuteContract{
		Contract:    contractAddress,
		Msg:         execMsgBz,
		SentFunds:   sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		CallbackSig: nil,
	}

	multisigAddr := multisigTxCreator(t, &ctx, keeper, 3, 2, 2, &sdkMsg)

	execRes, err := keeper.Execute(ctx, contractAddress, multisigAddr, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, true)
	}
	data := getDecryptedData(t, execRes.Data, nonce)
	execEvents := getDecryptedWasmEvents(t, ctx, nonce)

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

func TestMultiSigInMultiSig(t *testing.T) {
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, _, privKeyA, _, privKeyB := setupTest(t, "./testdata/test-contract/contract.wasm")

	privKeys, pubKeys, multisigPubkey := generateMultisigAddr(ctx, keeper, 5, 3)
	multiSigPubKeys := []crypto.PubKey{multisigPubkey, privKeyA.PubKey(), privKeyB.PubKey()}
	multimultisigPubkey := generateMultisigAddrExisting(ctx, keeper, multiSigPubKeys, 2)

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		Sender: sdk.AccAddress(multimultisigPubkey.Address()),
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	builder := authtx.NewTxConfig(nil, authtx.DefaultSignModes).NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(&sdkMsg)
	multimultiSignBytes := getSignBytes(builder)
	multimultiSig := multisig.NewMultisig(3)

	// Sign by multisig
	multiSignature := generateSignatures(t, privKeys, pubKeys, builder, 3)
	fmt.Printf("multisig sig: %v\n", multiSignature)

	// Sign by wallet A
	walletASignature, _ := privKeyA.Sign(multimultiSignBytes)
	walletASignatureData := sdksigning.SingleSignatureData{
		SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
		Signature: walletASignature,
	}

	fmt.Printf("wallet A sig: %v\n", walletASignature)

	_ = multisig.AddSignatureFromPubKey(multimultiSig, multiSignature, multisigPubkey, multiSigPubKeys)
	_ = multisig.AddSignatureFromPubKey(multimultiSig, &walletASignatureData, privKeyA.PubKey(), multiSigPubKeys)

	fmt.Printf("multimultisig sig: %v\n", multimultiSig)

	multimultisigAcc := keeper.accountKeeper.GetAccount(ctx, multimultisigPubkey.Address().Bytes())
	signature := sdksigning.SignatureV2{
		PubKey:   multimultisigPubkey,
		Sequence: multimultisigAcc.GetSequence(),
		Data:     multimultiSig,
	}
	err = builder.SetSignatures(signature)
	require.NoError(t, err)

	tx := builder.(protoTxProvider)
	txBytes, err := tx.GetProtoTx().Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)

	contractAddressA, err := keeper.Instantiate(
		ctx,
		codeID,
		sdk.AccAddress(multimultisigPubkey.Address()),
		/* nil, */
		initMsgBz,
		"demo contract 1",
		sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		nil,
	)
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

func TestMultiSigInMultiSigDifferentOrder(t *testing.T) {
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, _, privKeyA, _, privKeyB := setupTest(t, "./testdata/test-contract/contract.wasm")

	privKeys, pubKeys, multisigPubkey := generateMultisigAddr(ctx, keeper, 5, 3)
	multiPubKeys := []crypto.PubKey{privKeyA.PubKey(), privKeyB.PubKey(), multisigPubkey}
	multimultisigPubkey := generateMultisigAddrExisting(ctx, keeper, multiPubKeys, 2)

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		Sender: sdk.AccAddress(multimultisigPubkey.Address()),
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	builder := authtx.NewTxConfig(nil, authtx.DefaultSignModes).NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(&sdkMsg)

	multimultiSignBytes := getSignBytes(builder)
	multimultiSig := multisig.NewMultisig(3)

	// Sign by multisig
	multiSignature := generateSignatures(t, privKeys, pubKeys, builder, 3)
	fmt.Printf("multisig sig: %v\n", multiSignature)

	// Sign by wallet A
	walletASignature, _ := privKeyA.Sign(multimultiSignBytes)
	walletASignatureData := sdksigning.SingleSignatureData{
		SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
		Signature: walletASignature,
	}

	fmt.Printf("wallet A sig: %v\n", walletASignature)

	err = multisig.AddSignatureFromPubKey(multimultiSig, &walletASignatureData, privKeyA.PubKey(), multiPubKeys)
	err = multisig.AddSignatureFromPubKey(multimultiSig, multiSignature, multisigPubkey, multiPubKeys)

	fmt.Printf("multimultisig sig: %v\n", multimultiSig)

	multimultisigAcc := keeper.accountKeeper.GetAccount(ctx, multimultisigPubkey.Address().Bytes())
	signature := sdksigning.SignatureV2{
		PubKey:   multimultisigPubkey,
		Sequence: multimultisigAcc.GetSequence(),
		Data:     multimultiSig,
	}
	err = builder.SetSignatures(signature)
	require.NoError(t, err)

	tx := builder.(protoTxProvider)
	txBytes, err := tx.GetProtoTx().Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)

	contractAddressA, err := keeper.Instantiate(
		ctx,
		codeID,
		sdk.AccAddress(multimultisigPubkey.Address()),
		/* nil, */
		initMsgBz,
		"demo contract 1",
		sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		nil,
	)
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

func TestInvalidKeyType(t *testing.T) {
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, _, _, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")

	edKey := ed25519.GenPrivKey()
	edPub := edKey.PubKey()
	edAddr := sdk.AccAddress(edPub.Address())
	baseAcct := authtypes.NewBaseAccountWithAddress(edAddr)
	coins := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	_ = baseAcct.SetPubKey(edPub)

	if err := keeper.bankKeeper.MintCoins(ctx, faucetAccountName, sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))); err != nil {
		panic(err)
	}

	_ = keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, faucetAccountName, edAddr, coins)

	keeper.accountKeeper.SetAccount(ctx, baseAcct)

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	//nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		Sender: edAddr,
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	ctx = prepareInitSignedTxMultipleMsgs(t, keeper, ctx, []sdk.AccAddress{edAddr}, []crypto.PrivKey{edKey}, []sdk.Msg{&sdkMsg}, codeID)

	_, err = keeper.Instantiate(ctx, codeID, edAddr /* nil,*/, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestInvalidKeyTypeInMultisig(t *testing.T) {
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, _, privKeyA, _, privKeyB := setupTest(t, "./testdata/test-contract/contract.wasm")

	edKey := ed25519.GenPrivKey()
	edPub := edKey.PubKey()
	edAddr := sdk.AccAddress(edPub.Address())
	baseAcct := authtypes.NewBaseAccountWithAddress(edAddr)
	coins := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	_ = baseAcct.SetPubKey(edPub)

	if err := keeper.bankKeeper.MintCoins(ctx, faucetAccountName, coins); err != nil {
		panic(err)
	}

	_ = keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, faucetAccountName, edAddr, coins)

	keeper.accountKeeper.SetAccount(ctx, baseAcct)

	multisigPubkey := generateMultisigAddrExisting(ctx, keeper, []crypto.PubKey{edPub, privKeyA.PubKey(), privKeyB.PubKey()}, 2)

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	sdkMsg := types.MsgInstantiateContract{
		Sender: multisigPubkey.Address().Bytes(),
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	builder := authtx.NewTxConfig(nil, authtx.DefaultSignModes).NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(&sdkMsg)

	multiSignature := generateSignatures(
		t,
		[]crypto.PrivKey{privKeyA, privKeyB, edKey},
		[]crypto.PubKey{privKeyA.PubKey(), privKeyB.PubKey(), edPub},
		builder,
		3,
	)

	multisigAcc := keeper.accountKeeper.GetAccount(ctx, multisigPubkey.Address().Bytes())
	signature := sdksigning.SignatureV2{
		PubKey:   multisigPubkey,
		Sequence: multisigAcc.GetSequence(),
		Data:     multiSignature,
	}
	err = builder.SetSignatures(signature)
	require.NoError(t, err)

	tx := builder.(protoTxProvider)
	txBytes, err := tx.GetProtoTx().Marshal()
	require.NoError(t, err)
	ctx = ctx.WithTxBytes(txBytes)

	_, err = keeper.Instantiate(
		ctx,
		codeID,
		sdk.AccAddress(multisigPubkey.Address()),
		/* nil, */
		initMsgBz,
		"demo contract 1",
		sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		nil,
	)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongFundsNoFunds(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privKeyA, initMsgBz, codeID, nil)

	_, err = keeper.Instantiate(ctx, codeID, walletA /* nil,*/, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 1000)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
	}
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongFundsSomeFunds(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privKeyA, initMsgBz, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 200)))

	_, err = keeper.Instantiate(ctx, codeID, walletA /* nil,*/, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 1000)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
	}
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongMessage(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/test-contract/contract.wasm")

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	notTheRealMsg := `{"no_logs":{}}`

	notReallyTheMsg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(notTheRealMsg),
	}

	notTheRealMsgBz, err := wasmCtx.Encrypt(notReallyTheMsg.Serialize())
	require.NoError(t, err)

	ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privKeyA, initMsgBz, codeID, nil)

	_, err = keeper.Instantiate(ctx, codeID, walletA /* nil, */, notTheRealMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 1000)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
	}
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongContractAddress(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, walletB, privKeyB := setupTest(t, "./testdata/erc20.wasm")

	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, _, error := initHelper(t, keeper, ctx, codeID, walletB, privKeyB, initMsg, true, defaultGasForTests)
	require.Empty(t, error)
	differentContractAddress, _, error := initHelper(t, keeper, ctx, codeID, walletB, privKeyB, initMsg, true, defaultGasForTests)
	require.Empty(t, error)

	require.NotEqual(t, contractAddress, differentContractAddress)

	execMsg := fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String())

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(execMsg),
	}

	execMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := execMsgBz[0:32]

	ctx = PrepareExecSignedTx(t, keeper, ctx, walletA, privKeyA, execMsgBz, contractAddress, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

	_, err = keeper.Execute(ctx, differentContractAddress, walletA, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
	}
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}
