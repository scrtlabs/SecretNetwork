package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/CosmWasm/wasmd/x/wasm/internal/types"
	wasmTypes "github.com/CosmWasm/wasmd/x/wasm/internal/types"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/staking"
	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"
)

func TestGenesisExportImport(t *testing.T) {
	srcKeeper, srcCtx, srcStoreKeys, srcCleanup := setupKeeper(t)
	defer srcCleanup()
	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	// store some test data
	f := fuzz.New().Funcs(ModelFuzzers...)
	for i := 0; i < 25; i++ {
		var (
			codeInfo    types.CodeInfo
			contract    types.ContractInfo
			stateModels []types.Model
			history     []types.ContractCodeHistoryEntry
		)
		f.Fuzz(&codeInfo)
		f.Fuzz(&contract)
		f.Fuzz(&stateModels)
		f.NilChance(0).Fuzz(&history)
		codeID, err := srcKeeper.Create(srcCtx, codeInfo.Creator, wasmCode, codeInfo.Source, codeInfo.Builder, &codeInfo.InstantiateConfig)
		require.NoError(t, err)
		contract.CodeID = codeID
		contractAddr := srcKeeper.generateContractAddress(srcCtx, codeID)
		srcKeeper.setContractInfo(srcCtx, contractAddr, &contract)
		srcKeeper.appendToContractHistory(srcCtx, contractAddr, history...)
		srcKeeper.importContractState(srcCtx, contractAddr, stateModels)
	}
	var wasmParams types.Params
	f.Fuzz(&wasmParams)
	srcKeeper.setParams(srcCtx, wasmParams)

	// export
	exportedState := ExportGenesis(srcCtx, srcKeeper)
	// order should not matter
	rand.Shuffle(len(exportedState.Codes), func(i, j int) {
		exportedState.Codes[i], exportedState.Codes[j] = exportedState.Codes[j], exportedState.Codes[i]
	})
	rand.Shuffle(len(exportedState.Contracts), func(i, j int) {
		exportedState.Contracts[i], exportedState.Contracts[j] = exportedState.Contracts[j], exportedState.Contracts[i]
	})
	rand.Shuffle(len(exportedState.Sequences), func(i, j int) {
		exportedState.Sequences[i], exportedState.Sequences[j] = exportedState.Sequences[j], exportedState.Sequences[i]
	})
	exportedGenesis, err := json.Marshal(exportedState)
	require.NoError(t, err)

	// reset contract history in source DB for comparision with dest DB
	srcKeeper.IterateContractInfo(srcCtx, func(address sdk.AccAddress, info wasmTypes.ContractInfo) bool {
		info.ResetFromGenesis(srcCtx)
		srcKeeper.setContractInfo(srcCtx, address, &info)
		return false
	})

	// re-import
	dstKeeper, dstCtx, dstStoreKeys, dstCleanup := setupKeeper(t)
	defer dstCleanup()

	var importState wasmTypes.GenesisState
	err = json.Unmarshal(exportedGenesis, &importState)
	require.NoError(t, err)
	InitGenesis(dstCtx, dstKeeper, importState)

	// compare whole DB
	for j := range srcStoreKeys {
		srcIT := srcCtx.KVStore(srcStoreKeys[j]).Iterator(nil, nil)
		dstIT := dstCtx.KVStore(dstStoreKeys[j]).Iterator(nil, nil)

		for i := 0; srcIT.Valid(); i++ {
			require.True(t, dstIT.Valid(), "[%s] destination DB has less elements than source. Missing: %s", srcStoreKeys[j].Name(), srcIT.Key())
			require.Equal(t, srcIT.Key(), dstIT.Key(), i)

			isContractHistory := srcStoreKeys[j].Name() == types.StoreKey && bytes.HasPrefix(srcIT.Key(), types.ContractHistoryStorePrefix)
			if !isContractHistory { // only skip history entries because we know they are different
				require.Equal(t, srcIT.Value(), dstIT.Value(), "[%s] element (%d): %X", srcStoreKeys[j].Name(), i, srcIT.Key())
			}
			srcIT.Next()
			dstIT.Next()
		}
		if !assert.False(t, dstIT.Valid()) {
			t.Fatalf("dest Iterator still has key :%X", dstIT.Key())
		}
	}
}

func TestFailFastImport(t *testing.T) {
	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	myCodeInfo := wasmTypes.CodeInfoFixture(wasmTypes.WithSHA256CodeHash(wasmCode))
	specs := map[string]struct {
		src        types.GenesisState
		expSuccess bool
	}{
		"happy path: code info correct": {
			src: types.GenesisState{
				Codes: []types.Code{{
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}},
				Sequences: []types.Sequence{
					{IDKey: types.KeyLastCodeID, Value: 2},
					{IDKey: types.KeyLastInstanceID, Value: 1},
				},
				Params: types.DefaultParams(),
			},
			expSuccess: true,
		},
		"happy path: code ids can contain gaps": {
			src: types.GenesisState{
				Codes: []types.Code{{
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}, {
					CodeID:     3,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}},
				Sequences: []types.Sequence{
					{IDKey: types.KeyLastCodeID, Value: 10},
					{IDKey: types.KeyLastInstanceID, Value: 1},
				},
				Params: types.DefaultParams(),
			},
			expSuccess: true,
		},
		"happy path: code order does not matter": {
			src: types.GenesisState{
				Codes: []types.Code{{
					CodeID:     2,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}, {
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}},
				Contracts: nil,
				Sequences: []types.Sequence{
					{IDKey: types.KeyLastCodeID, Value: 3},
					{IDKey: types.KeyLastInstanceID, Value: 1},
				},
				Params: types.DefaultParams(),
			},
			expSuccess: true,
		},
		"prevent code hash mismatch": {src: types.GenesisState{
			Codes: []types.Code{{
				CodeID:     1,
				CodeInfo:   wasmTypes.CodeInfoFixture(func(i *wasmTypes.CodeInfo) { i.CodeHash = make([]byte, sha256.Size) }),
				CodesBytes: wasmCode,
			}},
			Params: types.DefaultParams(),
		}},
		"prevent duplicate codeIDs": {src: types.GenesisState{
			Codes: []types.Code{
				{
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				},
				{
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				},
			},
			Params: types.DefaultParams(),
		}},
		"happy path: code id in info and contract do match": {
			src: types.GenesisState{
				Codes: []types.Code{{
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}},
				Contracts: []types.Contract{
					{
						ContractAddress: contractAddress(1, 1),
						ContractInfo:    types.ContractInfoFixture(func(c *wasmTypes.ContractInfo) { c.CodeID = 1 }, types.OnlyGenesisFields),
					},
				},
				Sequences: []types.Sequence{
					{IDKey: types.KeyLastCodeID, Value: 2},
					{IDKey: types.KeyLastInstanceID, Value: 2},
				},
				Params: types.DefaultParams(),
			},
			expSuccess: true,
		},
		"happy path: code info with two contracts": {
			src: types.GenesisState{
				Codes: []types.Code{{
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}},
				Contracts: []types.Contract{
					{
						ContractAddress: contractAddress(1, 1),
						ContractInfo:    types.ContractInfoFixture(func(c *wasmTypes.ContractInfo) { c.CodeID = 1 }, types.OnlyGenesisFields),
					}, {
						ContractAddress: contractAddress(1, 2),
						ContractInfo:    types.ContractInfoFixture(func(c *wasmTypes.ContractInfo) { c.CodeID = 1 }, types.OnlyGenesisFields),
					},
				},
				Sequences: []types.Sequence{
					{IDKey: types.KeyLastCodeID, Value: 2},
					{IDKey: types.KeyLastInstanceID, Value: 3},
				},
				Params: types.DefaultParams(),
			},
			expSuccess: true,
		},
		"prevent contracts that points to non existing codeID": {
			src: types.GenesisState{
				Contracts: []types.Contract{
					{
						ContractAddress: contractAddress(1, 1),
						ContractInfo:    types.ContractInfoFixture(func(c *wasmTypes.ContractInfo) { c.CodeID = 1 }, types.OnlyGenesisFields),
					},
				},
				Params: types.DefaultParams(),
			},
		},
		"prevent duplicate contract address": {
			src: types.GenesisState{
				Codes: []types.Code{{
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}},
				Contracts: []types.Contract{
					{
						ContractAddress: contractAddress(1, 1),
						ContractInfo:    types.ContractInfoFixture(func(c *wasmTypes.ContractInfo) { c.CodeID = 1 }, types.OnlyGenesisFields),
					}, {
						ContractAddress: contractAddress(1, 1),
						ContractInfo:    types.ContractInfoFixture(func(c *wasmTypes.ContractInfo) { c.CodeID = 1 }, types.OnlyGenesisFields),
					},
				},
				Params: types.DefaultParams(),
			},
		},
		"prevent duplicate contract model keys": {
			src: types.GenesisState{
				Codes: []types.Code{{
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}},
				Contracts: []types.Contract{
					{
						ContractAddress: contractAddress(1, 1),
						ContractInfo:    types.ContractInfoFixture(func(c *wasmTypes.ContractInfo) { c.CodeID = 1 }, types.OnlyGenesisFields),
						ContractState: []types.Model{
							{
								Key:   []byte{0x1},
								Value: []byte("foo"),
							},
							{
								Key:   []byte{0x1},
								Value: []byte("bar"),
							},
						},
					},
				},
				Params: types.DefaultParams(),
			},
		},
		"prevent duplicate sequences": {
			src: types.GenesisState{
				Sequences: []types.Sequence{
					{IDKey: []byte("foo"), Value: 1},
					{IDKey: []byte("foo"), Value: 9999},
				},
				Params: types.DefaultParams(),
			},
		},
		"prevent code id seq init value == max codeID used": {
			src: types.GenesisState{
				Codes: []types.Code{{
					CodeID:     2,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}},
				Sequences: []types.Sequence{
					{IDKey: types.KeyLastCodeID, Value: 1},
				},
				Params: types.DefaultParams(),
			},
		},
		"prevent contract id seq init value == count contracts": {
			src: types.GenesisState{
				Codes: []types.Code{{
					CodeID:     1,
					CodeInfo:   myCodeInfo,
					CodesBytes: wasmCode,
				}},
				Contracts: []types.Contract{
					{
						ContractAddress: contractAddress(1, 1),
						ContractInfo:    types.ContractInfoFixture(func(c *wasmTypes.ContractInfo) { c.CodeID = 1 }, types.OnlyGenesisFields),
					},
				},
				Sequences: []types.Sequence{
					{IDKey: types.KeyLastCodeID, Value: 2},
					{IDKey: types.KeyLastInstanceID, Value: 1},
				},
				Params: types.DefaultParams(),
			},
		},
	}

	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			keeper, ctx, _, cleanup := setupKeeper(t)
			defer cleanup()

			require.NoError(t, types.ValidateGenesis(spec.src))
			got := InitGenesis(ctx, keeper, spec.src)
			if spec.expSuccess {
				require.NoError(t, got)
				return
			}
			require.Error(t, got)
		})
	}
}

func TestImportContractWithCodeHistoryReset(t *testing.T) {
	genesis := `
{
	"params":{
		"code_upload_access": {
			"permission": "Everybody"
		},
		"instantiate_default_permission": "Everybody"
	},
  "codes": [
    {
      "code_id": "1",
      "code_info": {
        "code_hash": %q,
        "creator": "cosmos1qtu5n0cnhfkjj6l2rq97hmky9fd89gwca9yarx",
        "source": "https://example.com",
        "builder": "foo/bar:tag",
        "instantiate_config": {
          "permission": "OnlyAddress",
          "address": "cosmos1qtu5n0cnhfkjj6l2rq97hmky9fd89gwca9yarx"
        }
      },
      "code_bytes": %q
    }
  ],
  "contracts": [
    {
      "contract_address": "cosmos18vd8fpwxzck93qlwghaj6arh4p7c5n89uzcee5",
      "contract_info": {
        "code_id": "1",
        "creator": "cosmos13x849jzd03vne42ynpj25hn8npjecxqrjghd8x",
        "admin": "cosmos1h5t8zxmjr30e9dqghtlpl40f2zz5cgey6esxtn",
        "label": "ȀĴnZV芢毤"
      }
    }
  ],
  "sequences": [
  {"id_key": %q, "value": "2"},
  {"id_key": %q, "value": "2"}
  ]
}`
	keeper, ctx, _, dstCleanup := setupKeeper(t)
	defer dstCleanup()

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	wasmCodeHash := sha256.Sum256(wasmCode)
	enc64 := base64.StdEncoding.EncodeToString
	var importState wasmTypes.GenesisState
	err = keeper.cdc.UnmarshalJSON([]byte(
		fmt.Sprintf(genesis, enc64(wasmCodeHash[:]), enc64(wasmCode),
			enc64(append([]byte{0x04}, []byte("lastCodeId")...)),
			enc64(append([]byte{0x04}, []byte("lastContractId")...))),
	), &importState)
	require.NoError(t, err)
	require.NoError(t, importState.ValidateBasic())

	ctx = ctx.WithBlockHeight(0).WithGasMeter(sdk.NewInfiniteGasMeter())

	// when
	err = InitGenesis(ctx, keeper, importState)
	require.NoError(t, err)

	// verify wasm code
	gotWasmCode, err := keeper.GetByteCode(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, wasmCode, gotWasmCode, "byte code does not match")

	// verify code info
	gotCodeInfo := keeper.GetCodeInfo(ctx, 1)
	require.NotNil(t, gotCodeInfo)
	codeCreatorAddr, _ := sdk.AccAddressFromBech32("cosmos1qtu5n0cnhfkjj6l2rq97hmky9fd89gwca9yarx")
	expCodeInfo := types.CodeInfo{
		CodeHash: wasmCodeHash[:],
		Creator:  codeCreatorAddr,
		Source:   "https://example.com",
		Builder:  "foo/bar:tag",
		InstantiateConfig: wasmTypes.AccessConfig{
			Type:    types.OnlyAddress,
			Address: codeCreatorAddr,
		},
	}
	assert.Equal(t, expCodeInfo, *gotCodeInfo)

	// verify contract
	contractAddr, _ := sdk.AccAddressFromBech32("cosmos18vd8fpwxzck93qlwghaj6arh4p7c5n89uzcee5")
	gotContractInfo := keeper.GetContractInfo(ctx, contractAddr)
	require.NotNil(t, gotContractInfo)
	contractCreatorAddr, _ := sdk.AccAddressFromBech32("cosmos13x849jzd03vne42ynpj25hn8npjecxqrjghd8x")
	adminAddr, _ := sdk.AccAddressFromBech32("cosmos1h5t8zxmjr30e9dqghtlpl40f2zz5cgey6esxtn")

	expContractInfo := types.ContractInfo{
		CodeID:  1,
		Creator: contractCreatorAddr,
		Admin:   adminAddr,
		Label:   "ȀĴnZV芢毤",
		Created: &types.AbsoluteTxPosition{BlockHeight: 0, TxIndex: 0},
	}
	assert.Equal(t, expContractInfo, *gotContractInfo)

	expHistory := []types.ContractCodeHistoryEntry{{
		Operation: types.GenesisContractCodeHistoryType,
		CodeID:    1,
		Updated:   types.NewAbsoluteTxPosition(ctx),
	},
	}
	assert.Equal(t, expHistory, keeper.GetContractHistory(ctx, contractAddr))
}

func setupKeeper(t *testing.T) (Keeper, sdk.Context, []sdk.StoreKey, func()) {
	t.Helper()
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	cleanup := func() { os.RemoveAll(tempDir) }
	//t.Cleanup(cleanup) todo: add with Go 1.14
	var (
		keyParams  = sdk.NewKVStoreKey(params.StoreKey)
		tkeyParams = sdk.NewTransientStoreKey(params.TStoreKey)
		keyWasm    = sdk.NewKVStoreKey(wasmTypes.StoreKey)
	)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyWasm, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, sdk.StoreTypeTransient, db)
	require.NoError(t, ms.LoadLatestVersion())

	ctx := sdk.NewContext(ms, abci.Header{
		Height: 1234567,
		Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
	}, false, log.NewNopLogger())
	cdc := MakeTestCodec()
	pk := params.NewKeeper(cdc, keyParams, tkeyParams)
	wasmConfig := wasmTypes.DefaultWasmConfig()
	srcKeeper := NewKeeper(cdc, keyWasm, pk.Subspace(wasmTypes.DefaultParamspace), auth.AccountKeeper{}, nil, staking.Keeper{}, nil, tempDir, wasmConfig, "", nil, nil)
	srcKeeper.setParams(ctx, wasmTypes.DefaultParams())

	return srcKeeper, ctx, []sdk.StoreKey{keyWasm, keyParams}, cleanup
}
