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
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

func getSignBytes(
	t *testing.T, signModeHandler authsigning.SignModeHandler, builder client.TxBuilder, multisigAccount Account, signer Account,
) []byte {
	sig := sdksigning.SignatureV2{
		PubKey:   signer.public,
		Sequence: multisigAccount.acct.GetSequence(),
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
			Signature: nil,
		},
	}
	err := builder.SetSignatures(sig)
	require.NoError(t, err)
	signerData := authsigning.SignerData{
		ChainID:       "",
		AccountNumber: signer.acct.GetAccountNumber(),
		Sequence:      signer.acct.GetSequence(),
	}
	bytesToSign, err := signModeHandler.GetSignBytes(sdksigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, signerData, builder.GetTx())
	require.NoError(t, err)

	return bytesToSign
}

func generateSignatures(
	t *testing.T, signModeHandler authsigning.SignModeHandler,
	builder client.TxBuilder, multisigAccount Account, signers []Account, actualSigners int,
) *sdksigning.MultiSignatureData {
	multiSig := multisig.NewMultisig(len(signers))

	for i := 0; i < len(signers); i++ {
		signBytes := getSignBytes(t, signModeHandler, builder, multisigAccount, signers[i])
		var signature []byte
		if i < actualSigners {
			signature, _ = signers[i].private.Sign(signBytes)
		}
		signatureData := sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
			Signature: signature,
		}

		err := multisig.AddSignatureFromPubKey(multiSig, &signatureData, signers[i].public, Accounts(signers).pubKeys())
		require.NoError(t, err)
	}

	fmt.Printf("Multisig is %v\n", multiSig)

	return multiSig
}

func multisigTxCreator(
	t *testing.T, ctx *sdk.Context, keeper Keeper, n int, threshold int, actualSigners int, sdkMsg sdk.Msg,
) (authsigning.SignModeHandler, []Account, Account) {
	signers, multisigAccount := generateMultisigAccount(*ctx, keeper, n, threshold)
	signModeHandler := multisigTxCreatorForExisting(t, ctx, multisigAccount, signers, actualSigners, sdkMsg)
	return signModeHandler, signers, multisigAccount
}
func multisigTxCreatorForExisting(
	t *testing.T, ctx *sdk.Context, multisigAccount Account, signers []Account, actualSigners int, sdkMsg sdk.Msg,
) authsigning.SignModeHandler {
	switch msg := sdkMsg.(type) {
	case *types.MsgInstantiateContract:
		msg.Sender = multisigAccount.address
	case *types.MsgExecuteContract:
		msg.Sender = multisigAccount.address
	}

	txConfig := authtx.NewTxConfig(nil, authtx.DefaultSignModes)
	signmodeHandler := txConfig.SignModeHandler()
	builder := txConfig.NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(sdkMsg)

	multiSignature := generateSignatures(t, signmodeHandler, builder, multisigAccount, signers, actualSigners)
	signature := sdksigning.SignatureV2{
		PubKey:   multisigAccount.public,
		Sequence: multisigAccount.acct.GetSequence(),
		Data:     multiSignature,
	}
	err := builder.SetSignatures(signature)
	require.NoError(t, err)

	tx := builder.(protoTxProvider)
	txbytes, err := tx.GetProtoTx().Marshal()
	require.NoError(t, err)
	*ctx = ctx.WithTxBytes(txbytes)

	return signmodeHandler
}

type Account struct {
	acct    authtypes.AccountI
	address sdk.AccAddress
	public  crypto.PubKey
	private crypto.PrivKey
}

func newAccount(ctx sdk.Context, keeper Keeper, private crypto.PrivKey) Account {
	public := private.PubKey()
	address := sdk.AccAddress(public.Address())
	acct, err := authante.GetSignerAcc(ctx, keeper.accountKeeper, address)
	if err != nil {
		panic(fmt.Sprintf("failed to get signer account %v", address))
	}
	return Account{
		acct,
		address,
		public,
		private,
	}
}

type Accounts []Account

func (accts Accounts) pubKeys() []crypto.PubKey {
	var pubKeys []crypto.PubKey
	for _, acct := range accts {
		pubKeys = append(pubKeys, acct.public)
	}
	return pubKeys
}

func generateMultisigAccount(ctx sdk.Context, keeper Keeper, n int, threshold int) ([]Account, Account) {
	accounts := make([]Account, n)

	for i := 0; i < n; i++ {
		deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
		_, privKey := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, deposit.Add(deposit...))
		accounts[i] = newAccount(ctx, keeper, privKey)
	}

	multisigAccount := generateMultisigAccountFromPublicKeys(ctx, keeper, Accounts(accounts).pubKeys(), threshold)

	return accounts, multisigAccount
}

func generateMultisigAccountFromPublicKeys(ctx sdk.Context, keeper Keeper, pubKeys []crypto.PubKey, threshold int) Account {
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

	return Account{
		acct:    baseAcct,
		address: addr,
		public:  multisigPubkey,
		private: nil,
	}
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
	msg := types.NewSecretMsg([]byte(codeHash), []byte(initMsg))

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	for i := 0; i < 5; i++ {
		for j := 0; j <= i; j++ {
			label := fmt.Sprintf("demo contract %d %d", i, j)
			sdkMsg := types.MsgInstantiateContract{
				// Admin:     nil,
				CodeID:    codeID,
				Label:     label,
				InitMsg:   initMsgBz,
				InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
			}

			_, _, multisigAddr := multisigTxCreator(t, &ctx, keeper, i+1, j+1, i+1, &sdkMsg)

			contractAddressA, err := keeper.Instantiate(ctx, codeID, multisigAddr.address /* nil, */, initMsgBz, label, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
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
			label := fmt.Sprintf("demo contract %d %d", i+1, j+1)
			sdkMsg := types.MsgInstantiateContract{
				// Admin:     nil,
				CodeID:    codeID,
				Label:     label,
				InitMsg:   initMsgBz,
				InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
			}

			_, _, multisigAddr := multisigTxCreator(t, &ctx, keeper, i+1, j+1, j+1, &sdkMsg)

			contractAddressA, err := keeper.Instantiate(ctx, codeID, multisigAddr.address /* nil,*/, initMsgBz, label, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
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

	_, _, multisigAddr := multisigTxCreator(t, &ctx, keeper, 3, 2, 1, &sdkMsg)

	_, err = keeper.Instantiate(ctx, codeID, multisigAddr.address /* nil,*/, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false)
	}
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestMultiSigExecute(t *testing.T) {
	t.SkipNow() // skipping till multisig is fixed
	ctx, keeper, codeID, codeHash, _, _, walletB, privKeyB := setupTest(t, "./testdata/erc20.wasm")

	accounts, multisigAccount := generateMultisigAccount(ctx, keeper, 5, 4)

	initMsg := fmt.Sprintf(
		`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`,
		multisigAccount.address, walletB.String(),
	)

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

	funds := sdk.NewCoins(sdk.NewInt64Coin("denom", 0))
	sdkMsg := types.MsgExecuteContract{
		Contract:    contractAddress,
		Msg:         execMsgBz,
		SentFunds:   funds,
		CallbackSig: nil,
	}

	_ = multisigTxCreatorForExisting(t, &ctx, multisigAccount, accounts, 4, &sdkMsg)

	execRes, err := keeper.Execute(ctx, contractAddress, multisigAccount.address, execMsgBz, funds, nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, true)
	}
	require.NoError(t, err)

	data := getDecryptedData(t, execRes.Data, nonce)
	wasmEvents := getDecryptedWasmEvents(t, ctx, nonce)
	require.Empty(t, data)
	require.Equal(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "action", Value: "transfer"},
				{Key: "sender", Value: multisigAccount.address.String()},
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

	_, _, multisigAddr := multisigTxCreator(t, &ctx, keeper, 3, 2, 2, &sdkMsg)

	execRes, err := keeper.Execute(ctx, contractAddress, multisigAddr.address, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
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

	accounts, multisigAccount := generateMultisigAccount(ctx, keeper, 5, 3)
	multiSigPubKeys := []crypto.PubKey{multisigAccount.public, privKeyA.PubKey(), privKeyB.PubKey()}
	multimultisigAccount := generateMultisigAccountFromPublicKeys(ctx, keeper, multiSigPubKeys, 2)

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		Sender: multimultisigAccount.address,
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	txConfig := authtx.NewTxConfig(nil, authtx.DefaultSignModes)
	signModeHandler := txConfig.SignModeHandler()
	builder := txConfig.NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(&sdkMsg)
	multimultiSignBytes := getSignBytes(t, signModeHandler, builder, multimultisigAccount, multisigAccount)
	multimultiSig := multisig.NewMultisig(3)

	// Sign by multisig
	multiSignature := generateSignatures(t, signModeHandler, builder, multisigAccount, accounts, 3)
	fmt.Printf("multisig sig: %v\n", multiSignature)

	// Sign by wallet A
	walletASignature, _ := privKeyA.Sign(multimultiSignBytes)
	walletASignatureData := sdksigning.SingleSignatureData{
		SignMode:  sdksigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		Signature: walletASignature,
	}
	walletBSignatureData := sdksigning.SingleSignatureData{
		SignMode:  sdksigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		Signature: nil, // No signature provided
	}

	fmt.Printf("wallet A sig: %v\n", walletASignature)

	_ = multisig.AddSignatureFromPubKey(multimultiSig, multiSignature, multisigAccount.public, multiSigPubKeys)
	_ = multisig.AddSignatureFromPubKey(multimultiSig, &walletASignatureData, privKeyA.PubKey(), multiSigPubKeys)
	_ = multisig.AddSignatureFromPubKey(multimultiSig, &walletBSignatureData, privKeyB.PubKey(), multiSigPubKeys)

	fmt.Printf("multimultisig sig: %v\n", multimultiSig)

	multimultisigAcc := keeper.accountKeeper.GetAccount(ctx, multimultisigAccount.address.Bytes())
	signature := sdksigning.SignatureV2{
		PubKey:   multimultisigAccount.public,
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
		multimultisigAccount.address,
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

	accounts, multisigAccount := generateMultisigAccount(ctx, keeper, 5, 3)
	multiSigPubKeys := []crypto.PubKey{multisigAccount.public, privKeyA.PubKey(), privKeyB.PubKey()}
	multimultisigAccount := generateMultisigAccountFromPublicKeys(ctx, keeper, multiSigPubKeys, 2)

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		Sender: multimultisigAccount.address,
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	txConfig := authtx.NewTxConfig(nil, authtx.DefaultSignModes)
	signModeHandler := txConfig.SignModeHandler()
	builder := txConfig.NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(&sdkMsg)
	multimultiSignBytes := getSignBytes(t, signModeHandler, builder, multimultisigAccount, multisigAccount)
	multimultiSig := multisig.NewMultisig(3)

	// Sign by multisig
	multiSignature := generateSignatures(t, signModeHandler, builder, multisigAccount, accounts, 3)
	fmt.Printf("multisig sig: %v\n", multiSignature)

	// Sign by wallet A
	walletASignature, _ := privKeyA.Sign(multimultiSignBytes)
	walletASignatureData := sdksigning.SingleSignatureData{
		SignMode:  sdksigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		Signature: walletASignature,
	}
	walletBSignatureData := sdksigning.SingleSignatureData{
		SignMode:  sdksigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		Signature: nil, // No signature provided
	}

	fmt.Printf("wallet A sig: %v\n", walletASignature)

	err = multisig.AddSignatureFromPubKey(multimultiSig, &walletBSignatureData, privKeyB.PubKey(), multiSigPubKeys)
	err = multisig.AddSignatureFromPubKey(multimultiSig, multiSignature, multisigAccount.public, multiSigPubKeys)
	err = multisig.AddSignatureFromPubKey(multimultiSig, &walletASignatureData, privKeyA.PubKey(), multiSigPubKeys)

	fmt.Printf("multimultisig sig: %v\n", multimultiSig)

	multimultisigAcc := keeper.accountKeeper.GetAccount(ctx, multimultisigAccount.address.Bytes())
	signature := sdksigning.SignatureV2{
		PubKey:   multimultisigAccount.public,
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
		multimultisigAccount.address,
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

	edPubAccount := Account{
		private: edKey,
		public:  edPub,
		address: edAddr,
		acct:    baseAcct,
	}
	AccountA := newAccount(ctx, keeper, privKeyA)
	AccountB := newAccount(ctx, keeper, privKeyB)

	pubKeys := []crypto.PubKey{edPub, privKeyA.PubKey(), privKeyB.PubKey()}
	accounts := []Account{edPubAccount, AccountA, AccountB}
	multisigPubkey := generateMultisigAccountFromPublicKeys(ctx, keeper, pubKeys, 2)

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	sdkMsg := types.MsgInstantiateContract{
		Sender: multisigPubkey.address.Bytes(),
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	txConfig := authtx.NewTxConfig(nil, authtx.DefaultSignModes)
	signModeHanler := txConfig.SignModeHandler()
	builder := txConfig.NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(&sdkMsg)

	multiSignature := generateSignatures(t, signModeHanler, builder, multisigPubkey, accounts, 3)

	multisigAcc := keeper.accountKeeper.GetAccount(ctx, multisigPubkey.address.Bytes())
	signature := sdksigning.SignatureV2{
		PubKey:   multisigPubkey.public,
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
		sdk.AccAddress(multisigPubkey.address),
		/* nil, */
		initMsgBz,
		"demo contract 1",
		sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongFundsNoFunds(t *testing.T) {
	t.SkipNow()
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
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongFundsSomeFunds(t *testing.T) {
	t.SkipNow()
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
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongMessage(t *testing.T) {
	t.SkipNow()
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
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongContractAddress(t *testing.T) {
	t.SkipNow()
	ctx, keeper, codeID, codeHash, walletA, privKeyA, walletB, privKeyB := setupTest(t, "./testdata/erc20.wasm")

	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	contractAddress, _, stderr := initHelper(t, keeper, ctx, codeID, walletB, privKeyB, initMsg, true, defaultGasForTests)
	require.Empty(t, stderr)
	differentContractAddress, _, stderr := initHelper(t, keeper, ctx, codeID, walletB, privKeyB, initMsg, true, defaultGasForTests)
	require.Empty(t, stderr)

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
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}
