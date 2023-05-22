package keeper

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"

	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

func TestQueryContractLabel(t *testing.T) {
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
	creator, privCreator := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))
	anyAddr, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, topUp)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	_, _, bob := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    anyAddr,
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

	initMsgBz, err = wasmCtx.Encrypt(msg.Serialize())
	require.NoError(t, err)

	label := "banana"

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, privCreator, initMsgBz, contractID, deposit)

	addr, _, err := keeper.Instantiate(ctx, contractID, creator, nil, initMsgBz, label, deposit, nil)
	require.NoError(t, err)

	// this gets us full error, not redacted sdk.Error
	q := NewLegacyQuerier(keeper)
	specs := map[string]struct {
		srcPath []string
		srcReq  abci.RequestQuery
		// smart queries return raw bytes from contract not []types.Model
		// if this is set, then we just compare - (should be json encoded string)
		expSmartRes string
		// if success and expSmartRes is not set, we parse into []types.Model and compare
		expModelLen      int
		expModelContains []types.Model
		expErr           *sdkErrors.Error
	}{
		"query label available": {
			srcPath: []string{QueryContractAddress, "banananana"},
			srcReq:  abci.RequestQuery{},
			expErr:  sdkErrors.ErrUnknownAddress,
		},
		"query label exists": {
			srcPath:     []string{QueryContractAddress, label},
			srcReq:      abci.RequestQuery{},
			expSmartRes: string(addr),
		},
	}

	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			// binResult, err := q(ctx, spec.srcPath, spec.srcReq)
			binResult, err := q(ctx, spec.srcPath, spec.srcReq)
			// require.True(t, spec.expErr.Is(err), "unexpected error")
			require.True(t, spec.expErr.Is(err), err)

			// if smart query, check custom response
			if spec.expSmartRes != "" {
				require.Equal(t, spec.expSmartRes, string(binResult))
				return
			}

			// otherwise, check returned models
			var r []types.Model
			if spec.expErr == nil {
				require.NoError(t, json.Unmarshal(binResult, &r))
				require.NotNil(t, r)
			}
			require.Len(t, r, spec.expModelLen)
			// and in result set
			for _, v := range spec.expModelContains {
				require.Contains(t, r, v)
			}
		})
	}
}

func TestQueryContractState(t *testing.T) {
	t.SkipNow() // cannot interact directly with state

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
	creator, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))
	anyAddr, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, topUp)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
	require.NoError(t, err)

	contractID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	_, _, bob := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    anyAddr,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, contractID)
	require.NoError(t, err)

	key := codeInfo.CodeHash
	keyStr := hex.EncodeToString(key)

	msg := types.SecretMsg{
		CodeHash: []byte(keyStr),
		Msg:      initMsgBz,
	}

	initMsgBz, err = wasmCtx.Encrypt(msg.Serialize())

	addr, _, err := keeper.Instantiate(ctx, contractID, creator, nil, initMsgBz, "demo contract to query", deposit, nil)
	require.NoError(t, err)

	contractModel := []types.Model{
		{Key: []byte("foo"), Value: []byte(`"bar"`)},
		{Key: []byte{0x0, 0x1}, Value: []byte(`{"count":8}`)},
	}
	keeper.importContractState(ctx, addr, contractModel)

	// this gets us full error, not redacted sdk.Error
	q := NewLegacyQuerier(keeper)
	specs := map[string]struct {
		srcPath []string
		srcReq  abci.RequestQuery
		// smart queries return raw bytes from contract not []types.Model
		// if this is set, then we just compare - (should be json encoded string)
		expSmartRes string
		// if success and expSmartRes is not set, we parse into []types.Model and compare
		expModelLen      int
		expModelContains []types.Model
		expErr           *sdkErrors.Error
	}{
		"query": {
			srcPath:     []string{QueryGetContractState, addr.String()},
			srcReq:      abci.RequestQuery{Data: []byte(`{"verifier":{}}`)},
			expSmartRes: fmt.Sprintf(`{"verifier":"%s"}`, anyAddr.String()),
		},
		"query invalid request": {
			srcPath: []string{QueryGetContractState, addr.String()},
			srcReq:  abci.RequestQuery{Data: []byte(`{"raw":{"key":"config"}}`)},
			expErr:  types.ErrQueryFailed,
		},
		/*
			"query raw key": {
				srcPath:          []string{QueryGetContractState, addr.String(), QueryMethodContractStateRaw},
				srcReq:           abci.RequestQuery{Data: []byte("foo")},
				expModelLen:      1,
				expModelContains: []types.Model{{Key: []byte("foo"), Value: []byte(`"bar"`)}},
			},
			"query raw binary key": {
				srcPath:          []string{QueryGetContractState, addr.String(), QueryMethodContractStateRaw},
				srcReq:           abci.RequestQuery{Data: []byte{0x0, 0x1}},
				expModelLen:      1,
				expModelContains: []types.Model{{Key: []byte{0x0, 0x1}, Value: []byte(`{"count":8}`)}},
			},
		*/
		"query smart": {
			srcPath:     []string{QueryGetContractState, addr.String(), QueryMethodContractStateSmart},
			srcReq:      abci.RequestQuery{Data: []byte(`{"verifier":{}}`)},
			expSmartRes: fmt.Sprintf(`{"verifier":"%s"}`, anyAddr.String()),
		},
		"query smart invalid request": {
			srcPath: []string{QueryGetContractState, addr.String(), QueryMethodContractStateSmart},
			srcReq:  abci.RequestQuery{Data: []byte(`{"raw":{"key":"config"}}`)},
			expErr:  types.ErrQueryFailed,
		},
		"query smart with invalid json": {
			srcPath: []string{QueryGetContractState, addr.String(), QueryMethodContractStateSmart},
			srcReq:  abci.RequestQuery{Data: []byte(`not a json string`)},
			expErr:  types.ErrQueryFailed,
		},
		/*
			"query unknown raw key": {
				srcPath:     []string{QueryGetContractState, addr.String(), QueryMethodContractStateRaw},
				srcReq:      abci.RequestQuery{Data: []byte("unknown")},
				expModelLen: 0,
			},
		*/
		"query with unknown address": {
			srcPath:     []string{QueryGetContractState, anyAddr.String()},
			expModelLen: 0,
			expErr:      types.ErrNotFound,
		},
	}

	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			binResult, err := q(ctx, spec.srcPath, spec.srcReq)
			// require.True(t, spec.expErr.Is(err), "unexpected error")
			require.True(t, spec.expErr.Is(err), err)

			// if smart query, check custom response
			if spec.expSmartRes != "" {
				require.Equal(t, spec.expSmartRes, string(binResult))
				return
			}

			// otherwise, check returned models
			var r []types.Model
			if spec.expErr == nil {
				require.NoError(t, json.Unmarshal(binResult, &r))
				require.NotNil(t, r)
			}
			require.Len(t, r, spec.expModelLen)
			// and in result set
			for _, v := range spec.expModelContains {
				require.Contains(t, r, v)
			}
		})
	}
}

func TestListContractByCodeOrdering(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 1000000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 500))
	creator, creatorPrivKey := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)
	anyAddr, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, topUp)

	wasmCode, err := os.ReadFile(TestContractPaths[hackAtomContract])
	require.NoError(t, err)

	codeID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	_, _, bob := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    anyAddr,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, codeID)
	require.NoError(t, err)

	key := codeInfo.CodeHash
	keyStr := hex.EncodeToString(key)

	msg := types.SecretMsg{
		CodeHash: []byte(keyStr),
		Msg:      initMsgBz,
	}

	initMsgBz, err = wasmCtx.Encrypt(msg.Serialize())

	// manage some realistic block settings
	var h int64 = 10
	setBlock := func(ctx sdk.Context, height int64, wasmKeeper Keeper) sdk.Context {
		ctx = ctx.WithBlockHeight(height)
		meter := sdk.NewGasMeter(1000000)
		ctx = ctx.WithGasMeter(meter)
		ctx = ctx.WithBlockGasMeter(meter)
		// StoreRandomOnNewBlock(ctx, wasmKeeper)
		return ctx
	}

	// create 10 contracts with real block/gas setup
	for i := range [10]int{} {
		// 3 tx per block, so we ensure both comparisons work
		if i%3 == 0 {
			ctx = setBlock(ctx, h, keeper)
			h++
		}
		creatorAcc, err := authante.GetSignerAcc(ctx, accKeeper, creator)
		require.NoError(t, err)

		instantiateMsg := types.MsgInstantiateContract{
			Sender:    creator,
			CodeID:    codeID,
			Label:     fmt.Sprintf("contract %d", i),
			InitMsg:   initMsgBz,
			InitFunds: topUp,
		}
		tx := NewTestTx(&instantiateMsg, creatorAcc, creatorPrivKey)

		txBytes, err := tx.Marshal()
		require.NoError(t, err)

		ctx = ctx.WithTxBytes(txBytes)

		_, _, err = keeper.Instantiate(ctx, codeID, creator, nil, initMsgBz, fmt.Sprintf("contract %d", i), topUp, nil)
		require.NoError(t, err)
	}

	// query and check the results are properly sorted
	q := NewLegacyQuerier(keeper)
	query := []string{QueryListContractByCode, fmt.Sprintf("%d", codeID)}
	data := abci.RequestQuery{}
	res, err := q(ctx, query, data)
	require.NoError(t, err)

	var contracts []types.ContractInfoWithAddress
	err = json.Unmarshal(res, &contracts)
	require.NoError(t, err)

	require.Equal(t, 10, len(contracts))

	for i, contract := range contracts {
		require.Equal(t, fmt.Sprintf("contract %d", i), contract.Label)
		require.NotEmpty(t, contract.ContractAddress)
		// ensure these are not shown
		// assert.Nil(t, contract.InitMsg)
		assert.Nil(t, contract.Created)
	}
}
