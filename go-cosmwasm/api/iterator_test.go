package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	"github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
)

type queueData struct {
	id      []byte
	store   *Lookup
	api     *GoAPI
	querier types.Querier
}

func (q queueData) Store(meter MockGasMeter) KVStore {
	return q.store.WithGasMeter(meter)
}

func setupQueueContractWithData(t *testing.T, cache Cache, values ...int) queueData {
	id := createQueueContract(t, cache)

	gasMeter1 := NewMockGasMeter(100000000)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(mockContractAddr, types.Coins{types.NewCoin(100, "ATOM")})
	params, err := json.Marshal(mockEnv("creator"))
	require.NoError(t, err)
	msg := []byte(`{}`)

	igasMeter1 := GasMeter(gasMeter1)
	res, _, err := Instantiate(cache, id, params, msg, &igasMeter1, store, api, &querier, 100000000, nil)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	for _, value := range values {
		// push 17
		var gasMeter2 GasMeter = NewMockGasMeter(100000000)
		push := []byte(fmt.Sprintf(`{"enqueue":{"value":%d}}`, value))
		res, _, err = Handle(cache, id, params, push, &gasMeter2, store, api, &querier, 100000000, nil)
		require.NoError(t, err)
		requireOkResponse(t, res, 0)
	}

	return queueData{
		id:      id,
		store:   store,
		api:     api,
		querier: querier,
	}
}

func setupQueueContract(t *testing.T, cache Cache) queueData {
	return setupQueueContractWithData(t, cache, 17, 22)
}

func TestQueueIterator(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	setup := setupQueueContract(t, cache)
	id, querier, api := setup.id, setup.querier, setup.api

	// query the sum
	gasMeter := NewMockGasMeter(100000000)
	igasMeter := GasMeter(gasMeter)
	store := setup.Store(gasMeter)
	query := []byte(`{"sum":{}}`)
	data, _, err := Query(cache, id, query, &igasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Nil(t, qres.Err, "%v", qres.Err)
	require.Equal(t, string(qres.Ok), `{"sum":39}`)

	// query reduce (multiple iterators at once)
	query = []byte(`{"reducer":{}}`)
	data, _, err = Query(cache, id, query, &igasMeter, store, api, &querier, 100000000)
	require.NoError(t, err)
	var reduced types.QueryResponse
	err = json.Unmarshal(data, &reduced)
	require.NoError(t, err)
	require.Nil(t, reduced.Err, "%v", reduced.Err)
	require.Equal(t, string(reduced.Ok), `{"counters":[[17,22],[22,0]]}`)
}

func TestQueueIteratorRaces(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	assert.Equal(t, len(iteratorStack), 0)

	contract1 := setupQueueContractWithData(t, cache, 17, 22)
	contract2 := setupQueueContractWithData(t, cache, 1, 19, 6, 35, 8)
	contract3 := setupQueueContractWithData(t, cache, 11, 6, 2)

	reduceQuery := func(t *testing.T, setup queueData, expected string) {
		id, querier, api := setup.id, setup.querier, setup.api
		gasMeter := NewMockGasMeter(100000000)
		igasMeter := GasMeter(gasMeter)
		store := setup.Store(gasMeter)

		// query reduce (multiple iterators at once)
		query := []byte(`{"reducer":{}}`)
		data, _, err := Query(cache, id, query, &igasMeter, store, api, &querier, 100000000)
		require.NoError(t, err)
		var reduced types.QueryResponse
		err = json.Unmarshal(data, &reduced)
		require.NoError(t, err)
		require.Nil(t, reduced.Err, "%v", reduced.Err)
		require.Equal(t, string(reduced.Ok), fmt.Sprintf(`{"counters":%s}`, expected))
	}

	// 30 concurrent batches (in go routines) to trigger any race condition
	numBatches := 30

	var wg sync.WaitGroup
	// for each batch, query each of the 3 contracts - so the contract queries get mixed together
	wg.Add(numBatches * 3)
	for i := 0; i < numBatches; i++ {
		go func() {
			reduceQuery(t, contract1, "[[17,22],[22,0]]")
			wg.Done()
		}()
		go func() {
			reduceQuery(t, contract2, "[[1,68],[19,35],[6,62],[35,0],[8,54]]")
			wg.Done()
		}()
		go func() {
			reduceQuery(t, contract3, "[[11,0],[6,11],[2,17]]")
			wg.Done()
		}()
	}
	wg.Wait()

	// when they finish, we should have popped everything off the stack
	assert.Equal(t, len(iteratorStack), 0)
}
