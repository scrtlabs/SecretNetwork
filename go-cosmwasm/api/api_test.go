package api

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
)

func TestCanonicalAddressFailure(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv("creator"))
	require.NoError(t, err)

	// if the human address is larger than 32 bytes, this will lead to an error in the go side
	longName := "long123456789012345678901234567890long"
	msg := []byte(`{"verifier": "` + longName + `", "beneficiary": "bob"}`)

	// make sure the call doesn't error, but we get a JSON-encoded error result from InitResult
	igasMeter := GasMeter(gasMeter)
	res, _, err := Instantiate(cache, id, params, msg, &igasMeter, store, api, &querier, 100000000, nil)
	require.NoError(t, err)
	var resp types.InitResult
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)

	// ensure the error message is what we expect
	require.NotNil(t, resp.Err)
	require.Nil(t, resp.Ok)
	// expect a generic message
	require.NotNil(t, resp.Err.GenericErr)
	// with this message
	require.Equal(t, resp.Err.Error(), "generic: canonicalize_address errored: human encoding too long")
}

func TestHumanAddressFailure(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	id, err := Create(cache, wasm)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv("creator"))
	require.NoError(t, err)

	// instantiate it normally
	msg := []byte(`{"verifier": "short", "beneficiary": "bob"}`)
	igasMeter := GasMeter(gasMeter)
	_, _, err = Instantiate(cache, id, params, msg, &igasMeter, store, api, &querier, 100000000, nil)
	require.NoError(t, err)

	// call query which will call canonicalize address
	badApi := NewMockFailureAPI()
	gasMeter3 := NewMockGasMeter(100000000)
	query := []byte(`{"verifier":{}}`)
	igasMeter3 := GasMeter(gasMeter3)
	res, _, err := Query(cache, id, query, &igasMeter3, store, badApi, &querier, 100000000)
	require.NoError(t, err)
	var resp types.QueryResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)

	// ensure the error message is what we expect (system -ok, stderr -generic)
	require.Nil(t, resp.Ok)
	require.NotNil(t, resp.Err)
	// expect a generic message
	require.NotNil(t, resp.Err.GenericErr)
	// with this message
	require.Equal(t, resp.Err.Error(), "generic: humanize_address errored: mock failure - human_address")
}
