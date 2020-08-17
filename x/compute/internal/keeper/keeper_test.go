package keeper

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enigmampc/SecretNetwork/go-cosmwasm/api"
	eng "github.com/enigmampc/SecretNetwork/types"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	stypes "github.com/enigmampc/cosmos-sdk/store/types"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	sdkerrors "github.com/enigmampc/cosmos-sdk/types/errors"
	"github.com/enigmampc/cosmos-sdk/x/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"

	wasmUtils "github.com/enigmampc/SecretNetwork/x/compute/client/utils"
	reg "github.com/enigmampc/SecretNetwork/x/registration"
)

const SupportedFeatures = "staking"

var wasmCtx = wasmUtils.WASMContext{
	TestKeyPairPath:  "/tmp/id_tx_io.json",
	TestMasterIOCert: nil,
}

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(eng.Bech32PrefixAccAddr, eng.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(eng.Bech32PrefixValAddr, eng.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(eng.Bech32PrefixConsAddr, eng.Bech32PrefixConsPub)
	config.Seal()

	_, err := api.InitBootstrap()
	if err != nil {
		panic(fmt.Sprintf("Error initializing the enclave: %v", err))
	}

	wasmCtx.TestMasterIOCert, err = ioutil.ReadFile(filepath.Join(".", reg.IoExchMasterCertPath))
	if err != nil {
		panic(fmt.Sprintf("Error reading 'io-master-cert.der': %v", err))
	}
}

func TestNewKeeper(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	_, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	require.NotNil(t, keepers.WasmKeeper)
}

func TestCreate(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit)

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

func TestCreateDuplicate(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit)

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
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	ctx = ctx.WithBlockHeader(abci.Header{Height: 1}).
		WithGasMeter(stypes.NewInfiniteGasMeter())

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit)

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	// create this once in simulation mode
	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), contractID)

	// then try to create it in non-simulation mode (should not fail)
	ctx, keepers = CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
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
			ctx: sdk.Context{}.WithBlockHeader(abci.Header{}).WithGasMeter(stypes.NewInfiniteGasMeter()),
			exp: false,
		},
		"any regular block": {
			ctx: sdk.Context{}.WithBlockHeader(abci.Header{Height: 1}).WithGasMeter(stypes.NewGasMeter(10000000)),
			exp: false,
		},
		"simulation": {
			ctx: sdk.Context{}.WithBlockHeader(abci.Header{Height: 1}).WithGasMeter(stypes.NewInfiniteGasMeter()),
			exp: true,
		},
	}
	for msg, _ := range specs {
		t.Run(msg, func(t *testing.T) {
			//require.Equal(t, spec.exp, isSimulationMode(spec.ctx))
		})
	}
}

func TestCreateWithGzippedPayload(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit)

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
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit)

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
	//keyStr := hex.EncodeToString(key)

	msg := types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(key)),
		Msg:      initMsgBz,
	}

	initMsgBz, err = wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	gasBefore := ctx.GasMeter().GasConsumed()

	// create with no balance is also legal
	addr, err := keeper.Instantiate(ctx, contractID, creator, nil, initMsgBz, "demo contract 1", nil)
	require.NoError(t, err)
	require.Equal(t, "secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg", addr.String())

	gasAfter := ctx.GasMeter().GasConsumed()
	require.Greater(t, gasAfter-gasBefore, uint64(20000))
	require.Less(t, gasAfter-gasBefore, uint64(60000))

	// ensure it is stored properly
	info := keeper.GetContractInfo(ctx, addr)
	require.NotNil(t, info)
	require.Equal(t, info.Creator, creator)
	require.Equal(t, info.CodeID, contractID)
	require.Equal(t, info.InitMsg, initMsgBz)
	require.Equal(t, info.Label, "demo contract 1")

	// test that creating again with the same label will fail
	addr, err = keeper.Instantiate(ctx, contractID, creator, nil, initMsgBz, "demo contract 1", nil)
	require.Error(t, err)
}

func TestInstantiateWithNonExistingCodeID(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit)

	require.NoError(t, err)

	initMsg := InitMsg{}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	initMsgBz, err = wasmCtx.Encrypt(initMsgBz)
	require.NoError(t, err)

	const nonExistingCodeID = 9999
	addr, err := keeper.Instantiate(ctx, nonExistingCodeID, creator, nil, initMsgBz, "demo contract 2", nil)
	require.True(t, types.ErrNotFound.Is(err), err)
	require.Nil(t, addr)
}

func TestExecute(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

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

	key := keeper.GetCodeInfo(ctx, contractID).CodeHash
	//keyStr := hex.EncodeToString(key)

	msg := types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(key)),
		Msg:      initMsgBz,
	}

	initMsgBz, err = wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	gasBefore := ctx.GasMeter().GasConsumed()

	// create with no balance is also legal
	addr, err := keeper.Instantiate(ctx, contractID, creator, nil, initMsgBz, "demo contract 1", deposit)

	require.NoError(t, err)

	require.Equal(t, "secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg", addr.String())

	// ensure bob doesn't exist
	bobAcct := accKeeper.GetAccount(ctx, bob)
	require.Nil(t, bobAcct)

	// ensure funder has reduced balance
	creatorAcct := accKeeper.GetAccount(ctx, creator)
	require.NotNil(t, creatorAcct)
	// we started at 2*deposit, should have spent one above
	assert.Equal(t, deposit, creatorAcct.GetCoins())

	// ensure contract has updated balance
	contractAcct := accKeeper.GetAccount(ctx, addr)
	require.NotNil(t, contractAcct)
	assert.Equal(t, deposit, contractAcct.GetCoins())

	// unauthorized - trialCtx so we don't change state
	trialCtx := ctx.WithMultiStore(ctx.MultiStore().CacheWrap().(sdk.MultiStore))

	_, _, trialExecErr := execHelper(t, keeper, trialCtx, addr, creator, `{"release":{}}`, true, defaultGasForTests, 0)
	require.Error(t, trialExecErr)
	require.Error(t, trialExecErr.Unauthorized)
	require.Contains(t, trialExecErr.Error(), "unauthorized")

	// verifier can execute, and get proper gas amount
	start := time.Now()

	gasBefore = ctx.GasMeter().GasConsumed()

	require.NoError(t, err)
	//res, _, err := execHelper(t, keeper, trialCtx, addr, creator, `{"release":{}}`, true, defaultGasForTests)

	initMsgBz = []byte(`{"release":{}}`)

	key = keeper.GetCodeInfo(ctx, contractID).CodeHash
	//keyStr := hex.EncodeToString(key)

	msg = types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(key)),
		Msg:      initMsgBz,
	}


	msgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	res, err := keeper.Execute(ctx, addr, fred, msgBz, topUp)

	diff := time.Now().Sub(start)
	require.NoError(t, err)
	require.NotNil(t, res)

	// make sure gas is properly deducted from ctx
	gasAfter := ctx.GasMeter().GasConsumed()
	require.Greater(t, gasAfter-gasBefore, uint64(25000))
	require.Less(t, gasAfter-gasBefore, uint64(50000))

	// ensure bob now exists and got both payments released
	bobAcct = accKeeper.GetAccount(ctx, bob)
	require.NotNil(t, bobAcct)
	balance := bobAcct.GetCoins()
	assert.Equal(t, deposit.Add(topUp...), balance)

	// ensure contract has updated balance
	contractAcct = accKeeper.GetAccount(ctx, addr)
	require.NotNil(t, contractAcct)
	assert.Equal(t, sdk.Coins(nil), contractAcct.GetCoins())

	t.Logf("Duration: %v (%d gas)\n", diff, gasAfter-gasBefore)
}

func TestExecuteWithNonExistingAddress(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))

	// unauthorized - trialCtx so we don't change state
	nonExistingAddress := addrFromUint64(9999)
	msgBz, err := wasmCtx.Encrypt([]byte(`{}`))
	require.NoError(t, err)
	_, err = keeper.Execute(ctx, nonExistingAddress, creator, msgBz, nil)
	require.True(t, types.ErrNotFound.Is(err), err)
}

func TestExecuteWithPanic(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

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

	addr, _, err := initHelper(t, keeper, ctx, contractID, creator, string(initMsgBz), false, defaultGasForTests)

	execMsgBz, err := wasmCtx.Encrypt([]byte(`{"panic":{}}`))
	require.NoError(t, err)

	// let's make sure we get a reasonable error, no panic/crash
	_, err = keeper.Execute(ctx, addr, fred, execMsgBz, topUp)
	require.Error(t, err)
}

func TestExecuteWithCpuLoop(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

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

	hash := keeper.GetCodeInfo(ctx, contractID).CodeHash

	msg := types.SecretMsg{
		CodeHash: []byte(hex.EncodeToString(hash)),
		Msg:      initMsgBz,
	}

	msgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	addr, err := keeper.Instantiate(ctx, contractID, creator, nil, msgBz, "demo contract 5", deposit)
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
		require.True(t, ok, "%v", r)
	}()

	// this must fail
	_, err = keeper.Execute(ctx, addr, fred, execMsgBz, nil)
	assert.True(t, false)
	// make sure gas ran out
	// TODO: wasmer doesn't return gas used on error. we should consume it (for error on metering failure)
	// require.Equal(t, gasLimit, ctx.GasMeter().GasConsumed())
}

func TestExecuteWithStorageLoop(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

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

	addr, _, err := initHelper(t, keeper, ctx, contractID, creator, string(initMsgBz), false, defaultGasForTests)

	// make sure we set a limit before calling
	var gasLimit uint64 = 400_000
	ctx = ctx.WithGasMeter(sdk.NewGasMeter(gasLimit))
	require.Equal(t, uint64(0), ctx.GasMeter().GasConsumed())

	// ensure we get an out of gas panic
	defer func() {
		r := recover()
		require.NotNil(t, r)
		_, ok := r.(sdk.ErrorOutOfGas)
		require.True(t, ok, "%v", r)
	}()

	codeHash := keeper.GetContractHash(ctx, addr)
	codeHashStr := hex.EncodeToString(codeHash)

	msg := types.SecretMsg{
		CodeHash: []byte(codeHashStr),
		Msg:      []byte(`{"storage_loop":{}}`),
	}

	msgBz, err := wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	// this should throw out of gas exception (panic)
	_, err = keeper.Execute(ctx, addr, fred, msgBz, nil)
	require.True(t, false, "We must panic before this line")
}

func TestMigrate(t *testing.T) {
	t.SkipNow() // secret network does not support migrate
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	fred := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 5000)))

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	originalContractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	newContractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)
	require.NotEqual(t, originalContractID, newContractID)

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
			codeID:      originalContractID,
			migrateMsg:  migMsgBz,
			expVerifier: newVerifierAddr,
		},
		"all good with different code id": {
			admin:       creator,
			caller:      creator,
			codeID:      newContractID,
			migrateMsg:  migMsgBz,
			expVerifier: newVerifierAddr,
		},
		"all good with admin set": {
			admin:       fred,
			caller:      fred,
			codeID:      newContractID,
			migrateMsg:  migMsgBz,
			expVerifier: newVerifierAddr,
		},
		"prevent migration when admin was not set on instantiate": {
			caller: creator,
			codeID: originalContractID,
			expErr: sdkerrors.ErrUnauthorized,
		},
		"prevent migration when not sent by admin": {
			caller: creator,
			admin:  fred,
			codeID: originalContractID,
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
			codeID:               originalContractID,
			expErr:               sdkerrors.ErrInvalidRequest,
		},
		"fail in contract with invalid migrate msg": {
			admin:      creator,
			caller:     creator,
			codeID:     originalContractID,
			migrateMsg: bytes.Repeat([]byte{0x1}, 7),
			expErr:     types.ErrMigrationFailed,
		},
		"fail in contract without migrate msg": {
			admin:  creator,
			caller: creator,
			codeID: originalContractID,
			expErr: types.ErrMigrationFailed,
		},
	}

	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
			addr, err := keeper.Instantiate(ctx, originalContractID, creator, spec.admin, initMsgBz, "demo contract", nil)
			require.NoError(t, err)
			if spec.overrideContractAddr != nil {
				addr = spec.overrideContractAddr
			}
			_, err = keeper.Migrate(ctx, addr, spec.caller, spec.codeID, spec.migrateMsg)
			require.True(t, spec.expErr.Is(err), "expected %v but got %+v", spec.expErr, err)
			if spec.expErr != nil {
				return
			}
			cInfo := keeper.GetContractInfo(ctx, addr)
			assert.Equal(t, spec.codeID, cInfo.CodeID)
			assert.Equal(t, originalContractID, cInfo.PreviousCodeID)
			assert.Equal(t, types.NewCreatedAt(ctx), cInfo.LastUpdated)

			m := keeper.QueryRaw(ctx, addr, []byte("config"))
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
	creator := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	fred := createFakeFundedAccount(ctx, accKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 5000)))

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
	contractAddr, err := keeper.Instantiate(ctx, originalContractID, creator, fred, initMsgBz, "demo contract", deposit)
	require.NoError(t, err)

	migMsg := struct {
		Payout sdk.AccAddress `json:"payout"`
	}{Payout: myPayoutAddr}
	migMsgBz, err := json.Marshal(migMsg)
	require.NoError(t, err)
	ctx = ctx.WithEventManager(sdk.NewEventManager()).WithBlockHeight(ctx.BlockHeight() + 1)
	res, err := keeper.Migrate(ctx, contractAddr, fred, burnerContractID, migMsgBz)
	require.NoError(t, err)
	dataBz, err := base64.StdEncoding.DecodeString(string(res.Data))
	require.NoError(t, err)
	assert.Equal(t, "burnt", string(dataBz))
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

func TestUpdateContractAdmin(t *testing.T) {
	t.SkipNow() // secret network does not support migrate
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keepers := CreateTestInput(t, false, tempDir, SupportedFeatures, nil, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit.Add(deposit...))
	fred := createFakeFundedAccount(ctx, accKeeper, topUp)

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
		"all good with new admin empty": {
			instAdmin: fred,
			newAdmin:  nil,
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

type InitMsg struct {
	Verifier    sdk.AccAddress `json:"verifier"`
	Beneficiary sdk.AccAddress `json:"beneficiary"`
}

func createFakeFundedAccount(ctx sdk.Context, am auth.AccountKeeper, coins sdk.Coins) sdk.AccAddress {
	_, _, addr := keyPubAddr()
	baseAcct := auth.NewBaseAccountWithAddress(addr)
	_ = baseAcct.SetCoins(coins)
	am.SetAccount(ctx, &baseAcct)

	return addr
}

var keyCounter uint64 = 0

// we need to make this deterministic (same every test run), as encoded address size and thus gas cost,
// depends on the actual bytes (due to ugly CanonicalAddress encoding)
func keyPubAddr() (crypto.PrivKey, crypto.PubKey, sdk.AccAddress) {
	keyCounter++
	seed := make([]byte, 8)
	binary.BigEndian.PutUint64(seed, keyCounter)

	key := ed25519.GenPrivKeyFromSecret(seed)
	pub := key.PubKey()
	addr := sdk.AccAddress(pub.Address())
	return key, pub, addr
}
