package keeper

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/CosmWasm/wasmd/x/wasm/internal/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreCodeProposal(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	ctx, keepers := CreateTestInput(t, false, tempDir, "staking", nil, nil)
	govKeeper, wasmKeeper := keepers.GovKeeper, keepers.WasmKeeper
	wasmKeeper.setParams(ctx, types.Params{UploadAccess: types.AllowNobody, DefaultInstantiatePermission: types.Nobody})
	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	var anyAddress sdk.AccAddress = make([]byte, sdk.AddrLen)

	src := types.StoreCodeProposalFixture(func(p *types.StoreCodeProposal) {
		p.RunAs = anyAddress
		p.WASMByteCode = wasmCode
		p.Source = "https://example.com/mysource"
		p.Builder = "foo/bar:v0.0.0"
	})

	// when stored
	storedProposal, err := govKeeper.SubmitProposal(ctx, src)
	require.NoError(t, err)

	// and proposal execute
	handler := govKeeper.Router().GetRoute(storedProposal.ProposalRoute())
	err = handler(ctx, storedProposal.Content)
	require.NoError(t, err)

	// then
	cInfo := wasmKeeper.GetCodeInfo(ctx, 1)
	require.NotNil(t, cInfo)
	assert.Equal(t, anyAddress, cInfo.Creator)
	assert.Equal(t, "foo/bar:v0.0.0", cInfo.Builder)
	assert.Equal(t, "https://example.com/mysource", cInfo.Source)

	storedCode, err := wasmKeeper.GetByteCode(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, wasmCode, storedCode)
}

func TestInstantiateProposal(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	ctx, keepers := CreateTestInput(t, false, tempDir, "staking", nil, nil)
	govKeeper, wasmKeeper := keepers.GovKeeper, keepers.WasmKeeper
	wasmKeeper.setParams(ctx, types.Params{UploadAccess: types.AllowNobody, DefaultInstantiatePermission: types.Nobody})

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	require.NoError(t, wasmKeeper.importCode(ctx, 1,
		types.CodeInfoFixture(types.WithSHA256CodeHash(wasmCode)),
		wasmCode),
	)

	var (
		oneAddress   sdk.AccAddress = bytes.Repeat([]byte{0x1}, sdk.AddrLen)
		otherAddress sdk.AccAddress = bytes.Repeat([]byte{0x2}, sdk.AddrLen)
	)
	src := types.InstantiateContractProposalFixture(func(p *types.InstantiateContractProposal) {
		p.CodeID = 1
		p.RunAs = oneAddress
		p.Admin = otherAddress
		p.Label = "testing"
	})

	// when stored
	storedProposal, err := govKeeper.SubmitProposal(ctx, src)
	require.NoError(t, err)

	// and proposal execute
	handler := govKeeper.Router().GetRoute(storedProposal.ProposalRoute())
	err = handler(ctx, storedProposal.Content)
	require.NoError(t, err)

	// then
	contractAddr, err := sdk.AccAddressFromBech32("cosmos18vd8fpwxzck93qlwghaj6arh4p7c5n89uzcee5")
	require.NoError(t, err)

	cInfo := wasmKeeper.GetContractInfo(ctx, contractAddr)
	require.NotNil(t, cInfo)
	assert.Equal(t, uint64(1), cInfo.CodeID)
	assert.Equal(t, oneAddress, cInfo.Creator)
	assert.Equal(t, otherAddress, cInfo.Admin)
	assert.Equal(t, "testing", cInfo.Label)
	expHistory := []types.ContractCodeHistoryEntry{{
		Operation: types.InitContractCodeHistoryType,
		CodeID:    src.CodeID,
		Updated:   types.NewAbsoluteTxPosition(ctx),
		Msg:       src.InitMsg,
	}}
	assert.Equal(t, expHistory, wasmKeeper.GetContractHistory(ctx, contractAddr))
}

func TestMigrateProposal(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	ctx, keepers := CreateTestInput(t, false, tempDir, "staking", nil, nil)
	govKeeper, wasmKeeper := keepers.GovKeeper, keepers.WasmKeeper
	wasmKeeper.setParams(ctx, types.Params{UploadAccess: types.AllowNobody, DefaultInstantiatePermission: types.Nobody})

	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	codeInfoFixture := types.CodeInfoFixture(types.WithSHA256CodeHash(wasmCode))
	require.NoError(t, wasmKeeper.importCode(ctx, 1, codeInfoFixture, wasmCode))
	require.NoError(t, wasmKeeper.importCode(ctx, 2, codeInfoFixture, wasmCode))

	var (
		anyAddress   sdk.AccAddress = bytes.Repeat([]byte{0x1}, sdk.AddrLen)
		otherAddress sdk.AccAddress = bytes.Repeat([]byte{0x2}, sdk.AddrLen)
		contractAddr                = contractAddress(1, 1)
	)

	contractInfoFixture := types.ContractInfoFixture(func(c *types.ContractInfo) {
		c.Label = "testing"
		c.Admin = anyAddress
	})
	key, err := hex.DecodeString("636F6E666967")
	require.NoError(t, err)
	m := types.Model{Key: key, Value: []byte(`{"verifier":"AAAAAAAAAAAAAAAAAAAAAAAAAAA=","beneficiary":"AAAAAAAAAAAAAAAAAAAAAAAAAAA=","funder":"AQEBAQEBAQEBAQEBAQEBAQEBAQE="}`)}
	require.NoError(t, wasmKeeper.importContract(ctx, contractAddr, &contractInfoFixture, []types.Model{m}))

	migMsg := struct {
		Verifier sdk.AccAddress `json:"verifier"`
	}{Verifier: otherAddress}
	migMsgBz, err := json.Marshal(migMsg)
	require.NoError(t, err)

	src := types.MigrateContractProposal{
		WasmProposal: types.WasmProposal{
			Title:       "Foo",
			Description: "Bar",
		},
		CodeID:     2,
		Contract:   contractAddr,
		MigrateMsg: migMsgBz,
		RunAs:      otherAddress,
	}

	// when stored
	storedProposal, err := govKeeper.SubmitProposal(ctx, src)
	require.NoError(t, err)

	// and proposal execute
	handler := govKeeper.Router().GetRoute(storedProposal.ProposalRoute())
	err = handler(ctx, storedProposal.Content)
	require.NoError(t, err)

	// then
	require.NoError(t, err)
	cInfo := wasmKeeper.GetContractInfo(ctx, contractAddr)
	require.NotNil(t, cInfo)
	assert.Equal(t, uint64(2), cInfo.CodeID)
	assert.Equal(t, anyAddress, cInfo.Admin)
	assert.Equal(t, "testing", cInfo.Label)
	expHistory := []types.ContractCodeHistoryEntry{{
		Operation: types.GenesisContractCodeHistoryType,
		CodeID:    1,
		Updated:   types.NewAbsoluteTxPosition(ctx),
	}, {
		Operation: types.MigrateContractCodeHistoryType,
		CodeID:    src.CodeID,
		Updated:   types.NewAbsoluteTxPosition(ctx),
		Msg:       src.MigrateMsg,
	}}
	assert.Equal(t, expHistory, wasmKeeper.GetContractHistory(ctx, contractAddr))

}

func TestAdminProposals(t *testing.T) {
	var (
		otherAddress sdk.AccAddress = bytes.Repeat([]byte{0x2}, sdk.AddrLen)
		contractAddr                = contractAddress(1, 1)
	)
	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)

	specs := map[string]struct {
		state       types.ContractInfo
		srcProposal gov.Content
		expAdmin    sdk.AccAddress
	}{
		"update with different admin": {
			state: types.ContractInfoFixture(),
			srcProposal: types.UpdateAdminProposal{
				WasmProposal: types.WasmProposal{
					Title:       "Foo",
					Description: "Bar",
				},
				Contract: contractAddr,
				NewAdmin: otherAddress,
			},
			expAdmin: otherAddress,
		},
		"update with old admin empty": {
			state: types.ContractInfoFixture(func(info *types.ContractInfo) {
				info.Admin = nil
			}),
			srcProposal: types.UpdateAdminProposal{
				WasmProposal: types.WasmProposal{
					Title:       "Foo",
					Description: "Bar",
				},
				Contract: contractAddr,
				NewAdmin: otherAddress,
			},
			expAdmin: otherAddress,
		},
		"clear admin": {
			state: types.ContractInfoFixture(),
			srcProposal: types.ClearAdminProposal{
				WasmProposal: types.WasmProposal{
					Title:       "Foo",
					Description: "Bar",
				},
				Contract: contractAddr,
			},
			expAdmin: nil,
		},
		"clear with old admin empty": {
			state: types.ContractInfoFixture(func(info *types.ContractInfo) {
				info.Admin = nil
			}),
			srcProposal: types.ClearAdminProposal{
				WasmProposal: types.WasmProposal{
					Title:       "Foo",
					Description: "Bar",
				},
				Contract: contractAddr,
			},
			expAdmin: nil,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			tempDir, err := ioutil.TempDir("", "wasm")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)
			ctx, keepers := CreateTestInput(t, false, tempDir, "staking", nil, nil)
			govKeeper, wasmKeeper := keepers.GovKeeper, keepers.WasmKeeper
			wasmKeeper.setParams(ctx, types.Params{UploadAccess: types.AllowNobody, DefaultInstantiatePermission: types.Nobody})

			codeInfoFixture := types.CodeInfoFixture(types.WithSHA256CodeHash(wasmCode))
			require.NoError(t, wasmKeeper.importCode(ctx, 1, codeInfoFixture, wasmCode))

			require.NoError(t, wasmKeeper.importContract(ctx, contractAddr, &spec.state, []types.Model{}))
			// when stored
			storedProposal, err := govKeeper.SubmitProposal(ctx, spec.srcProposal)
			require.NoError(t, err)

			// and execute proposal
			handler := govKeeper.Router().GetRoute(storedProposal.ProposalRoute())
			err = handler(ctx, storedProposal.Content)
			require.NoError(t, err)

			// then
			cInfo := wasmKeeper.GetContractInfo(ctx, contractAddr)
			require.NotNil(t, cInfo)
			assert.Equal(t, spec.expAdmin, cInfo.Admin)
		})
	}
}

func TestUpdateParamsProposal(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	ctx, keepers := CreateTestInput(t, false, tempDir, "staking", nil, nil)
	govKeeper, wasmKeeper := keepers.GovKeeper, keepers.WasmKeeper

	var (
		cdc                                   = keepers.WasmKeeper.cdc
		myAddress              sdk.AccAddress = make([]byte, sdk.AddrLen)
		oneAddressAccessConfig                = types.OnlyAddress.With(myAddress)
	)

	specs := map[string]struct {
		src                params.ParamChange
		expUploadConfig    types.AccessConfig
		expInstantiateType types.AccessType
	}{
		"update upload permission param": {
			src: params.ParamChange{
				Subspace: types.DefaultParamspace,
				Key:      string(types.ParamStoreKeyUploadAccess),
				Value:    string(cdc.MustMarshalJSON(&types.AllowNobody)),
			},
			expUploadConfig:    types.AllowNobody,
			expInstantiateType: types.Everybody,
		},
		"update upload permission param with address": {
			src: params.ParamChange{
				Subspace: types.DefaultParamspace,
				Key:      string(types.ParamStoreKeyUploadAccess),
				Value:    string(cdc.MustMarshalJSON(&oneAddressAccessConfig)),
			},
			expUploadConfig:    oneAddressAccessConfig,
			expInstantiateType: types.Everybody,
		},
		"update instantiate param": {
			src: params.ParamChange{
				Subspace: types.DefaultParamspace,
				Key:      string(types.ParamStoreKeyInstantiateAccess),
				Value:    string(cdc.MustMarshalJSON(types.Nobody)),
			},
			expUploadConfig:    types.AllowEverybody,
			expInstantiateType: types.Nobody,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			wasmKeeper.setParams(ctx, types.DefaultParams())

			proposal := params.ParameterChangeProposal{
				Title:       "Foo",
				Description: "Bar",
				Changes:     []params.ParamChange{spec.src},
			}

			// when stored
			storedProposal, err := govKeeper.SubmitProposal(ctx, proposal)
			require.NoError(t, err)

			// and proposal execute
			handler := govKeeper.Router().GetRoute(storedProposal.ProposalRoute())
			err = handler(ctx, storedProposal.Content)
			require.NoError(t, err)

			// then
			assert.True(t, spec.expUploadConfig.Equals(wasmKeeper.getUploadAccessConfig(ctx)),
				"got %#v not %#v", wasmKeeper.getUploadAccessConfig(ctx), spec.expUploadConfig)
			assert.Equal(t, spec.expInstantiateType, wasmKeeper.getInstantiateAccessConfig(ctx))
		})
	}
}
