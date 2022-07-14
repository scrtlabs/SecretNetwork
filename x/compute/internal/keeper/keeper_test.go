package keeper

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/enigmampc/SecretNetwork/go-cosmwasm/api"
	eng "github.com/enigmampc/SecretNetwork/types"
	wasmUtils "github.com/enigmampc/SecretNetwork/x/compute/client/utils"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	reg "github.com/enigmampc/SecretNetwork/x/registration"
)

const SupportedFeatures = "staking"

var wasmCtx = wasmUtils.WASMContext{
	TestKeyPairPath:  "/tmp/id_tx_io.json",
	TestMasterIOCert: reg.MasterCertificate{Bytes: nil},
}

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(eng.Bech32PrefixAccAddr, eng.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(eng.Bech32PrefixValAddr, eng.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(eng.Bech32PrefixConsAddr, eng.Bech32PrefixConsPub)
	config.Seal()

	spid, err := ioutil.ReadFile("../../../../ias_keys/develop/spid.txt")
	apiKey, err := ioutil.ReadFile("../../../../ias_keys/develop/api_key.txt")

	fmt.Printf("This IS spid: %v\n", spid)
	fmt.Printf("This IS api key: %v\n", apiKey)

	_, err = api.InitBootstrap(spid, apiKey)
	if err != nil {
		panic(fmt.Sprintf("Error initializing the enclave: %v", err))
	}

	wasmCtx.TestMasterIOCert.Bytes, err = ioutil.ReadFile(filepath.Join(".", reg.IoExchMasterCertPath))
	if err != nil {
		panic(fmt.Sprintf("Error reading 'io-master-cert.der': %v", err))
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

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), contractID)
	// and verify content
	storedCode, err := keeper.GetByteCode(ctx, contractID)
	require.NoError(t, err)
	require.Equal(t, wasmCode, storedCode)
}

/*
func TestCreateStoresInstantiatePermission(t *testing.T) {
	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)
	var (
		deposit = sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
		myAddr  = bytes.Repeat([]byte{1}, sdk.AddrLen)
	)

	specs := map[string]struct {
		srcPermission types.AccessType
		expInstConf   types.AccessConfig
	}{
		"default": {
			srcPermission: types.DefaultParams().DefaultInstantiatePermission,
			expInstConf:   types.AllowEverybody,
		},
		"everybody": {
			srcPermission: types.Everybody,
			expInstConf:   types.AllowEverybody,
		},
		"nobody": {
			srcPermission: types.Nobody,
			expInstConf:   types.AllowNobody,
		},
		"onlyAddress with matching address": {
			srcPermission: types.OnlyAddress,
			expInstConf:   types.AccessConfig{Type: types.OnlyAddress, Address: myAddr},
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			tempDir, err := ioutil.TempDir("", "wasm")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
			accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
			fundAccounts(ctx, accKeeper, myAddr, deposit)

			codeID, err := keeper.Create(ctx, myAddr, wasmCode, "https://github.com/CosmWasm/wasmd/blob/master/x/wasm/testdata/escrow.wasm", "any/builder:tag")
			require.NoError(t, err)

			codeInfo := keeper.GetCodeInfo(ctx, codeID)
			require.NotNil(t, codeInfo)
			assert.True(t, spec.expInstConf.Equals(codeInfo.InstantiateConfig), "got %#v", codeInfo.InstantiateConfig)
		})
	}
}

func TestCreateWithParamPermissions(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := CreateFakeFundedAccount(ctx, accKeeper, deposit)
	otherAddr := CreateFakeFundedAccount(ctx, accKeeper, deposit)

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	specs := map[string]struct {
		srcPermission types.AccessConfig
		expError      *sdkerrors.Error
	}{
		"default": {
			srcPermission: types.DefaultUploadAccess,
		},
		"everybody": {
			srcPermission: types.AllowEverybody,
		},
		"nobody": {
			srcPermission: types.AllowNobody,
			expError:      sdkerrors.ErrUnauthorized,
		},
		"onlyAddress with matching address": {
			srcPermission: types.OnlyAddress.With(creator),
		},
		"onlyAddress with non matching address": {
			srcPermission: types.OnlyAddress.With(otherAddr),
			expError:      sdkerrors.ErrUnauthorized,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			_, err := keeper.Create(ctx, creator, wasmCode, "https://github.com/CosmWasm/wasmd/blob/master/x/wasm/testdata/escrow.wasm", "any/builder:tag")
			require.True(t, spec.expError.Is(err), err)
			if spec.expError != nil {
				return
			}
		})
	}
}
*/

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

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
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
	storedCode, err := keeper.GetByteCode(ctx, contractID)
	require.NoError(t, err)
	require.Equal(t, wasmCode, storedCode)
	storedCode, err = keeper.GetByteCode(ctx, duplicateID)
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

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
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
	code, err := keeper.GetByteCode(ctx, contractID)
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

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm.gzip")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), contractID)
	// and verify content
	storedCode, err := keeper.GetByteCode(ctx, contractID)
	require.NoError(t, err)
	rawCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)
	require.Equal(t, rawCode, storedCode)
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

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "https://github.com/enigmampc/SecretNetwork/blob/master/cosmwasm/contracts/hackatom/src/contract.rs", "")
	require.NoError(t, err)

	_, _, bob := keyPubAddr()
	_, _, fred := keyPubAddr()

	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	key := keeper.GetCodeInfo(ctx, contractID).CodeHash

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
		Sender: creator,
		// Admin:     nil,
		CodeID:    contractID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}
	tx := NewTestTx(&instantiateMsg, creatorAcc, privKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)

	// create with no balance is also legal
	contractAddr, _, err := keeper.Instantiate(ctx, contractID, creator /* , nil */, initMsgBz, "demo contract 1", nil, nil)
	require.NoError(t, err)
	require.Equal(t, "secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg", contractAddr.String())

	gasAfter := ctx.GasMeter().GasConsumed()
	require.Greater(t, gasAfter-gasBefore, uint64(10000))
	require.Less(t, gasAfter-gasBefore, uint64(90000))

	// ensure it is stored properly
	info := keeper.GetContractInfo(ctx, contractAddr)
	require.NotNil(t, info)
	require.Equal(t, info.Creator, creator)
	require.Equal(t, info.CodeID, contractID)
	require.Equal(t, info.Label, "demo contract 1")

	// test that creating again with the same label will fail
	_, _, err = keeper.Instantiate(ctx, contractID, creator /* , nil */, initMsgBz, "demo contract 1", nil, nil)
	require.Error(t, err)

	/*
		exp := []types.ContractCodeHistoryEntry{{
			Operation: types.InitContractCodeHistoryType,
			CodeID:    contractID,
			Updated:   types.NewAbsoluteTxPosition(ctx),
			Msg:       json.RawMessage(initMsgBz),
		}}
		assert.Equal(t, exp, keeper.GetContractHistory(ctx, contractAddr))
	*/
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
			ctx, keeper, codeID, _, _, _, _, _ := setupTest(t, "./testdata/contract.wasm")

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
			addr, _, err := initHelperImpl(t, keeper, ctx, codeID, bob, bobPriv, string(initMsgBz), false, false, defaultGasForTests, wasmCalls, int64(deposit))
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

/*
func TestInstantiateWithPermissions(t *testing.T) {
	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	var (
		deposit   = sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
		myAddr    = bytes.Repeat([]byte{1}, sdk.AddrLen)
		otherAddr = bytes.Repeat([]byte{2}, sdk.AddrLen)
		anyAddr   = bytes.Repeat([]byte{3}, sdk.AddrLen)
	)

	initMsg := InitMsg{
		Verifier:    anyAddr,
		Beneficiary: anyAddr,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	specs := map[string]struct {
		srcPermission types.AccessConfig
		srcActor      sdk.AccAddress
		expError      *sdkerrors.Error
	}{
		"default": {
			srcPermission: types.DefaultUploadAccess,
			srcActor:      anyAddr,
		},
		"everybody": {
			srcPermission: types.AllowEverybody,
			srcActor:      anyAddr,
		},
		"nobody": {
			srcPermission: types.AllowNobody,
			srcActor:      myAddr,
			expError:      sdkerrors.ErrUnauthorized,
		},
		"onlyAddress with matching address": {
			srcPermission: types.OnlyAddress.With(myAddr),
			srcActor:      myAddr,
		},
		"onlyAddress with non matching address": {
			srcPermission: types.OnlyAddress.With(otherAddr),
			expError:      sdkerrors.ErrUnauthorized,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			tempDir, err := ioutil.TempDir("", "wasm")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
			accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
			fundAccounts(ctx, accKeeper, spec.srcActor, deposit)

			contractID, err := keeper.Create(ctx, myAddr, wasmCode, "https://github.com/CosmWasm/wasmd/blob/master/x/wasm/testdata/escrow.wasm", "")
			require.NoError(t, err)

			_,_, err = keeper.Instantiate(ctx, contractID, spec.srcActor, nil, initMsgBz, "demo contract 1", nil)
			assert.True(t, spec.expError.Is(err), "got %+v", err)
		})
	}
}
*/
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
		Sender: creator,
		// Admin:     nil,
		CodeID:    nonExistingCodeID,
		Label:     "demo contract 1",
		InitMsg:   initMsgBz,
		InitFunds: nil,
	}
	tx := NewTestTx(&instantiateMsg, creatorAcc, privKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)

	addr, _, err := keeper.Instantiate(ctx, nonExistingCodeID, creator /* , nil */, initMsgBz, "demo contract 2", nil, nil)
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

	key := keeper.GetCodeInfo(ctx, contractID).CodeHash
	// keyStr := hex.EncodeToString(key)

	msg := types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(key)),
		Msg:      initMsgBz,
	}

	initMsgBz, err = wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	gasBefore := ctx.GasMeter().GasConsumed()

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, creatorPrivKey, initMsgBz, contractID, deposit)
	// create with no balance is also legal
	addr, _, err := keeper.Instantiate(ctx, contractID, creator /* , nil */, initMsgBz, "demo contract 1", deposit, nil)

	require.NoError(t, err)

	require.Equal(t, "secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg", addr.String())

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

	_, _, _, trialExecErr := execHelper(t, keeper, trialCtx, addr, creator, creatorPrivKey, `{"release":{}}`, true, false, defaultGasForTests, 0)
	require.Error(t, trialExecErr)
	require.Error(t, trialExecErr.Unauthorized)
	require.Contains(t, trialExecErr.Error(), "unauthorized")

	// verifier can execute, and get proper gas amount
	start := time.Now()

	gasBefore = ctx.GasMeter().GasConsumed()

	require.NoError(t, err)
	// res, _, _, err := execHelper(t, keeper, trialCtx, addr, creator, `{"release":{}}`, true, false, defaultGasForTests)

	initMsgBz = []byte(`{"release":{}}`)

	key = keeper.GetCodeInfo(ctx, contractID).CodeHash
	// keyStr := hex.EncodeToString(key)

	msg = types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(key)),
		Msg:      initMsgBz,
	}

	msgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	ctx = PrepareExecSignedTx(t, keeper, ctx, fred, privFred, msgBz, addr, topUp)

	res, err := keeper.Execute(ctx, addr, fred, msgBz, topUp, nil)
	diff := time.Now().Sub(start)
	require.NoError(t, err)
	require.NotNil(t, res)

	// make sure gas is properly deducted from ctx
	gasAfter := ctx.GasMeter().GasConsumed()
	require.Greater(t, gasAfter-gasBefore, uint64(10000))
	require.Less(t, gasAfter-gasBefore, uint64(90000))

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
			ctx, keeper, codeID, _, _, _, _, _ := setupTest(t, "./testdata/contract.wasm")

			deposit := int64(100)
			var funds int64 = 0
			if spec.fundAddr {
				funds = 200
			}
			bob, bobPriv := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", funds)))
			fred, _ := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))

			initMsgBz, err := json.Marshal(InitMsg{Verifier: bob, Beneficiary: fred})
			require.NoError(t, err)

			contractAddr, _, err := initHelperImpl(t, keeper, ctx, codeID, bob, bobPriv, string(initMsgBz), true, false, defaultGasForTests, -1, 0)
			require.Empty(t, err)

			wasmCalls := int64(-1)
			if spec.expError {
				wasmCalls = 0
			}

			// when
			_, _, _, err = execHelperImpl(t, keeper, ctx, contractAddr, bob, bobPriv, `{"release":{}}`, false, false, defaultGasForTests, deposit, wasmCalls)

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
	nonExistingAddress := addrFromUint64(9999)
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

	_, err = keeper.Execute(ctx, nonExistingAddress, creator, msgBz, nil, nil)
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

	addr, _, err := initHelper(t, keeper, ctx, contractID, creator, creatorPrivKey, string(initMsgBz), false, false, defaultGasForTests)

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

	// let's make sure we get a reasonable error, no panic/crash
	_, err = keeper.Execute(ctx, addr, fred, execMsgBz, topUp, nil)
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

	hash := keeper.GetCodeInfo(ctx, contractID).CodeHash

	msg := types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(hash)),
		Msg:      initMsgBz,
	}

	msgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	creatorAcc, err := authante.GetSignerAcc(ctx, accKeeper, creator)
	require.NoError(t, err)

	instantiateMsg := types.MsgInstantiateContract{
		Sender: creator,
		// Admin:     nil,
		CodeID:    contractID,
		Label:     "demo contract 1",
		InitMsg:   msgBz,
		InitFunds: deposit,
	}
	tx := NewTestTx(&instantiateMsg, creatorAcc, creatorPrivKey)

	txBytes, err := tx.Marshal()
	require.NoError(t, err)

	ctx = ctx.WithTxBytes(txBytes)

	addr, _, err := keeper.Instantiate(ctx, contractID, creator /* , nil */, msgBz, "demo contract 5", deposit, nil)
	require.NoError(t, err)

	// make sure we set a limit before calling
	var gasLimit uint64 = 400_000
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(gasLimit))
	require.Equal(t, uint64(0), ctx.GasMeter().GasConsumed())

	codeHash := keeper.GetContractHash(ctx, addr)
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

	// this must fail
	_, err = keeper.Execute(ctx, addr, fred, execMsgBz, nil, nil)
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

	addr, _, err := initHelper(t, keeper, ctx, contractID, creator, creatorPrivKey, string(initMsgBz), false, false, defaultGasForTests)

	// make sure we set a limit before calling
	var gasLimit uint64 = 400_002
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(gasLimit))
	require.Equal(t, uint64(0), ctx.GasMeter().GasConsumed())

	// ensure we get an out of gas panic
	defer func() {
		r := recover()
		require.NotNil(t, r)
		_, ok := r.(sdk.ErrorOutOfGas)
		require.True(t, ok, "%+v", r)
	}()

	codeHash := keeper.GetContractHash(ctx, addr)
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

	// this should throw out of gas exception (panic)
	_, err = keeper.Execute(ctx, addr, fred, msgBz, nil, nil)
	require.True(t, false, "We must panic before this line")
}

/*
func TestMigrate(t *testing.T) {
	t.SkipNow() // secret network does not support migrate
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, _ := CreateFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	fred, _ := CreateFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 5000)))

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	originalCodeID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	newCodeID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.NotEqual(t, originalCodeID, newCodeID)

	_, _, anyAddr := keyPubAddr()
	_, _, newVerifierAddr := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: anyAddr,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)
	initMsgBz, err = wasmCtx.Encrypt(initMsgBz)
	require.NoError(t, err)

	migMsg := struct {
		Verifier sdk.AccAddress `json:"verifier"`
	}{Verifier: newVerifierAddr}
	migMsgBz, err := json.Marshal(migMsg)
	require.NoError(t, err)

	specs := map[string]struct {
		admin                sdk.AccAddress
		overrideContractAddr sdk.AccAddress
		caller               sdk.AccAddress
		codeID               uint64
		migrateMsg           []byte
		expErr               *sdkerrors.Error
		expVerifier          sdk.AccAddress
	}{
		"all good with same code id": {
			admin:       creator,
			caller:      creator,
			codeID:      originalCodeID,
			migrateMsg:  migMsgBz,
			expVerifier: newVerifierAddr,
		},
		"all good with different code id": {
			admin:       creator,
			caller:      creator,
			codeID:      newCodeID,
			migrateMsg:  migMsgBz,
			expVerifier: newVerifierAddr,
		},
		"all good with admin set": {
			admin:       fred,
			caller:      fred,
			codeID:      newCodeID,
			migrateMsg:  migMsgBz,
			expVerifier: newVerifierAddr,
		},
		"prevent migration when admin was not set on instantiate": {
			caller: creator,
			codeID: originalCodeID,
			expErr: sdkerrors.ErrUnauthorized,
		},
		"prevent migration when not sent by admin": {
			caller: creator,
			admin:  fred,
			codeID: originalCodeID,
			expErr: sdkerrors.ErrUnauthorized,
		},
		"fail with non existing code id": {
			admin:  creator,
			caller: creator,
			codeID: 99999,
			expErr: sdkerrors.ErrInvalidRequest,
		},
		"fail with non existing contract addr": {
			admin:                creator,
			caller:               creator,
			overrideContractAddr: anyAddr,
			codeID:               originalCodeID,
			expErr:               sdkerrors.ErrInvalidRequest,
		},
		"fail in contract with invalid migrate msg": {
			admin:      creator,
			caller:     creator,
			codeID:     originalCodeID,
			migrateMsg: bytes.Repeat([]byte{0x1}, 7),
			expErr:     types.ErrMigrationFailed,
		},
		"fail in contract without migrate msg": {
			admin:  creator,
			caller: creator,
			codeID: originalCodeID,
			expErr: types.ErrMigrationFailed,
		},
	}

	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
			contractAddr, err := keeper.Instantiate(ctx, originalCodeID, creator, spec.admin, initMsgBz, "demo contract", nil, nil)
			require.NoError(t, err)
			if spec.overrideContractAddr != nil {
				contractAddr = spec.overrideContractAddr
			}
			_, err = keeper.Migrate(ctx, contractAddr, spec.caller, spec.codeID, spec.migrateMsg)
			require.True(t, spec.expErr.Is(err), "expected %v but got %+v", spec.expErr, err)
			if spec.expErr != nil {
				return
			}
			cInfo := keeper.GetContractInfo(ctx, contractAddr)
			assert.Equal(t, spec.codeID, cInfo.CodeID)

			expHistory := []types.ContractCodeHistoryEntry{{
				Operation: types.InitContractCodeHistoryType,
				CodeID:    originalCodeID,
				Updated:   types.NewAbsoluteTxPosition(ctx),
				Msg:       initMsgBz,
			}, {
				Operation: types.MigrateContractCodeHistoryType,
				CodeID:    spec.codeID,
				Updated:   types.NewAbsoluteTxPosition(ctx),
				Msg:       spec.migrateMsg,
			}}
			assert.Equal(t, expHistory, keeper.GetContractHistory(ctx, contractAddr))

			m := keeper.QueryRaw(ctx, contractAddr, []byte("config"))
			require.Len(t, m, 1)
			var stored map[string][]byte
			require.NoError(t, json.Unmarshal(m[0].Value, &stored))
			require.Contains(t, stored, "verifier")
			require.NoError(t, err)
			assert.Equal(t, spec.expVerifier, sdk.AccAddress(stored["verifier"]))
		})
	}
}

func TestMigrateWithDispatchedMessage(t *testing.T) {
	t.SkipNow() // secret network does not support migrate
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, _ := CreateFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	fred, _ := CreateFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 5000)))

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)
	burnerCode, err := ioutil.ReadFile("./testdata/burner.wasm")
	require.NoError(t, err)

	originalContractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	burnerContractID, err := keeper.Create(ctx, creator, burnerCode, "", "")
	require.NoError(t, err)
	require.NotEqual(t, originalContractID, burnerContractID)

	_, _, myPayoutAddr := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: fred,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)
	initMsgBz, err = wasmCtx.Encrypt(initMsgBz)
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	contractAddr, err := keeper.Instantiate(ctx, originalContractID, creator, fred, initMsgBz, "demo contract", deposit, nil)
	require.NoError(t, err)

	migMsg := struct {
		Payout sdk.AccAddress `json:"payout"`
	}{Payout: myPayoutAddr}
	migMsgBz, err := json.Marshal(migMsg)
	require.NoError(t, err)
	ctx = ctx.WithEventManager(sdk.NewEventManager()).WithBlockHeight(ctx.BlockHeight() + 1)
	res, err := keeper.Migrate(ctx, contractAddr, fred, burnerContractID, migMsgBz)
	require.NoError(t, err)
	assert.Equal(t, "burnt 1 keys", string(res.Data))
	assert.Equal(t, "", res.Log)
	type dict map[string]interface{}
	expEvents := []dict{
		{
			"Type": "wasm",
			"Attr": []dict{
				{"contract_address": contractAddr},
				{"action": "burn"},
				{"payout": myPayoutAddr},
			},
		},
		{
			"Type": "transfer",
			"Attr": []dict{
				{"recipient": myPayoutAddr},
				{"sender": contractAddr},
				{"amount": "100000denom"},
			},
		},
		{
			"Type": "message",
			"Attr": []dict{
				{"sender": contractAddr},
			},
		},
		{
			"Type": "message",
			"Attr": []dict{
				{"module": "bank"},
			},
		},
	}
	expJsonEvts := string(mustMarshal(t, expEvents))
	assert.JSONEq(t, expJsonEvts, prettyEvents(t, ctx.EventManager().Events()))

	// all persistent data cleared
	m := keeper.QueryRaw(ctx, contractAddr, []byte("config"))
	require.Len(t, m, 0)

	// and all deposit tokens sent to myPayoutAddr
	balance := accKeeper.GetAccount(ctx, myPayoutAddr).GetCoins()
	assert.Equal(t, deposit, balance)
}
*/

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

/*
func TestUpdateContractAdmin(t *testing.T) {
	t.SkipNow() // secret network does not support migrate
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator, _ := CreateFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	fred, _ := CreateFakeFundedAccount(ctx, accKeeper, topUp)

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	originalContractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	_, _, anyAddr := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: anyAddr,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)
	initMsgBz, err = wasmCtx.Encrypt(initMsgBz)
	require.NoError(t, err)
	specs := map[string]struct {
		instAdmin            sdk.AccAddress
		newAdmin             sdk.AccAddress
		overrideContractAddr sdk.AccAddress
		caller               sdk.AccAddress
		expErr               *sdkerrors.Error
	}{
		"all good with admin set": {
			instAdmin: fred,
			newAdmin:  anyAddr,
			caller:    fred,
		},
		"prevent update when admin was not set on instantiate": {
			caller:   creator,
			newAdmin: fred,
			expErr:   sdkerrors.ErrUnauthorized,
		},
		"prevent updates from non admin address": {
			instAdmin: creator,
			newAdmin:  fred,
			caller:    fred,
			expErr:    sdkerrors.ErrUnauthorized,
		},
		"fail with non existing contract addr": {
			instAdmin:            creator,
			newAdmin:             anyAddr,
			caller:               creator,
			overrideContractAddr: anyAddr,
			expErr:               sdkerrors.ErrInvalidRequest,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			addr, err := keeper.Instantiate(ctx, originalContractID, creator, spec.instAdmin, initMsgBz, "demo contract", nil, nil)
			require.NoError(t, err)
			if spec.overrideContractAddr != nil {
				addr = spec.overrideContractAddr
			}
			err = keeper.UpdateContractAdmin(ctx, addr, spec.caller, spec.newAdmin)
			require.True(t, spec.expErr.Is(err), "expected %v but got %+v", spec.expErr, err)
			if spec.expErr != nil {
				return
			}
			cInfo := keeper.GetContractInfo(ctx, addr)
			assert.Equal(t, spec.newAdmin, cInfo.Admin)
		})
	}
}

func TestClearContractAdmin(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator := CreateFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	fred := CreateFakeFundedAccount(ctx, accKeeper, topUp)

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	originalContractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	_, _, anyAddr := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: anyAddr,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)
	specs := map[string]struct {
		instAdmin            sdk.AccAddress
		overrideContractAddr sdk.AccAddress
		caller               sdk.AccAddress
		expErr               *sdkerrors.Error
	}{
		"all good when called by proper admin": {
			instAdmin: fred,
			caller:    fred,
		},
		"prevent update when admin was not set on instantiate": {
			caller: creator,
			expErr: sdkerrors.ErrUnauthorized,
		},
		"prevent updates from non admin address": {
			instAdmin: creator,
			caller:    fred,
			expErr:    sdkerrors.ErrUnauthorized,
		},
		"fail with non existing contract addr": {
			instAdmin:            creator,
			caller:               creator,
			overrideContractAddr: anyAddr,
			expErr:               sdkerrors.ErrInvalidRequest,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			addr, err := keeper.Instantiate(ctx, originalContractID, creator, spec.instAdmin, initMsgBz, "demo contract", nil)
			require.NoError(t, err)
			if spec.overrideContractAddr != nil {
				addr = spec.overrideContractAddr
			}
			err = keeper.ClearContractAdmin(ctx, addr, spec.caller)
			require.True(t, spec.expErr.Is(err), "expected %v but got %+v", spec.expErr, err)
			if spec.expErr != nil {
				return
			}
			cInfo := keeper.GetContractInfo(ctx, addr)
			assert.Empty(t, cInfo.Admin)
		})
	}
}
*/

type InitMsg struct {
	Verifier    sdk.AccAddress `json:"verifier"`
	Beneficiary sdk.AccAddress `json:"beneficiary"`
}

type InstantiateMsg struct {
	Counter uint64 `json:"counter"`
	Expires uint64 `json:"expires"`
}
