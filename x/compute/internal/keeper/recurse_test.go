package keeper

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	wasmTypes "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

type Recurse struct {
	Depth            uint32         `json:"depth"`
	Work             uint32         `json:"work"`
	Contract         sdk.AccAddress `json:"contract"`
	ContractCodeHash string         `json:"contract_code_hash"`
}

type recurseWrapper struct {
	Recurse Recurse `json:"recurse"`
}

func buildQuery(t *testing.T, msg Recurse, codeHash string) []byte {
	wrapper := recurseWrapper{Recurse: msg}
	wrapper.Recurse.ContractCodeHash = codeHash
	bz, err := json.Marshal(wrapper)
	require.NoError(t, err)
	return bz
}

type recurseResponse struct {
	Hashed []byte `json:"hashed"`
}

// number os wasm queries called from a contract
var totalWasmQueryCounter int

func initRecurseContract(t *testing.T) (contract sdk.AccAddress, creator sdk.AccAddress, ctx sdk.Context, keeper Keeper) {
	var realWasmQuerier func(ctx sdk.Context, request *wasmTypes.WasmQuery) ([]byte, error)
	countingQuerier := &QueryPlugins{
		Wasm: func(ctx sdk.Context, request *wasmTypes.WasmQuery) ([]byte, error) {
			totalWasmQueryCounter++
			return realWasmQuerier(ctx, request)
		},
	}
	encoders := DefaultEncoders()
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, countingQuerier)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper
	realWasmQuerier = WasmQuerier(&keeper)

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, creatorPriv := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit.Add(deposit...))

	// store the code
	wasmCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)
	codeID, err := keeper.Create(ctx, creator, wasmCode, "", "")
	require.NoError(t, err)

	// instantiate the contract
	_, _, bob := keyPubAddr()
	_, _, fred := keyPubAddr()
	initMsg := InitMsg{
		Verifier:    fred,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, creator, creatorPriv, string(initMsgBz), true, defaultGasForTests)
	require.Empty(t, initErr)

	return contractAddr, creator, ctx, keeper
}

func TestGasCostOnQuery(t *testing.T) {
	const (
		GasNoWork uint64 = types.InstanceCost + 2_756
		// Note: about 100 SDK gas (10k wasmer gas) for each round of sha256
		GasWork50 uint64 = types.InstanceCost + 8_464 // this is a little shy of 50k gas - to keep an eye on the limit

		GasReturnUnhashed uint64 = 647
		GasReturnHashed   uint64 = 597
	)

	cases := map[string]struct {
		gasLimit    uint64
		msg         Recurse
		expectedGas uint64
	}{
		"no recursion, no work": {
			gasLimit:    400_000,
			msg:         Recurse{},
			expectedGas: GasNoWork,
		},
		"no recursion, some work": {
			gasLimit: 400_000,
			msg: Recurse{
				Work: 50, // 50 rounds of sha256 inside the contract
			},
			expectedGas: GasWork50,
		},
		"recursion 1, no work": {
			gasLimit: 400_000,
			msg: Recurse{
				Depth: 1,
			},
			expectedGas: 2*GasNoWork + GasReturnUnhashed,
		},
		"recursion 1, some work": {
			gasLimit: 400_000,
			msg: Recurse{
				Depth: 1,
				Work:  50,
			},
			expectedGas: 2*GasWork50 + GasReturnHashed,
		},
		"recursion 4, some work": {
			gasLimit: 400_000,
			msg: Recurse{
				Depth: 4,
				Work:  50,
			},
			// this is (currently) 244_708 gas
			expectedGas: 5*GasWork50 + 4*GasReturnHashed,
		},
	}

	contractAddr, creator, ctx, keeper := initRecurseContract(t)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// external limit has no effect (we get a panic if this is enforced)
			keeper.queryGasLimit = 1000

			// make sure we set a limit before calling
			ctx = ctx.WithGasMeter(sdk.NewGasMeter(tc.gasLimit))
			require.Equal(t, uint64(0), ctx.GasMeter().GasConsumed())

			// do the query
			recurse := tc.msg
			recurse.Contract = contractAddr

			msg := buildQuery(t, recurse, hex.EncodeToString(keeper.GetContractHash(ctx, contractAddr)))

			data, qErr := queryHelper(t, keeper, ctx, contractAddr, string(msg), true, tc.gasLimit)
			require.Empty(t, qErr)

			// check the gas is what we expected
			if tc.expectedGas == 0 {
				assert.Equal(t, ctx.GasMeter().GasConsumed(), uint64(0))
			} else {
				assert.Greater(t, ctx.GasMeter().GasConsumed(), uint64(0))
			}

			// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
			var resp recurseResponse
			err := json.Unmarshal([]byte(data), &resp)
			require.NoError(t, err)
			if recurse.Work == 0 {
				assert.Equal(t, len(resp.Hashed), len(creator.String()))
			} else {
				assert.Equal(t, len(resp.Hashed), 32)
			}
		})
	}
}

func TestGasOnExternalQuery(t *testing.T) {
	const (
		// todo: tune gas numbers
		GasWork50 uint64 = types.InstanceCost + 846
	)

	cases := map[string]struct {
		gasLimit    uint64
		msg         Recurse
		expectPanic bool
	}{
		"no recursion, plenty gas": {
			gasLimit: 400_000,
			msg: Recurse{
				Work: 50, // 50 rounds of sha256 inside the contract
			},
			expectPanic: false,
		},
		"recursion 4, plenty gas": {
			// this uses 244708 gas
			gasLimit: 400_000,
			msg: Recurse{
				Depth: 4,
				Work:  50,
			},
			expectPanic: false,
		},
		"no recursion, external gas limit": {
			gasLimit: 5000, // this is not enough
			msg: Recurse{
				Work: 50,
			},
			expectPanic: true,
		},
		"recursion 4, external gas limit": {
			// this uses 244708 gas but give less
			gasLimit: GasWork50,
			msg: Recurse{
				Depth: 4,
				Work:  50,
			},
			expectPanic: true,
		},
	}

	contractAddr, _, ctx, keeper := initRecurseContract(t)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			keeper.queryGasLimit = tc.gasLimit

			recurse := tc.msg
			recurse.Contract = contractAddr
			msg := buildQuery(t, recurse, hex.EncodeToString(keeper.GetContractHash(ctx, contractAddr)))

			secretMsg := types.SecretMsg{
				CodeHash: []byte(hex.EncodeToString(keeper.GetContractHash(ctx, contractAddr))),
				Msg:      msg,
			}

			msg, err := wasmCtx.Encrypt(secretMsg.Serialize())
			require.NoError(t, err)

			// do the query
			path := []string{QueryGetContractState, contractAddr.String(), QueryMethodContractStateSmart}
			req := abci.RequestQuery{Data: msg}
			if tc.expectPanic {
				require.Panics(t, func() {
					// this should run out of gas
					_, err := NewLegacyQuerier(keeper)(ctx, path, req)
					t.Logf("%v", err)
				})
			} else {
				// otherwise, make sure we get a good success
				_, err := NewLegacyQuerier(keeper)(ctx, path, req)
				require.NoError(t, err)
			}
		})
	}
}

func TestLimitRecursiveQueryGas(t *testing.T) {
	// The point of this test from https://github.com/CosmWasm/cosmwasm/issues/456
	// Basically, if I burn 90% of gas in CPU loop, then query out (to my self)
	// the sub-query will have all the original gas (minus the 40k instance charge)
	// and can burn 90% and call a sub-contract again...
	// This attack would allow us to use far more than the provided gas before
	// eventually hitting an OutOfGas panic.

	const (
		// Note: about 100 SDK gas (10k wasmer gas) for each round of sha256
		GasWork2k uint64 = types.InstanceCost + 233_379 // we have 6x gas used in cpu than in the instance
		// This is overhead for calling into a sub-contract
		GasReturnHashed uint64 = 603
	)

	cases := map[string]struct {
		gasLimit                  uint64
		msg                       Recurse
		expectQueriesFromContract int
		expectedGas               uint64
		expectOutOfGas            bool
		expectOOM                 bool
		expectRecursionLimit      bool
	}{
		"no recursion, lots of work": {
			gasLimit: 4_000_000,
			msg: Recurse{
				Depth: 0,
				Work:  2000,
			},
			expectQueriesFromContract: 0,
			expectedGas:               GasWork2k,
		},
		"recursion 4, lots of work": {
			gasLimit: 4_000_000,
			msg: Recurse{
				Depth: 4,
				Work:  2000,
			},
			expectQueriesFromContract: 4,
			expectedGas:               GasWork2k + 9*(GasWork2k+GasReturnHashed),
			expectOutOfGas:            false,
			expectOOM:                 false,
			expectRecursionLimit:      false,
		},
		"recursion 9, lots of work": {
			gasLimit: 4_000_000,
			msg: Recurse{
				Depth: 9,
				Work:  2000,
			},
			expectQueriesFromContract: 9,
			expectedGas:               GasWork2k + 9*(GasWork2k+GasReturnHashed),
			expectOutOfGas:            false,
			expectOOM:                 false,
			expectRecursionLimit:      true,
		},
		// this is where we expect an error...
		// it has enough gas to run 4 times and die on the 5th (4th time dispatching to sub-contract)
		// however, if we don't charge the cpu gas before sub-dispatching, we can recurse over 20 times
		// TODO: figure out how to asset how deep it went
		"deep recursion, should die on 5th level": {
			gasLimit: 1_200_000,
			msg: Recurse{
				Depth: 50,
				Work:  2000,
			},
			expectQueriesFromContract: 4,
			expectOutOfGas:            false,
			expectOOM:                 false,
			expectRecursionLimit:      true,
		},
	}

	contractAddr, _, ctx, keeper := initRecurseContract(t)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// reset the counter before test
			totalWasmQueryCounter = 0

			// make sure we set a limit before calling
			ctx = ctx.WithGasMeter(sdk.NewGasMeter(tc.gasLimit))
			require.Equal(t, uint64(0), ctx.GasMeter().GasConsumed())

			// prepare the query
			recurse := tc.msg
			recurse.Contract = contractAddr
			msg := buildQuery(t, recurse, hex.EncodeToString(keeper.GetContractHash(ctx, contractAddr)))

			// if we expect out of gas, make sure this panics
			if tc.expectOutOfGas {
				require.Panics(t, func() {
					_, qErr := queryHelper(t, keeper, ctx, contractAddr, string(msg), true, tc.gasLimit)
					t.Logf("Got error not panic: %#v", qErr)
				})
				assert.Equal(t, tc.expectQueriesFromContract, totalWasmQueryCounter)
				return
			}

			// if we expect out of memory
			if tc.expectOOM {
				_, qErr := queryHelper(t, keeper, ctx, contractAddr, string(msg), true, tc.gasLimit)
				require.NotEmpty(t, qErr)
				require.NotNil(t, qErr.GenericErr)
				require.Contains(t, qErr.GenericErr.Msg, "Execution error: Enclave: enclave ran out of heap memory")
				return
			}

			if tc.expectRecursionLimit {
				_, qErr := queryHelper(t, keeper, ctx, contractAddr, string(msg), true, tc.gasLimit)
				require.NotEmpty(t, qErr)
				require.NotNil(t, qErr.GenericErr)
				require.Contains(t, qErr.GenericErr.Msg, "Querier system error: Query recursion limit exceeded")
				return
			}

			// otherwise, we expect a successful call
			_, qErr := queryHelper(t, keeper, ctx, contractAddr, string(msg), true, tc.gasLimit)
			require.Empty(t, qErr)
			// assert.Equal(t, tc.expectedGas, ctx.GasMeter().GasConsumed())

			assert.Equal(t, tc.expectQueriesFromContract, totalWasmQueryCounter)
		})
	}
}
