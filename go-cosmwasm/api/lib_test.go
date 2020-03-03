package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/enigmampc/EnigmaBlockchain/go-cosmwasm/types"
)

func TestInitAndReleaseCache(t *testing.T) {
	dataDir := "/foo"
	_, err := InitCache(dataDir, 3)
	require.Error(t, err)

	tmpdir, err := ioutil.TempDir("", "go-cosmwasm")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	cache, err := InitCache(tmpdir, 3)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func withCache(t *testing.T) (Cache, func()) {
	tmpdir, err := ioutil.TempDir("", "go-cosmwasm")
	require.NoError(t, err)
	cache, err := InitCache(tmpdir, 3)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpdir)
		ReleaseCache(cache)
	}
	return cache, cleanup
}

func TestCreateAndGet(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := ioutil.ReadFile("./testdata/contract_0.7.wasm")
	require.NoError(t, err)

	id, err := Create(cache, wasm)
	require.NoError(t, err)

	code, err := GetCode(cache, id)
	require.NoError(t, err)
	require.Equal(t, wasm, code)
}

func TestCreateFailsWithBadData(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm := []byte("some invalid data")
	_, err := Create(cache, wasm)
	require.Error(t, err)
}

func mockEnv(signer []byte) types.Env {
	return types.Env{
		Block: types.BlockInfo{
			Height:  123,
			Time:    1578939743,
			ChainID: "foobar",
		},
		Message: types.MessageInfo{
			Signer: signer,
			SentFunds: []types.Coin{{
				Denom:  "ATOM",
				Amount: "100",
			}},
		},
		Contract: types.ContractInfo{
			Address: binaryAddr("contract"),
			Balance: []types.Coin{{
				Denom:  "ATOM",
				Amount: "100",
			}},
		},
	}
}

func binaryAddr(human string) []byte {
	res := make([]byte, 42)
	copy(res, []byte(human))
	return res
}

func TestInstantiate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/contract_0.7.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	// instantiate it with this store
	store := NewLookup()
	api := NewMockAPI()
	params, err := json.Marshal(mockEnv(binaryAddr("creator")))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, cost, err := Instantiate(cache, id, params, msg, store, api, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(64_095), cost)

	var resp types.CosmosResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	require.Equal(t, "", resp.Err)
	require.Equal(t, 0, len(resp.Ok.Messages))
}

func TestHandle(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	id := createTestContract(t, cache)

	// instantiate it with this store
	store := NewLookup()
	api := NewMockAPI()
	params, err := json.Marshal(mockEnv(binaryAddr("creator")))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	start := time.Now()
	res, cost, err := Instantiate(cache, id, params, msg, store, api, 100000000)
	diff := time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(64_095), cost)
	fmt.Printf("Time (64_095 gas): %s\n", diff)

	// execute with the same store
	params, err = json.Marshal(mockEnv(binaryAddr("fred")))
	require.NoError(t, err)
	start = time.Now()
	res, cost, err = Handle(cache, id, params, []byte(`{"release":{}}`), store, api, 100000000)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 1)
	assert.Equal(t, uint64(101_455), cost)
	fmt.Printf("Time (101_455 gas): %s\n", diff)
}

func TestMultipleInstances(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	id := createTestContract(t, cache)

	// instance1 controlled by fred
	store1 := NewLookup()
	api := NewMockAPI()
	params, err := json.Marshal(mockEnv(binaryAddr("regen")))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, cost, err := Instantiate(cache, id, params, msg, store1, api, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(64_095), cost)

	// instance2 controlled by mary
	store2 := NewLookup()
	params, err = json.Marshal(mockEnv(binaryAddr("chorus")))
	require.NoError(t, err)
	msg = []byte(`{"verifier": "mary", "beneficiary": "sue"}`)
	res, cost, err = Instantiate(cache, id, params, msg, store2, api, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(63_255), cost)

	// fail to execute store1 with mary
	resp := exec(t, cache, id, "mary", store1, api, 87_509)
	require.Equal(t, "Unauthorized", resp.Err)

	// succeed to execute store1 with fred
	resp = exec(t, cache, id, "fred", store1, api, 101_455)
	require.Equal(t, "", resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
	logs := resp.Ok.Log
	require.Equal(t, 2, len(logs))
	require.Equal(t, "destination", logs[1].Key)
	require.Equal(t, "bob", logs[1].Value)

	// succeed to execute store2 with mary
	resp = exec(t, cache, id, "mary", store2, api, 101_455)
	require.Equal(t, "", resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
	logs = resp.Ok.Log
	require.Equal(t, 2, len(logs))
	require.Equal(t, "destination", logs[1].Key)
	require.Equal(t, "sue", logs[1].Value)
}

func requireOkResponse(t *testing.T, res []byte, expectedMsgs int) {
	var resp types.CosmosResponse
	err := json.Unmarshal(res, &resp)
	require.NoError(t, err)
	require.Equal(t, "", resp.Err)
	require.Equal(t, expectedMsgs, len(resp.Ok.Messages))
}

func createTestContract(t *testing.T, cache Cache) []byte {
	wasm, err := ioutil.ReadFile("./testdata/contract_0.7.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)
	return id
}

// exec runs the handle tx with the given signer
func exec(t *testing.T, cache Cache, id []byte, signer string, store KVStore, api *GoAPI, gas uint64) types.CosmosResponse {
	params, err := json.Marshal(mockEnv(binaryAddr(signer)))
	require.NoError(t, err)
	res, cost, err := Handle(cache, id, params, []byte(`{"release":{}}`), store, api, 100000000)
	require.NoError(t, err)
	assert.Equal(t, gas, cost)

	var resp types.CosmosResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	return resp
}

func TestQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	id := createTestContract(t, cache)

	// set up contract
	store := NewLookup()
	api := NewMockAPI()
	params, err := json.Marshal(mockEnv(binaryAddr("creator")))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, id, params, msg, store, api, 100000000)
	require.NoError(t, err)

	// invalid query
	query := []byte(`{"Raw":{"val":"config"}}`)
	data, _, err := Query(cache, id, query, store, api, 100000000)
	require.NoError(t, err)
	var badResp types.QueryResponse
	err = json.Unmarshal(data, &badResp)
	require.NoError(t, err)
	require.Equal(t, badResp.Err, "Error parsing QueryMsg: unknown variant `Raw`, expected `verifier`")

	// make a valid query
	query = []byte(`{"verifier":{}}`)
	data, _, err = Query(cache, id, query, store, api, 100000000)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Equal(t, qres.Err, "")
	require.Equal(t, string(qres.Ok), "fred")
}
