package keeper

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
	wasmtypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	eng "github.com/scrtlabs/SecretNetwork/types"
	wasmUtils "github.com/scrtlabs/SecretNetwork/x/compute/client/utils"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
	reg "github.com/scrtlabs/SecretNetwork/x/registration"
)

const SupportedFeatures = "staking,stargate,ibc3,random"

var wasmCtx = wasmUtils.WASMContext{
	TestKeyPairPath: "/tmp/id_tx_io.json",
	TestMasterIOKey: reg.MasterKey{Bytes: nil},
}

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(eng.Bech32PrefixAccAddr, eng.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(eng.Bech32PrefixValAddr, eng.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(eng.Bech32PrefixConsAddr, eng.Bech32PrefixConsPub)
	config.Seal()

	spid, err := os.ReadFile("../../../../ias_keys/develop/spid.txt")
	apiKey, err := os.ReadFile("../../../../ias_keys/develop/api_key.txt")

	fmt.Printf("This IS spid: %v\n", spid)
	fmt.Printf("This IS api key: %v\n", apiKey)

	_, err = api.InitBootstrap(spid, apiKey)
	if err != nil {
		panic(fmt.Sprintf("Error initializing the enclave: %v", err))
	}

	b64Bz, err := os.ReadFile(filepath.Join(".", reg.IoExchMasterKeyPath))
	if err != nil {
		panic(fmt.Sprintf("Error reading 'io-master-key.txt': %v", err))
	}

	wasmCtx.TestMasterIOKey.Bytes, err = base64.StdEncoding.DecodeString(string(b64Bz))
	if err != nil {
		panic(fmt.Sprintf("Error reading 'io-master-key.txt': %v", err))
	}
}

func TestNewKeeper(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	_, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	require.NotNil(t, keepers.WasmKeeper)
}

func TestCreate(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), contractID)
	// and verify content
	storedCode, err := keeper.GetWasm(ctx, contractID)
	require.NoError(t, err)
	require.Equal(t, wasmCode, storedCode)
}

func TestCreateDuplicate(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
	require.NoError(t, err)

	// create one copy
	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), contractID)

	// create second copy
	duplicateID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(2), duplicateID)

	// and verify both content is proper
	storedCode, err := keeper.GetWasm(ctx, contractID)
	require.NoError(t, err)
	require.Equal(t, wasmCode, storedCode)
	storedCode, err = keeper.GetWasm(ctx, duplicateID)
	require.NoError(t, err)
	require.Equal(t, wasmCode, storedCode)
}

func TestCreateWithSimulation(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	ctx = ctx.WithBlockHeight(1).
		WithGasMeter(stypes.NewInfiniteGasMeter())

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
	require.NoError(t, err)

	// create this once in simulation mode
	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), contractID)

	// then try to create it in non-simulation mode (should not fail)
	ctx, keepers = CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper = keepers.AccountKeeper, keepers.WasmKeeper
	contractID, err = keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), contractID)

	// and verify content
	code, err := keeper.GetWasm(ctx, contractID)
	require.NoError(t, err)
	require.Equal(t, code, wasmCode)
}

func TestIsSimulationMode(t *testing.T) {
	specs := map[string]struct {
		ctx sdk.Context
		exp bool
	}{
		"genesis block": {
			ctx: sdk.Context{}.WithBlockHeight(0).WithGasMeter(stypes.NewInfiniteGasMeter()),
			exp: false,
		},
		"any regular block": {
			ctx: sdk.Context{}.WithBlockHeight(1).WithGasMeter(stypes.NewGasMeter(10000000)),
			exp: false,
		},
		"simulation": {
			ctx: sdk.Context{}.WithBlockHeight(1).WithGasMeter(stypes.NewInfiniteGasMeter()),
			exp: true,
		},
	}
	for msg := range specs {
		t.Run(msg, func(t *testing.T) {
			// require.Equal(t, spec.exp, isSimulationMode(spec.ctx))
		})
	}
}

func TestCreateWithGzippedPayload(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)

	wasmCode, err := os.ReadFile(filepath.Join(".", contractPath, "test_gzip_contract.wasm.gz"))
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), contractID)
	// and verify content

	storedCode, err := keeper.GetWasm(ctx, contractID)
	h := sha1.New()
	h.Write(storedCode)
	hashStoredCode := h.Sum(nil)

	require.NoError(t, err)
	rawCode, err := os.ReadFile(filepath.Join(".", contractPath, "test_gzip_contract_raw.wasm"))

	h = sha1.New()
	h.Write(rawCode)
	hashRawCode := h.Sum(nil)

	require.NoError(t, err)
	require.Equal(t, hashRawCode, hashStoredCode)
}

func TestInstantiate(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, privKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "https://github.com/scrtlabs/SecretNetwork/blob/master/cosmwasm/contracts/hackatom/src/contract.rs", "")
	require.NoError(t, err)

	_, _, bob := keyPubAddr()
	_, _, fred := keyPubAddr()

	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, contractID)
	require.NoError(t, err)

	key := codeInfo.CodeHash

	msg := types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(key)),
		Msg:      initMsgBz,
	}

	initMsgBz, err = wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	gasBefore := ctx.GasMeter().GasConsumed()

	creatorAcc, err := authante.GetSignerAcc(ctx, accKeeper, creator)
	require.NoError(t, err)

	instantiateMsg := types.MsgInstantiateContract{
		Sender:    creator,
		CodeID:    contractID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}
	tx := NewTestTx(&instantiateMsg, creatorAcc, privKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	// create with no balance is also legal
	contractAddr, _, err := keeper.Instantiate(ctx, contractID, creator, nil, initMsgBz, "demo contract 1", nil, nil)
	require.NoError(t, err)
	require.Equal(t, "secret1uhfqhj6cvt7983n6xdxkjhfvx9833qk5pmgfl4", contractAddr.String())

	// gas can change +- 10% before we start failing, though maybe for consensus we should check a constant amount
	gasAfter := ctx.GasMeter().GasConsumed()
	require.Greater(t, gasAfter-gasBefore, types.InstanceCost)
	require.Less(t, gasAfter-gasBefore, types.InstanceCost+10_000)

	// ensure it is stored properly
	info := keeper.GetContractInfo(ctx, contractAddr)
	require.NotNil(t, info)
	require.Equal(t, info.Creator, creator)
	require.Equal(t, info.CodeID, contractID)
	require.Equal(t, info.Label, "demo contract 1")

	// test that creating again with the same label will fail
	_, _, err = keeper.Instantiate(ctx, contractID, creator, nil, initMsgBz, "demo contract 1", nil, nil)
	require.Error(t, err)
}

func TestInstantiateWithDeposit(t *testing.T) {
	specs := map[string]struct {
		fundAddr bool
		expError bool
	}{
		"address with funds": {
			fundAddr: true,
		},
		"address without funds": {
			fundAddr: false,
			expError: true,
		},
		/*
			"blocked address": {
				srcActor: supply.NewModuleAddress(auth.FeeCollectorName),
				fundAddr: true,
				expError: true,
			},
		*/
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			ctx, keeper, codeID, _, _, _, _, _ := setupTest(t, TestContractPaths[hackAtomContract], sdk.NewCoins())

			deposit := 100
			var funds int64 = 0
			if spec.fundAddr {
				funds = 200
			}
			bob, bobPriv := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", funds)))

			fred, _ := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

			initMsgBz, err := json.Marshal(InitMsg{Verifier: fred, Beneficiary: bob})
			require.NoError(t, err)

			wasmCalls := int64(-1)
			if spec.expError {
				wasmCalls = 0
			}

			// when
			_, _, addr, _, err := initHelperImpl(t, keeper, ctx, codeID, bob, nil, bobPriv, string(initMsgBz), false, false, defaultGasForTests, wasmCalls, sdk.NewCoins(sdk.NewInt64Coin("denom", int64(deposit))))
			// then
			if spec.expError {
				require.Error(t, err)
				return
			}
			require.Empty(t, err)
			contractAccount := keeper.accountKeeper.GetAccount(ctx, addr)
			coins := keeper.bankKeeper.GetAllBalances(ctx, contractAccount.GetAddress())
			assert.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("denom", 100)), coins)
		})
	}
}

func TestInstantiateWithNonExistingCodeID(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, privKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)

	const nonExistingCodeID = 9999

	initMsg := InitMsg{}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	initMsgBz, err = wasmCtx.Encrypt(initMsgBz)
	require.NoError(t, err)

	creatorAcc, err := authante.GetSignerAcc(ctx, accKeeper, creator)
	require.NoError(t, err)

	instantiateMsg := types.MsgInstantiateContract{
		Sender:    creator,
		CodeID:    nonExistingCodeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}
	tx := NewTestTx(&instantiateMsg, creatorAcc, privKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	addr, _, err := keeper.Instantiate(ctx, nonExistingCodeID, creator, nil, initMsgBz, "demo contract 2", nil, nil)
	require.True(t, types.ErrNotFound.Is(err), err)
	require.Nil(t, addr)
}

func TestExecute(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator, creatorPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))
	fred, privFred := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, topUp)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	_, _, bob := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)

	codeInfo, err := keeper.GetCodeInfo(ctx, contractID)
	require.NoError(t, err)

	key := codeInfo.CodeHash

	msg := types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(key)),
		Msg:      initMsgBz,
	}

	initMsgBz, err = wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	gasBefore := ctx.GasMeter().GasConsumed()

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, nil, creatorPrivKey, initMsgBz, contractID, deposit)
	// create with no balance is also legal
	addr, _, err := keeper.Instantiate(ctx, contractID, creator, nil, initMsgBz, "demo contract 1", deposit, nil)

	require.NoError(t, err)

	require.Equal(t, "secret12exhpaft5rv3t8lcw9nykudxaddq2nmtv4r3tl", addr.String())

	// ensure bob doesn't exist
	bobAcct := accKeeper.GetAccount(ctx, bob)
	require.Nil(t, bobAcct)

	// ensure funder has reduced balance
	creatorAcct := accKeeper.GetAccount(ctx, creator)
	require.NotNil(t, creatorAcct)
	// we started at 2*deposit, should have spent one above
	coins := keeper.bankKeeper.GetAllBalances(ctx, creatorAcct.GetAddress())
	assert.Equal(t, deposit, coins)

	// ensure contract has updated balance
	contractAcct := accKeeper.GetAccount(ctx, addr)
	require.NotNil(t, contractAcct)
	coins = keeper.bankKeeper.GetAllBalances(ctx, contractAcct.GetAddress())
	assert.Equal(t, deposit, coins)

	// unauthorized - trialCtx so we don't change state
	trialCtx := ctx.WithMultiStore(ctx.MultiStore().CacheWrap().(sdk.MultiStore))

	_, _, _, _, _, trialExecErr := execHelper(t, keeper, trialCtx, addr, creator, creatorPrivKey, `{"release":{}}`, true, false, defaultGasForTests, 0)
	require.Error(t, trialExecErr)
	require.Error(t, trialExecErr.Unauthorized)
	require.Contains(t, trialExecErr.Error(), "unauthorized")

	// verifier can execute, and get proper gas amount
	start := time.Now()

	gasBefore = ctx.GasMeter().GasConsumed()

	require.NoError(t, err)
	// res, _, _, err := execHelper(t, keeper, trialCtx, addr, creator, `{"release":{}}`, true, false, defaultGasForTests)

	initMsgBz = []byte(`{"release":{}}`)

	require.NoError(t, err)

	msg = types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(key)),
		Msg:      initMsgBz,
	}

	msgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	ctx = PrepareExecSignedTx(t, keeper, ctx, fred, privFred, msgBz, addr, topUp)

	res, err := keeper.Execute(ctx, addr, fred, msgBz, topUp, nil, wasmtypes.HandleTypeExecute)
	diff := time.Since(start)
	require.NoError(t, err)
	require.NotNil(t, res)

	// make sure gas is properly deducted from ctx
	gasAfter := ctx.GasMeter().GasConsumed()
	require.Greater(t, gasAfter-gasBefore, types.InstanceCost)
	require.Less(t, gasAfter-gasBefore, types.InstanceCost+8_000)

	// ensure bob now exists and got both payments released
	bobAcct = accKeeper.GetAccount(ctx, bob)
	require.NotNil(t, bobAcct)
	balance := keeper.bankKeeper.GetAllBalances(ctx, bobAcct.GetAddress())
	assert.Equal(t, deposit.Add(topUp...), balance)

	// ensure contract has updated balance
	contractAcct = accKeeper.GetAccount(ctx, addr)
	require.NotNil(t, contractAcct)
	coins = keeper.bankKeeper.GetAllBalances(ctx, contractAcct.GetAddress())
	assert.Equal(t, sdk.Coins{}, coins)

	t.Logf("Duration: %+v (%d gas)\n", diff, gasAfter-gasBefore)
}

func TestExecuteWithDeposit(t *testing.T) {
	specs := map[string]struct {
		srcActor    sdk.AccAddress
		beneficiary sdk.AccAddress
		expError    bool
		fundAddr    bool
	}{
		"actor with funds": {
			fundAddr: true,
			expError: false,
		},
		"actor without funds": {
			fundAddr: false,
			expError: true,
		},
		/*
			"blocked address as actor": {
				srcActor:    blockedAddr,
				fundAddr:    true,
				beneficiary: fred,
				expError:    true,
			},
			 "blocked address as beneficiary": {
				srcActor:    bob,
				fundAddr:    true,
				beneficiary: blockedAddr,
				expError:    true,
			},
		*/
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			ctx, keeper, codeID, _, _, _, _, _ := setupTest(t, TestContractPaths[hackAtomContract], sdk.NewCoins())

			deposit := int64(100)
			var funds int64 = 0
			if spec.fundAddr {
				funds = 200
			}
			bob, bobPriv := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", funds)))
			fred, _ := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

			initMsgBz, err := json.Marshal(InitMsg{Verifier: bob, Beneficiary: fred})
			require.NoError(t, err)

			_, _, contractAddr, _, err := initHelperImpl(t, keeper, ctx, codeID, bob, nil, bobPriv, string(initMsgBz), true, false, defaultGasForTests, -1, sdk.NewCoins())
			require.Empty(t, err)

			wasmCalls := int64(-1)
			if spec.expError {
				wasmCalls = 0
			}

			// when
			_, _, _, _, _, err = execHelperCustomWasmCount(t, keeper, ctx, contractAddr, bob, bobPriv, `{"release":{}}`, false, false, defaultGasForTests, deposit, wasmCalls)

			// then
			if spec.expError {
				require.Error(t, err)
				return
			}
			require.Empty(t, err)
			beneficiaryAccount := keeper.accountKeeper.GetAccount(ctx, fred)
			coins := keeper.bankKeeper.GetAllBalances(ctx, beneficiaryAccount.GetAddress())
			assert.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("denom", deposit)), coins)
		})
	}
}

func TestExecuteWithNonExistingAddress(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, privKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))

	// unauthorized - trialCtx so we don't change state
	nonExistingAddress := sdk.AccAddress([]byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9})
	msgBz, err := wasmCtx.Encrypt([]byte(`{}`))
	require.NoError(t, err)

	creatorAcc, err := authante.GetSignerAcc(ctx, accKeeper, creator)
	require.NoError(t, err)

	executeMsg := types.MsgExecuteContract{
		Sender:    creator,
		Contract:  nonExistingAddress,
		Msg:       msgBz,
		SentFunds: nil,
	}
	tx := NewTestTx(&executeMsg, creatorAcc, privKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	_, err = keeper.Execute(ctx, nonExistingAddress, creator, msgBz, nil, nil, wasmtypes.HandleTypeExecute)
	require.True(t, types.ErrNotFound.Is(err), err)
}

func TestExecuteWithPanic(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator, creatorPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))
	fred, fredPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, topUp)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
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

	_, _, addr, _, err := initHelper(t, keeper, ctx, contractID, creator, nil, creatorPrivKey, string(initMsgBz), false, false, defaultGasForTests)

	execMsgBz, err := wasmCtx.Encrypt([]byte(`{"panic":{}}`))
	require.NoError(t, err)

	fredAcc, err := authante.GetSignerAcc(ctx, accKeeper, fred)
	require.NoError(t, err)

	executeMsg := types.MsgExecuteContract{
		Sender:    fred,
		Contract:  addr,
		Msg:       execMsgBz,
		SentFunds: topUp,
	}
	tx := NewTestTx(&executeMsg, fredAcc, fredPrivKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	// let's make sure we get a reasonable error, no panic/crash
	_, err = keeper.Execute(ctx, addr, fred, execMsgBz, topUp, nil, wasmtypes.HandleTypeExecute)
	require.Error(t, err)
}

func TestExecuteWithCpuLoop(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator, creatorPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))
	fred, fredPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, topUp)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
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

	codeInfo, err := keeper.GetCodeInfo(ctx, contractID)
	require.NoError(t, err)

	hash := codeInfo.CodeHash

	msg := types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(hash)),
		Msg:      initMsgBz,
	}

	msgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	creatorAcc, err := authante.GetSignerAcc(ctx, accKeeper, creator)
	require.NoError(t, err)

	instantiateMsg := types.MsgInstantiateContract{
		Sender:    creator,
		CodeID:    contractID,
		Label:     "demo contract 1",
		InitMsg:   msgBz,
		InitFunds: deposit,
	}
	tx := NewTestTx(&instantiateMsg, creatorAcc, creatorPrivKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	addr, _, err := keeper.Instantiate(ctx, contractID, creator, nil, msgBz, "demo contract 5", deposit, nil)
	require.NoError(t, err)

	// make sure we set a limit before calling
	var gasLimit uint64 = 400_000
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(gasLimit))
	require.Equal(t, uint64(0), ctx.GasMeter().GasConsumed())

	codeHash, err := keeper.GetContractHash(ctx, addr)
	require.NoError(t, err)

	codeHashStr := hex.EncodeToString(codeHash)

	msg2 := types.SecretMsg{
		CodeHash: []byte(codeHashStr),
		Msg:      []byte(`{"cpu_loop":{}}`),
	}

	execMsgBz, err := wasmCtx.Encrypt(msg2.Serialize())
	require.NoError(t, err)

	// ensure we get an out of gas panic
	defer func() {
		r := recover()
		require.NotNil(t, r)
		_, ok := r.(sdk.ErrorOutOfGas)
		require.True(t, ok, "%+v", r)
	}()

	fredAcc, err := authante.GetSignerAcc(ctx, accKeeper, fred)
	require.NoError(t, err)

	executeMsg := types.MsgExecuteContract{
		Sender:    fred,
		Contract:  addr,
		Msg:       execMsgBz,
		SentFunds: nil,
	}
	tx = NewTestTx(&executeMsg, fredAcc, fredPrivKey)

	txBytes, err = tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	// this must fail
	_, err = keeper.Execute(ctx, addr, fred, execMsgBz, nil, nil, wasmtypes.HandleTypeExecute)
	assert.True(t, false)
	// make sure gas ran out
	// TODO: wasmer doesn't return gas used on error. we should consume it (for error on metering failure)
	// require.Equal(t, gasLimit, ctx.GasMeter().GasConsumed())
}

func TestExecuteWithStorageLoop(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator, creatorPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))
	fred, fredPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, topUp)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	_, _, bob := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)

	_, _, addr, _, err := initHelper(t, keeper, ctx, contractID, creator, nil, creatorPrivKey, string(initMsgBz), false, false, defaultGasForTests)

	// make sure we set a limit before calling
	var gasLimit uint64 = 400_002
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(gasLimit))
	require.Equal(t, uint64(0), ctx.GasMeter().GasConsumed())

	codeHash, err := keeper.GetContractHash(ctx, addr)
	require.NoError(t, err)

	codeHashStr := hex.EncodeToString(codeHash)

	msg := types.SecretMsg{
		CodeHash: []byte(codeHashStr),
		Msg:      []byte(`{"storage_loop":{}}`),
	}

	msgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	fredAcc, err := authante.GetSignerAcc(ctx, accKeeper, fred)
	require.NoError(t, err)

	executeMsg := types.MsgExecuteContract{
		Sender:    fred,
		Contract:  addr,
		Msg:       msgBz,
		SentFunds: nil,
	}
	tx := NewTestTx(&executeMsg, fredAcc, fredPrivKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)
	ctx = types.WithTXCounter(ctx, 1)
	// updateLightClientHelper(t, ctx)

	start := time.Now()

	// ensure we get an out of gas panic
	defer func() {
		r := recover()
		require.NotNil(t, r)
		_, ok := r.(sdk.ErrorOutOfGas)
		require.True(t, ok, "%+v", r)

		diff := time.Since(start)
		t.Logf("Duration till out of gas: %+v (%d gas)\n", diff, gasLimit)
	}()

	// this should throw out of gas exception (panic)
	_, err = keeper.Execute(ctx, addr, fred, msgBz, nil, nil, wasmtypes.HandleTypeExecute)
	require.True(t, false, "We must panic before this line")
}

func prettyEvents(t *testing.T, events sdk.Events) string {
	t.Helper()
	type prettyEvent struct {
		Type string
		Attr []map[string]string
	}

	r := make([]prettyEvent, len(events))
	for i, e := range events {
		attr := make([]map[string]string, len(e.Attributes))
		for j, a := range e.Attributes {
			attr[j] = map[string]string{string(a.Key): string(a.Value)}
		}
		r[i] = prettyEvent{Type: e.Type, Attr: attr}
	}
	return string(mustMarshal(t, r))
}

func mustMarshal(t *testing.T, r interface{}) []byte {
	t.Helper()
	bz, err := json.Marshal(r)
	require.NoError(t, err)
	return bz
}

type InitMsg struct {
	Verifier    sdk.AccAddress `json:"verifier"`
	Beneficiary sdk.AccAddress `json:"beneficiary"`
}

type InstantiateMsg struct {
	Counter uint64 `json:"counter"`
	Expires uint64 `json:"expires"`
}
