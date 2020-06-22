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

const DEFAULT_FEATURES = "staking"

func TestInitAndReleaseCache(t *testing.T) {
	dataDir := "/foo"
	_, err := InitCache(dataDir, DEFAULT_FEATURES, 3)
	require.Error(t, err)

	tmpdir, err := ioutil.TempDir("", "go-cosmwasm")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	cache, err := InitCache(tmpdir, DEFAULT_FEATURES, 3)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func withCache(t *testing.T) (Cache, func()) {
	tmpdir, err := ioutil.TempDir("", "go-cosmwasm")
	require.NoError(t, err)
	cache, err := InitCache(tmpdir, DEFAULT_FEATURES, 3)
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

	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
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

const mockContractAddr = "contract"

func mockEnv(sender []byte) types.Env {
	return types.Env{
		Block: types.BlockInfo{
			Height:  123,
			Time:    1578939743,
			ChainID: "foobar",
		},
		Message: types.MessageInfo{
			Sender: sender,
			SentFunds: []types.Coin{{
				Denom:  "ATOM",
				Amount: "100",
			}},
		},
		Contract: types.ContractInfo{
			Address: binaryAddr(mockContractAddr),
		},
	}
}

func binaryAddr(human string) []byte {
	res := make([]byte, 32)
	copy(res, []byte(human))
	return res
}

func TestInstantiate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup()
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv(binaryAddr("creator")))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, cost, err := Instantiate(cache, id, params, msg, &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x11f57), cost)

	var resp types.InitResult
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Err)
	require.Equal(t, 0, len(resp.Ok.Messages))
}

func TestHandle(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	id := createTestContract(t, cache)

	gasMeter1 := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup()
	api := NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(mockContractAddr, balance)
	params, err := json.Marshal(mockEnv(binaryAddr("creator")))
	require.NoError(t, err)

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	start := time.Now()
	res, cost, err := Instantiate(cache, id, params, msg, &gasMeter1, store, api, &querier, 100000000)
	diff := time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x11f57), cost)
	fmt.Printf("Time (%d gas): %s\n", 0xbb66, diff)

	// execute with the same store
	gasMeter2 := NewMockGasMeter(100000000)
	params, err = json.Marshal(mockEnv(binaryAddr("fred")))
	require.NoError(t, err)
	start = time.Now()
	res, cost, err = Handle(cache, id, params, []byte(`{"release":{}}`), &gasMeter2, store, api, &querier, 100000000)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	assert.Equal(t, uint64(0x1c135), cost)
	fmt.Printf("Time (%d gas): %s\n", cost, diff)

	// make sure it read the balance properly and we got 250 atoms
	var resp types.HandleResult
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
	dispatch := resp.Ok.Messages[0]
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	assert.Equal(t, send.ToAddress, "bob")
	assert.Equal(t, send.FromAddress, mockContractAddr)
	assert.Equal(t, send.Amount, balance)
}

func TestMigrate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	id := createTestContract(t, cache)

	gasMeter := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup()
	api := NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(mockContractAddr, balance)
	params, err := json.Marshal(mockEnv(binaryAddr("creator")))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, _, err := Instantiate(cache, id, params, msg, &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// verifier is fred
	query := []byte(`{"verifier":{}}`)
	data, _, err := Query(cache, id, query, &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Nil(t, qres.Err, "%v", qres.Err)
	require.Equal(t, string(qres.Ok), `{"verifier":"fred"}`)

	// migrate to a new verifier - alice
	// we use the same code blob as we are testing hackatom self-migration
	params, err = json.Marshal(mockEnv(binaryAddr("fred")))
	require.NoError(t, err)
	res, _, err = Migrate(cache, id, params, []byte(`{"verifier":"alice"}`), &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)

	// should update verifier to alice
	data, _, err = Query(cache, id, query, &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	var qres2 types.QueryResponse
	err = json.Unmarshal(data, &qres2)
	require.NoError(t, err)
	require.Nil(t, qres2.Err, "%v", qres2.Err)
	require.Equal(t, string(qres2.Ok), `{"verifier":"alice"}`)
}

func TestMultipleInstances(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	id := createTestContract(t, cache)

	// instance1 controlled by fred
	gasMeter1 := NewMockGasMeter(100000000)
	store1 := NewLookup()
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv(binaryAddr("regen")))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, cost, err := Instantiate(cache, id, params, msg, &gasMeter1, store1, api, &querier, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x11f57), cost)

	// instance2 controlled by mary
	gasMeter2 := NewMockGasMeter(100000000)
	store2 := NewLookup()
	params, err = json.Marshal(mockEnv(binaryAddr("chorus")))
	require.NoError(t, err)
	msg = []byte(`{"verifier": "mary", "beneficiary": "sue"}`)
	res, cost, err = Instantiate(cache, id, params, msg, &gasMeter2, store2, api, &querier, 100000000)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x11f57), cost)

	// fail to execute store1 with mary
	resp := exec(t, cache, id, "mary", store1, api, querier, 0x119df)
	require.Equal(t, resp.Err, &types.StdError{
		Unauthorized: &types.Unauthorized{},
	})

	// succeed to execute store1 with fred
	resp = exec(t, cache, id, "fred", store1, api, querier, 0x1c135)
	require.Nil(t, resp.Err, "%v", resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
	logs := resp.Ok.Log
	require.Equal(t, 2, len(logs))
	require.Equal(t, "destination", logs[1].Key)
	require.Equal(t, "bob", logs[1].Value)

	// succeed to execute store2 with mary
	resp = exec(t, cache, id, "mary", store2, api, querier, 0x1c135)
	require.Nil(t, resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
	logs = resp.Ok.Log
	require.Equal(t, 2, len(logs))
	require.Equal(t, "destination", logs[1].Key)
	require.Equal(t, "sue", logs[1].Value)
}

func requireOkResponse(t *testing.T, res []byte, expectedMsgs int) {
	var resp types.HandleResult
	err := json.Unmarshal(res, &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Err, "%v", resp.Err)
	require.Equal(t, expectedMsgs, len(resp.Ok.Messages))
}

func createTestContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/hackatom.wasm")
}

func createQueueContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/queue.wasm")
}

func createReflectContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/reflect.wasm")
}

func createContract(t *testing.T, cache Cache, wasmFile string) []byte {
	wasm, err := ioutil.ReadFile(wasmFile)
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)
	return id
}

// exec runs the handle tx with the given signer
func exec(t *testing.T, cache Cache, id []byte, signer string, store KVStore, api *GoAPI, querier Querier, gas uint64) types.HandleResult {
	gasMeter := NewMockGasMeter(100000000)
	params, err := json.Marshal(mockEnv(binaryAddr(signer)))
	require.NoError(t, err)
	res, cost, err := Handle(cache, id, params, []byte(`{"release":{}}`), &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	assert.Equal(t, gas, cost)

	var resp types.HandleResult
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	return resp
}

func TestQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	id := createTestContract(t, cache)

	// set up contract
	gasMeter1 := NewMockGasMeter(100000000)
	store := NewLookup()
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv(binaryAddr("creator")))
	require.NoError(t, err)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, id, params, msg, &gasMeter1, store, api, &querier, 100000000)
	require.NoError(t, err)

	// invalid query
	gasMeter2 := NewMockGasMeter(100000000)
	query := []byte(`{"Raw":{"val":"config"}}`)
	data, _, err := Query(cache, id, query, &gasMeter2, store, api, &querier, 100000000)
	require.NoError(t, err)
	var badResp types.QueryResponse
	err = json.Unmarshal(data, &badResp)
	require.NoError(t, err)
	require.Equal(t, badResp.Err, &types.StdError{
		ParseErr: &types.ParseErr{
			Target: "hackatom::contract::QueryMsg",
			Msg:    "unknown variant `Raw`, expected `verifier` or `other_balance`",
		},
	})

	// make a valid query
	gasMeter3 := NewMockGasMeter(100000000)
	query = []byte(`{"verifier":{}}`)
	data, _, err = Query(cache, id, query, &gasMeter3, store, api, &querier, 100000000)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Nil(t, qres.Err, "%v", qres.Err)
	require.Equal(t, string(qres.Ok), `{"verifier":"fred"}`)
}

func TestHackatomQuerier(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	id := createTestContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(100000000)
	store := NewLookup()
	api := NewMockAPI()
	initBalance := types.Coins{types.NewCoin(1234, "ATOM"), types.NewCoin(65432, "ETH")}
	querier := DefaultQuerier("foobar", initBalance)

	// make a valid query to the other address
	query := []byte(`{"other_balance":{"address":"foobar"}}`)
	// TODO The query happens before the contract is initialized. How is this legal?
	data, _, err := Query(cache, id, query, &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Nil(t, qres.Err, "%v", qres.Err)
	var balances types.AllBalancesResponse
	err = json.Unmarshal(qres.Ok, &balances)
	require.Equal(t, balances.Amount, initBalance)
}

func TestCustomReflectQuerier(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	id := createReflectContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(100000000)
	store := NewLookup()
	api := NewMockAPI()
	initBalance := types.Coins{types.NewCoin(1234, "ATOM")}
	querier := DefaultQuerier(mockContractAddr, initBalance)
	// we need this to handle the custom requests from the reflect contract
	innerQuerier := querier.(MockQuerier)
	innerQuerier.Custom = ReflectCustom{}
	querier = Querier(innerQuerier)

	// make a valid query to the other address
	query := []byte(`{"reflect_custom":{"text":"small Frys :)"}}`)
	data, _, err := Query(cache, id, query, &gasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Nil(t, qres.Err, "%v", qres.Err)

	var response CustomResponse
	err = json.Unmarshal(qres.Ok, &response)
	require.Equal(t, response.Msg, "SMALL FRYS :)")
}
