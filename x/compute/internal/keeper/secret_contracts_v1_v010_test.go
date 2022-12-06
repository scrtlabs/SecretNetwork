package keeper

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	v010cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestV1InitV010ContractWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	msg := fmt.Sprintf(`{"init_v10":{"counter":80, "code_id":%d, "code_hash":"%s"}}`, v010CodeID, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	accAddress, err := sdk.AccAddressFromBech32(string(data))
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, accAddress, `{"get_count_from_v1":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(80), resp.Get.Count)
}

func TestV1ExecuteV010ContractWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"exec_v10":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(20), binary.BigEndian.Uint32(data))
}

func TestV1InitV010ContractNoReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	msg := fmt.Sprintf(`{"init_v10_no_reply":{"counter":180, "code_id":%d, "code_hash":"%s"}}`, v010CodeID, v010CodeHash)

	_, _, _, ev, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)

	var newContractBech32 string
	for _, v := range ev[1] {
		if v.Key == "contract_address" {
			newContractBech32 = v.Value
			break
		}
	}
	require.NotEmpty(t, newContractBech32)

	accAddress, err := sdk.AccAddressFromBech32(newContractBech32)
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, accAddress, `{"get_count_from_v1":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(180), resp.Get.Count)
}

func TestV1ExecuteV010ContractNoReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"exec_v10_no_reply":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	queryRes, qErr := queryHelper(t, keeper, ctx, v010ContractAddress, `{"get_count_from_v1":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(20), resp.Get.Count)
}

func TestV1InitV010ContractWithReplyWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	msg := fmt.Sprintf(`{"init_v10_with_error":{"code_id":%d, "code_hash":"%s"}}`, v010CodeID, v010CodeHash)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
}

func TestV1ExecuteV010ContractWithReplyWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"exec_v10_with_error":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
}

func TestV1InitV010ContractNoReplyWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
	msg := fmt.Sprintf(`{"init_v10_no_reply_with_error":{"code_id":%d, "code_hash":"%s"}}`, v010CodeID, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)
	require.Nil(t, data)
}

func TestV1ExecuteV010ContractNoReplyWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"exec_v10_no_reply_with_error":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)
	require.Nil(t, data)
}

func TestV1QueryV010ContractWithError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	msg := fmt.Sprintf(`{"query_v10_with_error":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)
	require.Nil(t, data)
}

func TestV010InitV1ContractFromInitWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, initEvents, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, true, defaultGasForTests)
	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "10")

	require.Empty(t, err)
	var newContractBech32 string
	for _, v := range initEvents[1] {
		if v.Key == "contract_address" {
			newContractBech32 = v.Value
			break
		}
	}
	require.NotEmpty(t, newContractBech32)
	accAddress, err := sdk.AccAddressFromBech32(newContractBech32)
	require.Empty(t, err)

	queryRes, qErr = queryHelper(t, keeper, ctx, accAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "1")
}

func TestV010InitV1ContractFromExecuteWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "10")

	_, _, execData, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, execData)

	var newContractBech32 string
	for _, v := range execEvents[1] {
		if v.Key == "contract_address" {
			newContractBech32 = v.Value
			break
		}
	}
	require.NotEmpty(t, newContractBech32)
	accAddress, err := sdk.AccAddressFromBech32(newContractBech32)
	require.Empty(t, err)

	queryRes, qErr = queryHelper(t, keeper, ctx, accAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "1")
}

func TestV010ExecuteV1ContractFromInitWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":199, "expires":100}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	_, _, _, _, err = initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"increment\":{\"addition\": 1}}`), true, true, defaultGasForTests)
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(200), resp.Get.Count)
}

func TestV010ExecuteV1ContractFromExecuteWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":299, "expires":100}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"increment\":{\"addition\": 1}}`), true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
	require.Empty(t, qErr)

	// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
	var resp v1QueryResponse
	e := json.Unmarshal([]byte(queryRes), &resp)
	require.NoError(t, e)
	require.Equal(t, uint32(300), resp.Get.Count)
}

func TestV010QueryV1ContractFromInitWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, events, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"receive_external_query_v1\":{\"num\":1}}`), true, true, defaultGasForTests)
	require.Empty(t, err)
	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "c", Value: "2"},
			},
		},
		events,
	)
}

func TestV010QueryV1ContractFromExecuteWithOkResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	_, _, _, events, _, err := execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"receive_external_query_v1\":{\"num\":1}}`), true, true, defaultGasForTests, 0)
	require.Empty(t, err)
	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "c", Value: "2"},
			},
		},
		events,
	)
}

func TestV010InitV1ContractFromInitWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, _, _, err = initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d, "code_hash":"%s","label":"blabla", "msg":"%s"}}`, codeID, codeHash, `{\"counter\":{\"counter\":0, \"expires\":100}}`), true, true, defaultGasForTests)
	require.Contains(t, fmt.Sprintf("%+v", err), "got wrong counter on init")
}

func TestV010InitV1ContractFromExecuteWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get_contract_version":{}}`, true, false, math.MaxUint64)
	require.Empty(t, qErr)

	require.Equal(t, queryRes, "10")

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d, "code_hash":"%s","label":"blabla", "msg":"%s"}}`, codeID, codeHash, `{\"counter\":{\"counter\":0, \"expires\":100}}`), true, true, defaultGasForTests, 0)
	require.Contains(t, fmt.Sprintf("%+v", err), "got wrong counter on init")
}

func TestV010ExecuteV1ContractFromInitWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":199, "expires":100}}`, true, true, defaultGasForTests)
	_, _, _, _, err = initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"increment\":{\"addition\": 0}}`), true, true, defaultGasForTests)

	require.Contains(t, fmt.Sprintf("%+v", err), "got wrong counter on increment")
}

func TestV010ExecuteV1ContractFromExecuteWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":299, "expires":100}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"increment\":{\"addition\": 0}}`), true, true, defaultGasForTests, 0)
	require.Contains(t, fmt.Sprintf("%+v", err), "got wrong counter on increment")
}

func TestV010QueryV1ContractFromInitWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, _, _, err = initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"contract_error\":{\"error_type\":\"generic_err\"}}`), true, true, defaultGasForTests)
	require.Contains(t, fmt.Sprintf("%+v", err), "la la 🤯")
}

func TestV010QueryV1ContractFromExecuteWithErrResponse(t *testing.T) {
	ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, contractAddress.String(), codeHash, `{\"contract_error\":{\"error_type\":\"generic_err\"}}`), true, true, defaultGasForTests, 0)
	require.Contains(t, fmt.Sprintf("%+v", err), "la la 🤯")
}

func TestV1SendsLogsMixedWithV010WithoutReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)
	_, _, v1ContractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, v1ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"add_attributes_from_v010":{"addr":"%s","code_hash":"%s", "id":0}}`, v010ContractAddress, v010CodeHash), true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: v1ContractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}
	}

	require.True(t, hadCyber1)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "attr3", Value: "🍉"},
				{Key: "attr4", Value: "🥝"},
			},
		},
		logs,
	)
}

func TestV1SendsLogsMixedWithV010WithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
	require.NoError(t, err)
	v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)
	_, _, v1ContractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, v1ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"add_attributes_from_v010":{"addr":"%s","code_hash":"%s", "id":2500}}`, v010ContractAddress, v010CodeHash), true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber1 := false
	hadCyber3 := false

	for _, e := range events {
		if e.Type == "wasm-cyber1" {
			require.False(t, hadCyber1)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: v1ContractAddress.String()},
					{Key: "attr1", Value: "🦄"},
					{Key: "attr2", Value: "🌈"},
				},
				attrs,
			)

			hadCyber1 = true
		}

		if e.Type == "wasm-cyber3" {
			require.False(t, hadCyber3)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: v1ContractAddress.String()},
					{Key: "attr9", Value: "🤯"},
					{Key: "attr10", Value: "🤟"},
				},
				attrs,
			)

			hadCyber3 = true
		}
	}

	require.True(t, hadCyber1)
	require.True(t, hadCyber3)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr3", Value: "🐙"},
				{Key: "attr4", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "attr3", Value: "🍉"},
				{Key: "attr4", Value: "🥝"},
			},
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr11", Value: "😉"},
				{Key: "attr12", Value: "😊"},
			},
		},
		logs,
	)
}

func TestV010SendsLogsMixedWithV1(t *testing.T) {
	ctx, keeper, codeID, v1CodeHash, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
	require.NoError(t, err)

	v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
	require.NoError(t, err)

	_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, err)
	_, _, v1ContractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)
	nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, v010ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"add_mixed_events_and_attributes_from_v1":{"addr":"%s","code_hash":"%s"}}`, v1ContractAddress, v1CodeHash), true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	events := ctx.EventManager().Events()

	hadCyber2 := false

	for _, e := range events {
		if e.Type == "wasm-cyber2" {
			require.False(t, hadCyber2)
			attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
			require.Empty(t, err)

			requireLogAttributes(t,
				[]v010cosmwasm.LogAttribute{
					{Key: "contract_address", Value: v1ContractAddress.String()},
					{Key: "attr5", Value: "🐙"},
					{Key: "attr6", Value: "🦄"},
				},
				attrs,
			)

			hadCyber2 = true
		}
	}

	require.True(t, hadCyber2)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v010ContractAddress.String()},
				{Key: "attr1", Value: "🦄"},
				{Key: "attr2", Value: "🌈"},
			},
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr7", Value: "😅"},
				{Key: "attr8", Value: "🦄"},
			},
		},
		logs,
	)
}
