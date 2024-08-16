package keeper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	txsigning "cosmossdk.io/x/tx/signing"
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

	wasmtypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	v010types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

func getSignBytes(
	t *testing.T, ctx sdk.Context, signModeHandler txsigning.HandlerMap, builder client.TxBuilder, multisigAccount Account, signer Account, rootMultisigAccount *Account,
) []byte {
	// workaround for nested multisigs
	if rootMultisigAccount == nil {
		rootMultisigAccount = &multisigAccount
	}
	sig := sdksigning.SignatureV2{
		PubKey:   signer.public,
		Sequence: multisigAccount.acct.GetSequence() - 1,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
			Signature: nil,
		},
	}
	err := builder.SetSignatures(sig)
	require.NoError(t, err)
	signerData := authsigning.SignerData{
		ChainID:       TestConfig.ChainID,
		AccountNumber: rootMultisigAccount.acct.GetAccountNumber(),
		Sequence:      rootMultisigAccount.acct.GetSequence() - 1,
		Address:       rootMultisigAccount.acct.GetPubKey().String(),
		PubKey:        signer.acct.GetPubKey(),
	}
	bytesToSign, err := authsigning.GetSignBytesAdapter(ctx, &signModeHandler, sdksigning.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, signerData, builder.GetTx())
	require.NoError(t, err)

	return bytesToSign
}

func generateSignatures(
	t *testing.T, ctx sdk.Context, signModeHandler txsigning.HandlerMap,
	builder client.TxBuilder, multisigAccount Account, signers []Account, actualSigners int,
	rootMultisigAccount *Account,
) *sdksigning.MultiSignatureData {
	multiSig := multisig.NewMultisig(len(signers))

	for i := 0; i < len(signers); i++ {
		signBytes := getSignBytes(t, ctx, signModeHandler, builder, multisigAccount, signers[i], rootMultisigAccount)
		var signature []byte
		if i < actualSigners {
			fmt.Printf("SIGNBYTES %v", signBytes)
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
	t *testing.T, ctx *sdk.Context, keeper Keeper, n int, threshold int, actualSigners int, sdkMsg sdk.Msg, rootMultisigAccount *Account,
) (txsigning.HandlerMap, []Account, Account) {
	fmt.Println("CHECKPOINT 6.1")
	signers, multisigAccount := generateMultisigAccount(*ctx, keeper, n, threshold)
	fmt.Println("CHECKPOINT 6.2")
	signModeHandler := multisigTxCreatorForExisting(t, ctx, multisigAccount, signers, actualSigners, sdkMsg, rootMultisigAccount)
	fmt.Println("CHECKPOINT 6.3")
	return signModeHandler, signers, multisigAccount
}

func multisigTxCreatorForExisting(
	t *testing.T, ctx *sdk.Context, multisigAccount Account, signers []Account, actualSigners int, sdkMsg sdk.Msg, rootMultisigAccount *Account,
) txsigning.HandlerMap {
	fmt.Println("CHECKPOINT 6.2.1")
	switch msg := sdkMsg.(type) {
	case *types.MsgInstantiateContract:
		msg.Sender = multisigAccount.address
		msg.SenderAddress = multisigAccount.address.String()
	case *types.MsgExecuteContract:
		msg.Sender = multisigAccount.address
		msg.SenderAddress = multisigAccount.address.String()
	}
	fmt.Println("CHECKPOINT 6.2.2")

	txConfig := authtx.NewTxConfig(MakeTestCodec(), authtx.DefaultSignModes)
	fmt.Println("CHECKPOINT 6.2.3")
	signmodeHandler := txConfig.SignModeHandler()
	fmt.Println("CHECKPOINT 6.2.4")
	builder := txConfig.NewTxBuilder()
	fmt.Println("CHECKPOINT 6.2.5")
	builder.SetFeeAmount(nil)
	fmt.Println("CHECKPOINT 6.2.6")
	builder.SetGasLimit(0)
	fmt.Println("CHECKPOINT 6.2.7")
	builder.SetTimeoutHeight(0)
	fmt.Println("CHECKPOINT 6.2.8")

	_ = builder.SetMsgs(sdkMsg)

	fmt.Println("CHECKPOINT 6.2.9")
	multiSignature := generateSignatures(t, *ctx, *signmodeHandler, builder, multisigAccount, signers, actualSigners, rootMultisigAccount)
	fmt.Println("CHECKPOINT 6.2.10")
	signature := sdksigning.SignatureV2{
		PubKey:   multisigAccount.public,
		Sequence: multisigAccount.acct.GetSequence() - 1,
		Data:     multiSignature,
	}
	fmt.Println("CHECKPOINT 6.2.11")
	err := builder.SetSignatures(signature)
	require.NoError(t, err)
	fmt.Println("CHECKPOINT 6.2.12")

	tx := builder.(protoTxProvider)
	fmt.Println("CHECKPOINT 6.2.13")
	txBytes, err := tx.GetProtoTx().Marshal()
	fmt.Println("CHECKPOINT 6.2.14")
	require.NoError(t, err)
	*ctx = ctx.WithTxBytes(txBytes)
	fmt.Println("CHECKPOINT 6.2.15")
	*ctx = types.WithTXCounter(*ctx, 1)
	fmt.Println("CHECKPOINT 6.2.16")
	// updateLightClientHelper(t, *ctx)

	return *signmodeHandler
}

type Account struct {
	acct    sdk.AccountI
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

	fmt.Println("CHECKPOINT 6.1.1")
	for i := 0; i < n; i++ {
		deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
		fmt.Println("CHECKPOINT 6.1.2")
		_, privKey, _ := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, deposit.Add(deposit...))
		fmt.Println("CHECKPOINT 6.1.3")
		accounts[i] = newAccount(ctx, keeper, privKey)
		fmt.Println("CHECKPOINT 6.1.4")
	}

	fmt.Println("CHECKPOINT 6.1.5")
	multisigAccount := generateMultisigAccountFromPublicKeys(ctx, keeper, Accounts(accounts).pubKeys(), threshold)
	fmt.Println("CHECKPOINT 6.1.6")

	return accounts, multisigAccount
}

func generateMultisigAccountFromPublicKeys(ctx sdk.Context, keeper Keeper, pubKeys []crypto.PubKey, threshold int) Account {
	multisigPubkey := multisigkeys.NewLegacyAminoPubKey(threshold, pubKeys)

	// Register to keeper
	addr := multisigPubkey.Address().Bytes()
	baseAcct := authtypes.NewBaseAccountWithAddress(addr)
	baseAcct.SetAccountNumber(keeper.accountKeeper.NextAccountNumber(ctx))
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
	accounts := make([]sdk.AccountI, len(creators))
	for i, acc := range creators {
		account, err := authante.GetSignerAcc(ctx, keeper.accountKeeper, acc)
		require.NoError(t, err)
		accounts[i] = account
	}

	tx := NewTestTxMultiple(ctx, initMsgs, accounts, privKeys)
	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)
	return ctx
}

func TestMultipleSigners(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, walletB, privKeyB := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

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

	contractAddressA, _, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, true, false)
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

	contractAddressB, _, err := keeper.Instantiate(ctx, codeID, walletB, nil, initMsgBz, "demo contract 2", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false, false)
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
	ctx, keeper, codeID, _, walletA, _, walletB, privKeyB := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

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

	_, _, err = keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false, false)
	}
	require.Contains(t, err.Error(), "is not found in the tx signer set")
}

func TestMultiSig(t *testing.T) {
	fmt.Println("CHECKPOINT 1")
	ctx, keeper, codeID, codeHash, _, _, _, _ := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})
	fmt.Println("CHECKPOINT 2")

	initMsg := `{"nop":{}}`
	msg := types.NewSecretMsg([]byte(codeHash), []byte(initMsg))
	fmt.Println("CHECKPOINT 3")

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]
	fmt.Println("CHECKPOINT 4")

	for i := 0; i < 5; i++ {
		for j := 0; j <= i; j++ {
			fmt.Println("CHECKPOINT 5")
			label := fmt.Sprintf("demo contract %d %d", i, j)
			sdkMsg := types.MsgInstantiateContract{
				CodeID:    codeID,
				Label:     label,
				InitMsg:   initMsgBz,
				InitFunds: nil,
			}
			fmt.Println("CHECKPOINT 6")

			_, _, multisigAddr := multisigTxCreator(t, &ctx, keeper, i+1, j+1, i+1, &sdkMsg, nil)

			fmt.Println("CHECKPOINT 7")
			contractAddressA, _, err := keeper.Instantiate(ctx, codeID, multisigAddr.address, nil, initMsgBz, label, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			fmt.Println("CHECKPOINT 8")
			if err != nil {
				err = extractInnerError(t, err, nonce, false, false)
			}
			require.NoError(t, err)

			fmt.Println("CHECKPOINT 9")
			wasmEvents := getDecryptedWasmEvents(t, ctx, nonce)
			fmt.Println("CHECKPOINT 10")

			require.Equal(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddressA.String()},
						{Key: "init", Value: "🌈"},
					},
				},
				wasmEvents,
			)
			fmt.Println("CHECKPOINT 11")

			// Reset wasm events
			ctx, keeper, codeID, codeHash, _, _, _, _ = setupTest(t, "./testdata/contract.wasm", sdk.Coins{})
		}
	}
	fmt.Println("CHECKPOINT 12")
}

func TestMultiSigThreshold(t *testing.T) {
	ctx, keeper, codeID, codeHash, _, _, _, _ := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

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

			_, _, multisigAddr := multisigTxCreator(t, &ctx, keeper, i+1, j+1, j+1, &sdkMsg, nil)

			contractAddressA, _, err := keeper.Instantiate(ctx, codeID, multisigAddr.address, nil, initMsgBz, label, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			if err != nil {
				err = extractInnerError(t, err, nonce, true, false)
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
			ctx, keeper, codeID, _, _, _, _, _ = setupTest(t, "./testdata/contract.wasm", sdk.Coins{})
		}
	}
}

func TestMultiSigThresholdNotMet(t *testing.T) {
	ctx, keeper, codeID, codeHash, _, _, _, _ := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

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

	_, _, multisigAddr := multisigTxCreator(t, &ctx, keeper, 3, 2, 1, &sdkMsg, nil)

	_, _, err = keeper.Instantiate(ctx, codeID, multisigAddr.address, nil, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false, false)
	}
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestMultiSigExecute(t *testing.T) {
	ctx, keeper, codeID, codeHash, _, _, walletB, privKeyB := setupTest(t, "./testdata/erc20.wasm", sdk.Coins{})

	accounts, multisigAccount := generateMultisigAccount(ctx, keeper, 5, 4)

	initMsg := fmt.Sprintf(
		`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`,
		multisigAccount.address, walletB.String(),
	)

	_, _, contractAddress, _, error := initHelper(t, keeper, ctx, codeID, walletB, nil, privKeyB, initMsg, true, false, defaultGasForTests)
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

	_ = multisigTxCreatorForExisting(t, &ctx, multisigAccount, accounts, 4, &sdkMsg, nil)

	execRes, err := keeper.Execute(ctx, contractAddress, multisigAccount.address, execMsgBz, funds, nil, wasmtypes.HandleTypeExecute)
	if err != nil {
		err = extractInnerError(t, err, nonce, true, false)
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
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

	// init
	_, _, contractAddress, initEvents, error := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
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

	_, _, multisigAddr := multisigTxCreator(t, &ctx, keeper, 3, 2, 2, &sdkMsg, nil)

	execRes, err := keeper.Execute(ctx, contractAddress, multisigAddr.address, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil, wasmtypes.HandleTypeExecute)
	if err != nil {
		err = extractInnerError(t, err, nonce, true, false)
	}
	data := getDecryptedData(t, execRes.Data, nonce)
	execEvents := getDecryptedWasmEvents(t, ctx, nonce)

	require.Contains(t, execEvents[0], v010types.LogAttribute{Key: "contract_address", Value: contractAddress.String()})
	require.Contains(t, execEvents[0], v010types.LogAttribute{Key: "banana", Value: "🍌"})
	require.Contains(t, execEvents[1], v010types.LogAttribute{Key: "contract_address", Value: contractAddress.String()})
	require.Contains(t, execEvents[1], v010types.LogAttribute{Key: "kiwi", Value: "🥝"})
	require.Contains(t, execEvents[2], v010types.LogAttribute{Key: "contract_address", Value: contractAddress.String()})
	require.Contains(t, execEvents[2], v010types.LogAttribute{Key: "watermelon", Value: "🍉"})
	require.Equal(t, []byte{2, 3}, data)
}

func TestMultiSigInMultiSig(t *testing.T) {
	ctx, keeper, codeID, codeHash, _, privKeyA, _, privKeyB := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

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
		Sender:        multimultisigAccount.address,
		SenderAddress: multimultisigAccount.address.String(),
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	txConfig := authtx.NewTxConfig(MakeTestCodec(), authtx.DefaultSignModes)
	signModeHandler := txConfig.SignModeHandler()
	builder := txConfig.NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(&sdkMsg)
	multimultiSignBytes := getSignBytes(t, ctx, *signModeHandler, builder, multimultisigAccount, multisigAccount, &multimultisigAccount)
	multimultiSig := multisig.NewMultisig(3)

	// Sign by multisig
	multiSignature := generateSignatures(t, ctx, *signModeHandler, builder, multisigAccount, accounts, 3, &multimultisigAccount)
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
		Sequence: multimultisigAcc.GetSequence() - 1,
		Data:     multimultiSig,
	}
	err = builder.SetSignatures(signature)
	require.NoError(t, err)

	tx := builder.(protoTxProvider)
	txBytes, err := tx.GetProtoTx().Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	contractAddressA, _, err := keeper.Instantiate(
		ctx,
		codeID,
		multimultisigAccount.address,
		nil,
		initMsgBz,
		"demo contract 1",
		sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		nil,
	)
	if err != nil {
		err = extractInnerError(t, err, nonce, true, false)
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
	ctx, keeper, codeID, codeHash, _, privKeyA, _, privKeyB := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

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
		Sender:        multimultisigAccount.address,
		SenderAddress: multimultisigAccount.address.String(),
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	txConfig := authtx.NewTxConfig(MakeTestCodec(), authtx.DefaultSignModes)
	signModeHandler := txConfig.SignModeHandler()
	builder := txConfig.NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(&sdkMsg)
	multimultiSignBytes := getSignBytes(t, ctx, *signModeHandler, builder, multimultisigAccount, multisigAccount, &multimultisigAccount)
	multimultiSig := multisig.NewMultisig(3)

	// Sign by multisig
	multiSignature := generateSignatures(t, ctx, *signModeHandler, builder, multisigAccount, accounts, 3, &multimultisigAccount)
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
		Sequence: multimultisigAcc.GetSequence() - 1,
		Data:     multimultiSig,
	}
	err = builder.SetSignatures(signature)
	require.NoError(t, err)

	tx := builder.(protoTxProvider)
	txBytes, err := tx.GetProtoTx().Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	contractAddressA, _, err := keeper.Instantiate(
		ctx,
		codeID,
		multimultisigAccount.address,
		nil,
		initMsgBz,
		"demo contract 1",
		sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		nil,
	)
	if err != nil {
		err = extractInnerError(t, err, nonce, true, false)
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
	ctx, keeper, codeID, codeHash, _, _, _, _ := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

	edKey := ed25519.GenPrivKey()
	edPub := edKey.PubKey()
	edAddr := sdk.AccAddress(edPub.Address())
	baseAcct := authtypes.NewBaseAccountWithAddress(edAddr)
	baseAcct.SetAccountNumber(keeper.accountKeeper.NextAccountNumber(ctx))
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
	// nonce := initMsgBz[0:32]

	sdkMsg := types.MsgInstantiateContract{
		Sender: edAddr,
		// Admin:     nil,
		CodeID:    codeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
	}

	ctx = prepareInitSignedTxMultipleMsgs(t, keeper, ctx, []sdk.AccAddress{edAddr}, []crypto.PrivKey{edKey}, []sdk.Msg{&sdkMsg}, codeID)

	_, _, err = keeper.Instantiate(ctx, codeID, edAddr, nil, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
	require.Contains(t, err.Error(), "failed to deserialize data")
}

func TestInvalidKeyTypeInMultisig(t *testing.T) {
	ctx, keeper, codeID, codeHash, _, privKeyA, _, privKeyB := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

	edKey := ed25519.GenPrivKey()
	edPub := edKey.PubKey()
	edAddr := sdk.AccAddress(edPub.Address())
	baseAcct := authtypes.NewBaseAccountWithAddress(edAddr)
	baseAcct.SetAccountNumber(keeper.accountKeeper.NextAccountNumber(ctx))
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

	txConfig := authtx.NewTxConfig(MakeTestCodec(), authtx.DefaultSignModes)
	signModeHanler := txConfig.SignModeHandler()
	builder := txConfig.NewTxBuilder()
	builder.SetFeeAmount(nil)
	builder.SetGasLimit(0)
	builder.SetTimeoutHeight(0)

	_ = builder.SetMsgs(&sdkMsg)

	multiSignature := generateSignatures(t, ctx, *signModeHanler, builder, multisigPubkey, accounts, 3, nil)

	multisigAcc := keeper.accountKeeper.GetAccount(ctx, multisigPubkey.address.Bytes())
	signature := sdksigning.SignatureV2{
		PubKey:   multisigPubkey.public,
		Sequence: multisigAcc.GetSequence() - 1,
		Data:     multiSignature,
	}
	err = builder.SetSignatures(signature)
	require.NoError(t, err)

	tx := builder.(protoTxProvider)
	txBytes, err := tx.GetProtoTx().Marshal()
	require.NoError(t, err)
	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	_, _, err = keeper.Instantiate(
		ctx,
		codeID,
		sdk.AccAddress(multisigPubkey.address),
		nil,
		initMsgBz,
		"demo contract 1",
		sdk.NewCoins(sdk.NewInt64Coin("denom", 0)),
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongFundsNoFunds(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, nil, privKeyA, initMsgBz, codeID, nil)

	_, _, err = keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 1000)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false, false)
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongFundsSomeFunds(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

	initMsg := `{"nop":{}}`

	msg := types.SecretMsg{
		CodeHash: []byte(codeHash),
		Msg:      []byte(initMsg),
	}

	initMsgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)
	nonce := initMsgBz[0:32]

	ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, nil, privKeyA, initMsgBz, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 200)))

	_, _, err = keeper.Instantiate(ctx, codeID, walletA, nil, initMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 1000)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false, false)
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongMessage(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, "./testdata/contract.wasm", sdk.Coins{})

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

	ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, nil, privKeyA, initMsgBz, codeID, nil)

	_, _, err = keeper.Instantiate(ctx, codeID, walletA, nil, notTheRealMsgBz, "demo contract 1", sdk.NewCoins(sdk.NewInt64Coin("denom", 1000)), nil)
	if err != nil {
		err = extractInnerError(t, err, nonce, false, false)
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}

func TestWrongContractAddress(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, walletB, privKeyB := setupTest(t, "./testdata/erc20.wasm", sdk.Coins{})

	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	_, _, contractAddress, _, stderr := initHelper(t, keeper, ctx, codeID, walletB, nil, privKeyB, initMsg, true, false, defaultGasForTests)
	require.Empty(t, stderr)
	_, _, differentContractAddress, _, stderr := initHelper(t, keeper, ctx, codeID, walletB, nil, privKeyB, initMsg, true, false, defaultGasForTests)
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

	_, err = keeper.Execute(ctx, differentContractAddress, walletA, execMsgBz, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil, wasmtypes.HandleTypeExecute)
	if err != nil {
		err = extractInnerError(t, err, nonce, false, false)
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to verify transaction signature")
}
