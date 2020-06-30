package api

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
)

/*** Mock GasMeter ****/
// This code is borrowed from Cosmos-SDK store/types/gas.go

// ErrorOutOfGas defines an error thrown when an action results in out of gas.
type ErrorOutOfGas struct {
	Descriptor string
}

// ErrorGasOverflow defines an error thrown when an action results gas consumption
// unsigned integer overflow.
type ErrorGasOverflow struct {
	Descriptor string
}

type MockGasMeter struct {
	limit    Gas
	consumed Gas
}

// NewMockGasMeter returns a reference to a new MockGasMeter.
func NewMockGasMeter(limit Gas) GasMeter {
	return &MockGasMeter{
		limit:    limit,
		consumed: 0,
	}
}

func (g *MockGasMeter) GasConsumed() Gas {
	return g.consumed
}

func (g *MockGasMeter) Limit() Gas {
	return g.limit
}

func (g *MockGasMeter) GasConsumedToLimit() Gas {
	if g.IsPastLimit() {
		return g.limit
	}
	return g.consumed
}

// addUint64Overflow performs the addition operation on two uint64 integers and
// returns a boolean on whether or not the result overflows.
func addUint64Overflow(a, b uint64) (uint64, bool) {
	if math.MaxUint64-a < b {
		return 0, true
	}

	return a + b, false
}

func (g *MockGasMeter) ConsumeGas(amount Gas, descriptor string) {
	var overflow bool
	// TODO: Should we set the consumed field after overflow checking?
	g.consumed, overflow = addUint64Overflow(g.consumed, amount)
	if overflow {
		panic(ErrorGasOverflow{descriptor})
	}

	if g.consumed > g.limit {
		panic(ErrorOutOfGas{descriptor})
	}

}

func (g *MockGasMeter) IsPastLimit() bool {
	return g.consumed > g.limit
}

func (g *MockGasMeter) IsOutOfGas() bool {
	return g.consumed >= g.limit
}

/*** Mock KVStore ****/
// Much of this code is borrowed from Cosmos-SDK store/transient.go

type Lookup struct {
	db *dbm.MemDB
}

func NewLookup() KVStore {
	return Lookup{
		db: dbm.NewMemDB(),
	}
}

// Get wraps the underlying DB's Get method panicing on error.
func (l Lookup) Get(key []byte) []byte {
	v, err := l.db.Get(key)
	if err != nil {
		panic(err)
	}

	return v
}

// Set wraps the underlying DB's Set method panicing on error.
func (l Lookup) Set(key, value []byte) {
	if err := l.db.Set(key, value); err != nil {
		panic(err)
	}
}

// Delete wraps the underlying DB's Delete method panicing on error.
func (l Lookup) Delete(key []byte) {
	if err := l.db.Delete(key); err != nil {
		panic(err)
	}
}

// Iterator wraps the underlying DB's Iterator method panicing on error.
func (l Lookup) Iterator(start, end []byte) dbm.Iterator {
	iter, err := l.db.Iterator(start, end)
	if err != nil {
		panic(err)
	}

	return iter
}

// ReverseIterator wraps the underlying DB's ReverseIterator method panicing on error.
func (l Lookup) ReverseIterator(start, end []byte) dbm.Iterator {
	iter, err := l.db.ReverseIterator(start, end)
	if err != nil {
		panic(err)
	}

	return iter
}

// type Lookup struct {
// 	data map[string]string
// }
//
//
// func (l *Lookup) Get(key []byte) []byte {
// 	val := l.data[string(key)]
// 	return []byte(val)
// }
//
// func (l *Lookup) Set(key, value []byte) {
// 	l.data[string(key)] = string(value)
// }
//
// func (l *Lookup) Delete(key []byte) {
// 	delete(l.data, string(key))
// }

var _ KVStore = (*Lookup)(nil)

/***** Mock GoAPI ****/

const CanonicalLength = 32

func MockCanonicalAddress(human string) ([]byte, error) {
	if len(human) > CanonicalLength {
		return nil, fmt.Errorf("human encoding too long")
	}
	res := make([]byte, CanonicalLength)
	copy(res, []byte(human))
	return res, nil
}

func MockHumanAddress(canon []byte) (string, error) {
	if len(canon) != CanonicalLength {
		return "", fmt.Errorf("wrong canonical length")
	}
	cut := CanonicalLength
	for i, v := range canon {
		if v == 0 {
			cut = i
			break
		}
	}
	human := string(canon[:cut])
	return human, nil
}

func NewMockAPI() *GoAPI {
	return &GoAPI{
		HumanAddress:     MockHumanAddress,
		CanonicalAddress: MockCanonicalAddress,
	}
}

func TestMockApi(t *testing.T) {
	human := "foobar"
	canon, err := MockCanonicalAddress(human)
	require.NoError(t, err)
	assert.Equal(t, CanonicalLength, len(canon))

	recover, err := MockHumanAddress(canon)
	require.NoError(t, err)
	assert.Equal(t, recover, human)
}

/**** MockQuerier ****/

type MockQuerier struct {
	Bank    BankQuerier
	Custom  CustomQuerier
	usedGas uint64
}

var _ types.Querier = MockQuerier{}

func DefaultQuerier(contractAddr string, coins types.Coins) Querier {
	balances := map[string]types.Coins{
		contractAddr: coins,
	}
	return MockQuerier{
		Bank:    NewBankQuerier(balances),
		Custom:  NoCustom{},
		usedGas: 0,
	}
}

func (q MockQuerier) Query(request types.QueryRequest) ([]byte, error) {
	marshaled, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	q.usedGas += uint64(len(marshaled))
	if request.Bank != nil {
		return q.Bank.Query(request.Bank)
	}
	if request.Custom != nil {
		return q.Custom.Query(request.Custom)
	}
	if request.Staking != nil {
		return nil, types.UnsupportedRequest{"staking"}
	}
	if request.Wasm != nil {
		return nil, types.UnsupportedRequest{"wasm"}
	}
	return nil, types.Unknown{}
}

func (q MockQuerier) GasConsumed() uint64 {
	return q.usedGas
}

type BankQuerier struct {
	Balances map[string]types.Coins
}

func NewBankQuerier(balances map[string]types.Coins) BankQuerier {
	bal := make(map[string]types.Coins, len(balances))
	for k, v := range balances {
		dst := make([]types.Coin, len(v))
		copy(dst, v)
		bal[k] = dst
	}
	return BankQuerier{
		Balances: bal,
	}
}

func (q BankQuerier) Query(request *types.BankQuery) ([]byte, error) {
	if request.Balance != nil {
		denom := request.Balance.Denom
		var coin = types.NewCoin(0, denom)
		for _, c := range q.Balances[request.Balance.Address] {
			if c.Denom == denom {
				coin = c
			}
		}
		resp := types.BalanceResponse{
			Amount: coin,
		}
		return json.Marshal(resp)
	}
	if request.AllBalances != nil {
		coins := q.Balances[request.AllBalances.Address]
		resp := types.AllBalancesResponse{
			Amount: coins,
		}
		return json.Marshal(resp)
	}
	return nil, types.UnsupportedRequest{"Empty BankQuery"}
}

type CustomQuerier interface {
	Query(request json.RawMessage) ([]byte, error)
}

type NoCustom struct{}

var _ CustomQuerier = NoCustom{}

func (q NoCustom) Query(request json.RawMessage) ([]byte, error) {
	return nil, types.UnsupportedRequest{"custom"}
}

// ReflectCustom fulfills the requirements for testing `reflect` contract
type ReflectCustom struct{}

var _ CustomQuerier = ReflectCustom{}

type CustomQuery struct {
	Ping    *struct{}     `json:"ping,omitempty"`
	Capital *CapitalQuery `json:"capital,omitempty"`
}

type CapitalQuery struct {
	Text string `json:"text"`
}

type CustomResponse struct {
	Msg string `json:"msg"`
}

func (q ReflectCustom) Query(request json.RawMessage) ([]byte, error) {
	var query CustomQuery
	err := json.Unmarshal(request, &query)
	if err != nil {
		return nil, types.ParseErr{
			Target: "CustomQuery",
			Msg:    err.Error(),
		}
	}
	var resp CustomResponse
	if query.Ping != nil {
		resp.Msg = "PONG"
	} else if query.Capital != nil {
		resp.Msg = strings.ToUpper(query.Capital.Text)
	}
	return json.Marshal(resp)
}

//************ test code for mocks *************************//

func TestBankQuerierAllBalances(t *testing.T) {
	addr := "foobar"
	balance := types.Coins{types.NewCoin(12345678, "ATOM"), types.NewCoin(54321, "ETH")}
	q := DefaultQuerier(addr, balance)

	// query existing account
	req := types.QueryRequest{
		Bank: &types.BankQuery{
			AllBalances: &types.AllBalancesQuery{
				Address: addr,
			},
		},
	}
	res, err := q.Query(req)
	require.NoError(t, err)
	var resp types.AllBalancesResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Amount, balance)

	// query missing account
	req2 := types.QueryRequest{
		Bank: &types.BankQuery{
			AllBalances: &types.AllBalancesQuery{
				Address: "someone-else",
			},
		},
	}
	res, err = q.Query(req2)
	require.NoError(t, err)
	var resp2 types.AllBalancesResponse
	err = json.Unmarshal(res, &resp2)
	require.NoError(t, err)
	assert.Nil(t, resp2.Amount)
}

func TestBankQuerierBalance(t *testing.T) {
	addr := "foobar"
	balance := types.Coins{types.NewCoin(12345678, "ATOM"), types.NewCoin(54321, "ETH")}
	q := DefaultQuerier(addr, balance)

	// query existing account with matching denom
	req := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: addr,
				Denom:   "ATOM",
			},
		},
	}
	res, err := q.Query(req)
	require.NoError(t, err)
	var resp types.BalanceResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Amount, types.NewCoin(12345678, "ATOM"))

	// query existing account with missing denom
	req2 := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: addr,
				Denom:   "BTC",
			},
		},
	}
	res, err = q.Query(req2)
	require.NoError(t, err)
	var resp2 types.BalanceResponse
	err = json.Unmarshal(res, &resp2)
	require.NoError(t, err)
	assert.Equal(t, resp2.Amount, types.NewCoin(0, "BTC"))

	// query missing account
	req3 := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: "someone-else",
				Denom:   "ATOM",
			},
		},
	}
	res, err = q.Query(req3)
	require.NoError(t, err)
	var resp3 types.BalanceResponse
	err = json.Unmarshal(res, &resp3)
	require.NoError(t, err)
	assert.Equal(t, resp3.Amount, types.NewCoin(0, "ATOM"))
}

func TestReflectCustomQuerier(t *testing.T) {
	q := ReflectCustom{}

	// try ping
	msg, err := json.Marshal(CustomQuery{Ping: &struct{}{}})
	require.NoError(t, err)
	bz, err := q.Query(msg)
	require.NoError(t, err)
	var resp CustomResponse
	err = json.Unmarshal(bz, &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Msg, "PONG")

	// try captial
	msg2, err := json.Marshal(CustomQuery{Capital: &CapitalQuery{Text: "small."}})
	require.NoError(t, err)
	bz, err = q.Query(msg2)
	require.NoError(t, err)
	var resp2 CustomResponse
	err = json.Unmarshal(bz, &resp2)
	require.NoError(t, err)
	assert.Equal(t, resp2.Msg, "SMALL.")
}
