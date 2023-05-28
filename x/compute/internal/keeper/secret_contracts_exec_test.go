package keeper

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"

	v010types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	"golang.org/x/exp/slices"

	cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat/combin"

	crypto "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	ibcchanneltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
)

func setupChainTest(t *testing.T, wasmPath string, additionalCoinsInWallets sdk.Coins, amount uint64) (sdk.Context, Keeper, []uint64, []string, sdk.AccAddress, crypto.PrivKey, sdk.AccAddress, crypto.PrivKey) {
	ctx, keeper, walletA, privKeyA, walletB, privKeyB := setupBasicTest(t, additionalCoinsInWallets)

	codeIds := make([]uint64, amount)
	codeHashes := make([]string, amount)
	for i := uint64(0); i < amount; i++ {
		codeIds[i], codeHashes[i] = uploadChainCode(ctx, t, keeper, wasmPath, walletA, i)
	}

	return ctx, keeper, codeIds, codeHashes, walletA, privKeyA, walletB, privKeyB
}

func stringToCoins(balance string) sdk.Coins {
	result := sdk.NewCoins()

	individualCoins := strings.Split(balance, ",")
	for _, coin := range individualCoins {
		if coin == "" {
			continue
		}
		var amount int64
		var denom string
		_, _ = fmt.Sscanf(coin, "%d%s", &amount, &denom)
		result = result.Add(sdk.NewInt64Coin(denom, amount))
	}

	return result
}

func CoinsToInput(coins sdk.Coins) string {
	result := "["
	for i, coin := range coins {
		result += `{"amount":"`
		result += coin.Amount.String()
		result += `","denom":"`
		result += coin.Denom
		result += `"}`
		if i != len(coins)-1 {
			result += `,`
		}
	}
	result += "]"

	return result
}

func multisetsFrom[t any](elements []t, size int) [][]t {
	// init slice with 'size' elements, each one being the number of elements in the input slice
	lengths := make([]int, size)
	length := len(elements)
	for i := 0; i < len(lengths); i++ {
		lengths[i] = length
	}

	product := combin.Cartesian(lengths)
	// map indexes to element
	result := make([][]t, 0)
	for _, indexes := range product {
		inner := make([]t, 0)
		for _, j := range indexes {
			inner = append(inner, elements[j])
		}
		result = append(result, inner)
	}

	return result
}

func calcCodeHash(wasmPath string) string {
	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		panic(fmt.Sprintf("calcCodeHash: %+v", err))
	}

	h := sha256.New()

	h.Write(wasmCode)

	return hex.EncodeToString(h.Sum(nil))
}

type ExecuteDetails struct {
	ContractAddress string `json:"contract_address"`
	ContractHash    string `json:"contract_hash"`
	ShouldError     bool   `json:"should_error"`
	MsgId           uint64 `json:"msg_id"`
	Data            string `json:"data"`
}

func TestState(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Empty(t, data)

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(data))

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"remove_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Empty(t, data)
		})
	}
}

func TestAddrValidateFunction(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, v1ContractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, v1ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"validate_address":{"addr":"%s"}}`, v1ContractAddress), true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	resp, aErr := sdk.AccAddressFromBech32(string(data))
	require.Empty(t, aErr)

	require.Equal(t, resp, v1ContractAddress)

	_, _, data, _, _, err = execHelper(t, keeper, ctx, v1ContractAddress, walletA, privKeyA, fmt.Sprintf(`{"validate_address":{"addr":"secret18vd8fpwxzck93qlwghaj6arh4p7c5nyf7hmag8"}}`), true, true, defaultGasForTests, 0)
	require.Equal(t, string(data), "\"Apple\"")
}

func TestRandomEnv(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[randomContract], sdk.NewCoins())

	_, _, contractAddress, initEvents, initErr := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"get_env":{}}`, true, true, defaultGasForTests, -1, sdk.NewCoins(sdk.NewInt64Coin("denom", 1)))
	require.Empty(t, initErr)
	require.Len(t, initEvents, 1)

	initEvent := initEvents[0]
	envAttributeIndex := slices.IndexFunc(initEvent, func(c v010types.LogAttribute) bool { return c.Key == "env" })
	envAttribute := initEvent[envAttributeIndex]

	var actualMessageInfo cosmwasm.Env
	json.Unmarshal([]byte(envAttribute.Value), &actualMessageInfo)

	// var firstRandom string
	require.Len(t, actualMessageInfo.Block.Random, 32)

	expectedV1Env := fmt.Sprintf(
		`{"block":{"height":%d,"time":"%d","chain_id":"%s","random":"%s"},"transaction":null,"contract":{"address":"%s","code_hash":"%s"}}`,
		ctx.BlockHeight(),
		// env.block.time is nanoseconds since unix epoch
		ctx.BlockTime().UnixNano(),
		ctx.ChainID(),
		base64.StdEncoding.EncodeToString(actualMessageInfo.Block.Random),
		contractAddress.String(),
		calcCodeHash(TestContractPaths[randomContract]),
	)
	//
	requireEventsInclude(t,
		initEvents,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{
					Key:   "env",
					Value: expectedV1Env,
				},
			},
		},
	)

	//
	_, _, _, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_env":{}}`, true, true, defaultGasForTests, 1)
	require.Empty(t, execErr)

	execEvent := execEvents[0]
	envAttributeIndex = slices.IndexFunc(execEvent, func(c v010types.LogAttribute) bool { return c.Key == "env" })
	envAttribute = execEvent[envAttributeIndex]

	var actualExecEnv cosmwasm.Env
	json.Unmarshal([]byte(envAttribute.Value), &actualExecEnv)

	expectedV1EnvExec := fmt.Sprintf(
		`{"block":{"height":%d,"time":"%d","chain_id":"%s","random":"%s"},"transaction":null,"contract":{"address":"%s","code_hash":"%s"}}`,
		ctx.BlockHeight(),
		// env.block.time is nanoseconds since unix epoch
		ctx.BlockTime().UnixNano(),
		ctx.ChainID(),
		base64.StdEncoding.EncodeToString(actualExecEnv.Block.Random),
		contractAddress.String(),
		calcCodeHash(TestContractPaths[randomContract]),
	)

	requireEventsInclude(t,
		execEvents,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{
					Key:   "env",
					Value: expectedV1EnvExec,
				},
			},
		},
	)
}

func TestEnv(t *testing.T) {
	type ReturnedV1MessageInfo struct {
		Sender    cosmwasm.HumanAddress `json:"sender"`
		SentFunds cosmwasm.Coins        `json:"funds"`
		// Random    string                `json:"random"`
	}

	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"get_env":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins(sdk.NewInt64Coin("denom", 1)))
			require.Empty(t, initErr)
			require.Len(t, initEvents, 1)

			// var firstRandom string

			expectedV1Env := fmt.Sprintf(
				`{"block":{"height":%d,"time":"%d","chain_id":"%s"},"transaction":null,"contract":{"address":"%s","code_hash":"%s"}}`,
				ctx.BlockHeight(),
				// env.block.time is nanoseconds since unix epoch
				ctx.BlockTime().UnixNano(),
				ctx.ChainID(),
				contractAddress.String(),
				calcCodeHash(testContract.WasmFilePath),
			)

			if testContract.IsCosmWasmV1 {
				requireEventsInclude(t,
					initEvents,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{
								Key:   "env",
								Value: expectedV1Env,
							},
						},
					},
				)

				initEvent := initEvents[0]
				infoLogAttributeIndex := slices.IndexFunc(initEvent, func(c v010types.LogAttribute) bool { return c.Key == "info" })
				infoLogAttribute := initEvent[infoLogAttributeIndex]

				var actualMessageInfo ReturnedV1MessageInfo
				_ = json.Unmarshal([]byte(infoLogAttribute.Value), &actualMessageInfo)

				require.Equal(t, walletA.String(), actualMessageInfo.Sender)
				require.Equal(t, cosmwasm.Coins{{Denom: "denom", Amount: "1"}}, actualMessageInfo.SentFunds)

				// disabling random tests for now
				// require.Len(t, actualMessageInfo.Random, 44)
				// firstRandom = actualMessageInfo.Random
			} else {
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{
								Key: "env",
								Value: fmt.Sprintf(
									`{"block":{"height":%d,"time":%d,"chain_id":"%s"},"message":{"sender":"%s","sent_funds":[{"denom":"denom","amount":"1"}]},"contract":{"address":"%s"},"contract_key":"","contract_code_hash":"%s"}`,
									ctx.BlockHeight(),
									// env.block.time is seconds since unix epoch
									ctx.BlockTime().Unix(),
									ctx.ChainID(),
									walletA.String(),
									contractAddress.String(),
									calcCodeHash(testContract.WasmFilePath),
								),
							},
						},
					},
					initEvents,
				)
			}
			_, _, _, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_env":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 1)
			require.Empty(t, execErr)

			if testContract.IsCosmWasmV1 {
				requireEventsInclude(t,
					execEvents,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{
								Key:   "env",
								Value: expectedV1Env,
							},
						},
					},
				)

				execEvent := execEvents[0]
				infoLogAttributeIndex := slices.IndexFunc(execEvent, func(c v010types.LogAttribute) bool { return c.Key == "info" })
				infoLogAttribute := execEvent[infoLogAttributeIndex]

				var actualMessageInfo ReturnedV1MessageInfo
				_ = json.Unmarshal([]byte(infoLogAttribute.Value), &actualMessageInfo)

				require.Equal(t, walletA.String(), actualMessageInfo.Sender)
				require.Equal(t, cosmwasm.Coins{{Denom: "denom", Amount: "1"}}, actualMessageInfo.SentFunds)

				// disabling random tests
				// require.Len(t, actualMessageInfo.Random, 44)
				// require.NotEqual(t, firstRandom, actualMessageInfo.Random)
			} else {
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{
								Key: "env",
								Value: fmt.Sprintf(
									`{"block":{"height":%d,"time":%d,"chain_id":"%s"},"message":{"sender":"%s","sent_funds":[{"denom":"denom","amount":"1"}]},"contract":{"address":"%s"},"contract_key":"%s","contract_code_hash":"%s"}`,
									ctx.BlockHeight(),
									// env.block.time is seconds since unix epoch
									ctx.BlockTime().Unix(),
									ctx.ChainID(),
									walletA.String(),
									contractAddress.String(),
									base64.StdEncoding.EncodeToString(keeper.GetContractKey(ctx, contractAddress)),
									calcCodeHash(testContract.WasmFilePath),
								),
							},
						},
					},
					execEvents,
				)
			}

			if testContract.IsCosmWasmV1 {
				// only env (no msg info) in v1 query
				queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get_env":{}}`, true, false, math.MaxUint64)
				require.Empty(t, qErr)

				require.Equal(t, expectedV1Env, queryRes)
			} else {
				// no env or msg info in v0.10 query
			}
		})
	}
}

func TestNestedAttribute(t *testing.T) {
	// For more reference: https://github.com/scrtlabs/SecretNetwork/issues/1235
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, v1ContractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, data, events, _, err := execHelper(t, keeper, ctx, v1ContractAddress, walletA, privKeyA, `{"add_attribute_step1":{}}`, true, true, 10*defaultGasForTests, 0)
	require.Empty(t, err)

	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr1", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr2", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr3", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr4", Value: "🦄"},
			},
			{
				{Key: "contract_address", Value: v1ContractAddress.String()},
				{Key: "attr_reply", Value: "🦄"},
			},
		},
		events,
	)

	require.Equal(t, string(data), "\"reply\"")
}

func TestEmptyLogKeyValue(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"empty_log_key_value":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, execErr)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "my value is empty", Value: ""},
						{Key: "", Value: "my key is empty"},
					},
				},
				execEvents,
			)
		})
	}
}

func TestExecNoLogs(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"no_logs":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			// require.Empty(t, execEvents)
		})
	}
}

func TestEmptyData(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"empty_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Empty(t, data)
		})
	}
}

func TestNoData(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"no_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Empty(t, data)
		})
	}
}

func TestUnicodeData(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestSecp256k1Verify(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/

			t.Run("CorrectCompactPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectLongPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectMsgHashCompactPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectMsgHashLongPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectSigCompactPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectSigLongPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectCompactPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"AoSdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXc","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectLongPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BISdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXcFWl11YCgu65hzvNDQE2Qo1hwTMQ/42Xif8O/MrxzvxI=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
		})
	}
}

func TestEd25519Verify(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectMsg", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmUK"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectSig", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
		})
	}
}

func TestEd25519BatchVerify(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("100Correct", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectMsg", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("IncorrectSig", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					events,
				)
			})
			t.Run("CorrectEmptySigsEmptyMsgsOnePubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":[],"msgs":[]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectEmpty", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":[]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectEmptyPubkeysEmptySigsOneMsg", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectMultisig", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
			t.Run("CorrectMultiMsgOneSigner", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg==","uuNxLEzAYDbuJ+BiYN94pTqhD7UhvCJNbxAbnWz0B9DivkPXmqIULko0DddP2/tVXPtjJ90J20faiWCEC3QkDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","cGVhY2Ugb3V0"]}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					events,
				)
			})
		})
	}
}

func TestSecp256k1RecoverPubkey(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// https://paulmillr.com/noble/
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":0,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//"},
					},
				},
				events,
			)

			_, _, _, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":1,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "Ams198xOCEVnc/ESvxF2nxnE3AVFO8ahB22S1ZgX2vSR"},
					},
				},
				events,
			)
		})
	}
}

func TestSecp256k1Sign(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// priv iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao=
			// pub ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, err)

			signature := events[0][1].Value

			_, _, _, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"secp256k1_verify":{"iterations":1,"pubkey":"ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS","sig":"%s","msg_hash":"K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA="}}`, signature), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "true"},
					},
				},
				events,
			)
		})
	}
}

func TestEd25519Sign(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// priv z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo=
			// pub jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, err)

			signature := events[0][1].Value

			_, _, _, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"ed25519_verify":{"iterations":1,"pubkey":"jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=","sig":"%s","msg":"d2VuIG1vb24="}}`, signature), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "true"},
					},
				},
				events,
			)
		})
	}
}

func TestBenchmarkEd25519BatchVerifyAPI(t *testing.T) {
	t.SkipNow()
	// Assaf: I wrote the benchmark like this because the init functions take testing.T
	// and not testing.B and I just wanted to quickly get a feel for the performance improvements
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			start := time.Now()
			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1000,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1, math.MaxUint64, 0)

			require.Empty(t, err)

			elapsed := time.Since(start)
			fmt.Printf("TestBenchmarkEd25519BatchVerifyAPI took %s\n", elapsed)
		})
	}
}

func TestSleep(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"sleep":{"ms":3000}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Error(t, execErr)
			require.Error(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseMemoryLimit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":13631488}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			// this should fail with memory error because 13MiB is more than the allowed 12MiB

			require.Empty(t, data)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseGasLimit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// ensure we get an out of gas panic
			defer func() {
				r := recover()
				require.NotNil(t, r)
				_, ok := r.(sdk.ErrorOutOfGas)
				require.True(t, ok, "%+v", r)
			}()

			_, _, _, _, _, _ = execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, testContract.IsCosmWasmV1, 100_000, 0)

			// this should fail with out of gas because 1GiB will ask for
			// 134,217,728 gas units (8192 per page times 16,384 pages)
			// the default gas limit in ctx is 200,000 which translates into
			// 20,000,000 WASM gas units, so before the memory_grow opcode is reached
			// the gas metering sees a request that'll cost 134mn and the limit
			// is 20mn, so it throws an out of gas exception

			require.True(t, false)
		})
	}
}

func TestAllocateOnHeapMoreThanSGXHasFailBecauseMemoryLimit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, testContract.IsCosmWasmV1, 9_000_000, 0)

			// this should fail with memory error because 1GiB is more
			// than the allowed 12MiB, gas is 9mn so WASM gas is 900mn
			// which is bigger than the 134mn from the previous test

			require.Empty(t, data)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestPassNullPointerToImports(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			tests := []string{
				"read_db_key",
				"write_db_key",
				"write_db_value",
				"remove_db_key",
				"canonicalize_address_input",
				"humanize_address_input",
			}

			for _, passType := range tests {
				t.Run(passType, func(t *testing.T) {
					_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"pass_null_pointer_to_imports_should_throw":{"pass_type":"%s"}}`, passType), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

					require.NotNil(t, execErr.GenericErr)
					if testContract.IsCosmWasmV1 {
						require.Contains(t, execErr.GenericErr.Msg, "execute contract failed")
					} else {
						require.Contains(t, execErr.GenericErr.Msg, "failed to read memory")
					}
				})
			}
		})
	}
}

func TestV1ReplyLoop(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"sub_msg_loop":{"iter": 10}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
	require.Equal(t, uint32(20), binary.BigEndian.Uint32(data))
}

func TestBankMsgSend(t *testing.T) {
	for _, contract := range testContracts {
		t.Run(contract.CosmWasmVersion, func(t *testing.T) {
			for _, callType := range []string{"init", "exec"} {
				t.Run(callType, func(t *testing.T) {
					for _, test := range []struct {
						description    string
						input          string
						isSuccuss      bool
						errorMsg       string
						balancesBefore string
						balancesAfter  string
					}{
						{
							description:    "regular",
							input:          `[{"amount":"2","denom":"denom"}]`,
							isSuccuss:      true,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5002denom",
						},
						{
							description:    "multi-coin",
							input:          `[{"amount":"1","denom":"assaf"},{"amount":"1","denom":"denom"}]`,
							isSuccuss:      true,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5001assaf,5001denom",
						},
						{
							description:    "zero",
							input:          `[{"amount":"0","denom":"denom"}]`,
							isSuccuss:      false,
							errorMsg:       "encrypted: dispatch: submessages: 0denom: invalid coins",
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5000denom",
						},
						{
							description:    "insufficient funds",
							input:          `[{"amount":"3","denom":"denom"}]`,
							isSuccuss:      false,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5000denom",
							errorMsg:       "encrypted: dispatch: submessages: 2denom is smaller than 3denom: insufficient funds",
						},
						{
							description:    "non-existing denom",
							input:          `[{"amount":"1","denom":"blabla"}]`,
							isSuccuss:      false,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5000denom",
							errorMsg:       "encrypted: dispatch: submessages: 0blabla is smaller than 1blabla: insufficient funds",
						},
						{
							description:    "none",
							input:          `[]`,
							isSuccuss:      true,
							balancesBefore: "5000assaf,200000denom 5000assaf,5000denom",
							balancesAfter:  "4998assaf,199998denom 5000assaf,5000denom",
						},
					} {
						t.Run(test.description, func(t *testing.T) {
							ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, contract.WasmFilePath, sdk.NewCoins(sdk.NewInt64Coin("assaf", 5000)))

							walletACoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)
							walletBCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletB)

							require.Equal(t, test.balancesBefore, walletACoinsBefore.String()+" "+walletBCoinsBefore.String())

							var err cosmwasm.StdError
							var contractAddress sdk.AccAddress

							if callType == "init" {
								_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","amount":%s}}`, walletB.String(), test.input), false, contract.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins(sdk.NewInt64Coin("denom", 2), sdk.NewInt64Coin("assaf", 2)))
							} else {
								_, _, contractAddress, _, _ = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, contract.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins(sdk.NewInt64Coin("denom", 2), sdk.NewInt64Coin("assaf", 2)))

								_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","amount":%s}}`, walletB.String(), test.input), false, contract.IsCosmWasmV1, math.MaxUint64, 0)
							}

							if test.isSuccuss {
								require.Empty(t, err)
							} else {
								require.NotEmpty(t, err)
								require.Equal(t, err.Error(), test.errorMsg)
							}

							walletACoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)
							walletBCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletB)

							require.Equal(t, test.balancesAfter, walletACoinsAfter.String()+" "+walletBCoinsAfter.String())
						})
					}
				})
			}
		})
	}
}

func TestSendFunds(t *testing.T) {
	for _, callTypes := range multisetsFrom([]string{"init", "exec", "user"}, 2) {
		originType, destinationType := callTypes[0], callTypes[1]

		t.Run(originType+"->"+destinationType, func(t *testing.T) {
			alreadyTested := make(map[string]bool)
			alreadyTested["useruser->useruser"] = true // we are only testing contracts here
			for _, currentSubjects := range multisetsFrom(testContracts, 2) {
				originVersion, destinationVersion := currentSubjects[0], currentSubjects[1]

				// users don't have versions, so skip the repeating tests (user v1 = user v10)
				if originType == "user" {
					originVersion.CosmWasmVersion = "user"
				}
				if destinationType == "user" {
					destinationVersion.CosmWasmVersion = "user"
				}
				testTitle := originType + originVersion.CosmWasmVersion + "->" + destinationType + destinationVersion.CosmWasmVersion
				if _, value := alreadyTested[testTitle]; value {
					continue
				}
				alreadyTested[testTitle] = true

				t.Run(testTitle, func(t *testing.T) {
					for _, test := range []struct {
						description              string
						coinsToSend              string
						isSuccess                bool
						errorMsg                 string
						balancesBefore           string
						balancesAfter            string
						destinationBalancesAfter string
					}{
						{
							description:              "one, has all",
							coinsToSend:              `20one`,
							isSuccess:                true,
							balancesBefore:           "200000denom,5000one",
							balancesAfter:            "200000denom,4980one",
							destinationBalancesAfter: "20one",
						},
						{
							description:              "one, missing",
							coinsToSend:              `20one`,
							isSuccess:                false,
							errorMsg:                 "0one is smaller than 20one: insufficient funds",
							balancesBefore:           "5000another",
							balancesAfter:            "5000another",
							destinationBalancesAfter: "",
						},
						{
							description:              "one, not enough",
							coinsToSend:              `20one`,
							isSuccess:                false,
							errorMsg:                 "19one is smaller than 20one: insufficient funds",
							balancesBefore:           "5000another,19one",
							balancesAfter:            "5000another,19one",
							destinationBalancesAfter: "",
						},
						{
							description:              "zero",
							coinsToSend:              ``,
							isSuccess:                true,
							balancesBefore:           "5000assaf,200000denom",
							balancesAfter:            "5000assaf,200000denom",
							destinationBalancesAfter: "",
						},
						{
							description:              "multi-coin, has all",
							coinsToSend:              `130assaf,15denom`,
							isSuccess:                true,
							balancesBefore:           "5000assaf,200000denom",
							balancesAfter:            "4870assaf,199985denom",
							destinationBalancesAfter: "130assaf,15denom",
						},
						{
							description:              "multi-coin, missing one",
							coinsToSend:              `130assaf,15denom`,
							isSuccess:                false,
							errorMsg:                 "0assaf is smaller than 130assaf: insufficient funds",
							balancesBefore:           "200000denom",
							balancesAfter:            "200000denom",
							destinationBalancesAfter: "",
						},
						{
							description:              "multi-coin, not enough of one of them",
							coinsToSend:              `130assaf,15denom`,
							isSuccess:                false,
							errorMsg:                 "10denom is smaller than 15denom: insufficient funds",
							balancesBefore:           "5000assaf,10denom",
							balancesAfter:            "5000assaf,10denom",
							destinationBalancesAfter: "",
						},
						{
							description:              "multi-coin, not enough of all of them",
							coinsToSend:              `130assaf,15denom`,
							isSuccess:                false,
							errorMsg:                 "12assaf is smaller than 130assaf: insufficient funds",
							balancesBefore:           "12assaf,10denom",
							balancesAfter:            "12assaf,10denom",
							destinationBalancesAfter: "",
						},
					} {
						t.Run(test.description, func(t *testing.T) {
							ctx, keeper, helperWallet, helperPrivKey, _, _ := setupBasicTest(t, sdk.NewCoins(sdk.NewInt64Coin("assaf", 5000)))

							fundingWallet, fundingWalletPrivKey := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, stringToCoins(test.balancesBefore))
							receivingWallet, _ := CreateFakeFundedAccount(ctx, keeper.accountKeeper, keeper.bankKeeper, sdk.NewCoins())

							// verify that the account was funded correctly
							fundingWalletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, fundingWallet)
							require.Equal(t, test.balancesBefore, fundingWalletCoinsBefore.String())

							var originCodeId uint64
							if originType != "user" {
								originCodeId, _ = uploadCode(ctx, t, keeper, originVersion.WasmFilePath, helperWallet)
							}

							var destinationCodeId uint64
							var destinationHash string
							var destinationAddr sdk.AccAddress

							// prepare receiving contract
							if destinationType != "user" {
								destinationCodeId, destinationHash = uploadCode(ctx, t, keeper, destinationVersion.WasmFilePath, helperWallet)

								if destinationType == "exec" {
									_, _, destinationAddr, _, _ = initHelperImpl(t, keeper, ctx, destinationCodeId, helperWallet, helperPrivKey, `{"nop":{}}`, false, destinationVersion.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins())
								}
							}

							var err cosmwasm.StdError
							var originAddress sdk.AccAddress
							var msg string

							inputCoins := CoinsToInput(stringToCoins(test.coinsToSend))
							if destinationType == "user" {
								msg = fmt.Sprintf(`{"bank_msg_send":{"to":"%s","amount":%s}}`, receivingWallet.String(), inputCoins)
								destinationAddr = receivingWallet
							} else if destinationType == "init" {
								msg = fmt.Sprintf(`{"send_multiple_funds_to_init_callback":{"code_id":%d,"coins":%s,"code_hash":"%s"}}`, destinationCodeId, inputCoins, destinationHash)
								// destination address will only be known after the contract is init
							} else {
								msg = fmt.Sprintf(`{"send_multiple_funds_to_exec_callback":{"to":"%s","coins":%s,"code_hash":"%s"}}`, destinationAddr, inputCoins, destinationHash)
							}

							var wasmEvents []ContractEvent
							if originType == "init" {
								_, _, originAddress, wasmEvents, err = initHelperImpl(t, keeper, ctx, originCodeId, fundingWallet, fundingWalletPrivKey, msg, false, originVersion.IsCosmWasmV1, defaultGasForTests, -1, stringToCoins(test.balancesBefore))
							} else if originType == "exec" {
								_, _, originAddress, _, _ = initHelper(t, keeper, ctx, originCodeId, helperWallet, helperPrivKey, `{"nop":{}}`, false, originVersion.IsCosmWasmV1, defaultGasForTests)

								_, _, _, wasmEvents, _, err = execHelperMultipleCoins(t, keeper, ctx, originAddress, fundingWallet, fundingWalletPrivKey, msg, false, originVersion.IsCosmWasmV1, math.MaxUint64, stringToCoins(test.balancesBefore), -1)
							} else {
								// user sends directly to contract
								originAddress = fundingWallet
								wasmCount := int64(-1)
								if !test.isSuccess {
									wasmCount = 0
								}
								if destinationType == "exec" {
									_, _, _, _, _, err = execHelperMultipleCoins(t, keeper, ctx, destinationAddr, fundingWallet, fundingWalletPrivKey, `{"no_data":{}}`, false, destinationVersion.IsCosmWasmV1, math.MaxUint64, stringToCoins(test.coinsToSend), wasmCount)
								} else {
									_, _, destinationAddr, _, err = initHelperImpl(t, keeper, ctx, destinationCodeId, fundingWallet, fundingWalletPrivKey, `{"nop":{}}`, false, destinationVersion.IsCosmWasmV1, math.MaxUint64, wasmCount, stringToCoins(test.coinsToSend))
								}
							}

							if !test.isSuccess {
								require.NotEmpty(t, err)

								expectedErrorMsg := test.errorMsg
								if originType != "user" {
									expectedErrorMsg = "dispatch: submessages: " + expectedErrorMsg
								}
								expectedErrorMsg = "encrypted: " + expectedErrorMsg
								require.Equal(t, expectedErrorMsg, err.Error())
							} else {
								require.Empty(t, err)

								originCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, originAddress)
								require.Equal(t, test.balancesAfter, originCoinsAfter.String())

								if destinationType == "init" && originType != "user" {
									var newContractBech32 string
									for _, v := range wasmEvents[1] {
										if v.Key == "contract_address" {
											newContractBech32 = v.Value
											break
										}
									}
									require.NotEmpty(t, newContractBech32)

									destinationAddr, _ = sdk.AccAddressFromBech32(newContractBech32)
								}

								destinationCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, destinationAddr)
								require.Equal(t, test.destinationBalancesAfter, destinationCoinsAfter.String())
							}
						})
					}
				})
			}
		})
	}
}

func TestWasmMsgStructure(t *testing.T) {
	for _, from := range testContracts {
		t.Run(fmt.Sprintf("from %s", from.CosmWasmVersion), func(t *testing.T) {
			for _, to := range testContracts {
				t.Run(fmt.Sprintf("to %s", to.CosmWasmVersion), func(t *testing.T) {
					for _, firstCallType := range []string{"init", "exec"} {
						t.Run(fmt.Sprintf("first call %s", firstCallType), func(t *testing.T) {
							for _, secondCallType := range []string{"init", "exec"} {
								t.Run(fmt.Sprintf("second call %s", secondCallType), func(t *testing.T) {
									for _, test := range []struct {
										description      string
										msg              string
										isSuccess        bool
										isErrorEncrypted bool
										expectedError    string
									}{
										{
											description:      "Send invalid input",
											msg:              `{\"blabla\":{}}`,
											isSuccess:        false,
											isErrorEncrypted: true,
											expectedError:    "unknown variant",
										},
										{
											description:      "Success",
											msg:              `{\"wasm_msg\":{\"ty\": \"success\"}}`,
											isSuccess:        true,
											isErrorEncrypted: true,
											expectedError:    "",
										},
										{
											description:      "StdError",
											msg:              `{\"wasm_msg\":{\"ty\": \"err\"}}`,
											isSuccess:        false,
											isErrorEncrypted: true,
											expectedError:    "custom error",
										},
										{
											description:      "Panic",
											msg:              `{\"wasm_msg\":{\"ty\": \"panic\"}}`,
											isSuccess:        false,
											isErrorEncrypted: false,
											expectedError:    "panicked",
										},
									} {
										t.Run(test.description, func(t *testing.T) {
											ctx, keeper, fromCodeID, _, walletA, privKeyA, _, _ := setupTest(t, from.WasmFilePath, sdk.NewCoins())

											wasmCode, err := os.ReadFile(to.WasmFilePath)
											require.NoError(t, err)

											toCodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
											codeInfo, err := keeper.GetCodeInfo(ctx, toCodeID)
											require.NoError(t, err)
											toCodeHash := hex.EncodeToString(codeInfo.CodeHash)
											require.NoError(t, err)

											toAddress := sdk.AccAddress{}
											if secondCallType != "init" {
												_, _, toAddress, _, err = initHelper(t, keeper, ctx, toCodeID, walletA, privKeyA, `{"nop":{}}`, true, to.IsCosmWasmV1, defaultGasForTests)
												require.Empty(t, err)
											}

											fromAddress := sdk.AccAddress{}
											if firstCallType == "init" {
												_, _, _, _, err = initHelper(t, keeper, ctx, fromCodeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_%s":{"code_id": %d, "addr": "%s", "code_hash": "%s", "label": "%s", "msg": "%s"}}`, secondCallType, toCodeID, toAddress, toCodeHash, "blabla", test.msg), test.isErrorEncrypted, true, defaultGasForTests)
											} else if firstCallType == "exec" {
												_, _, fromAddress, _, err = initHelper(t, keeper, ctx, fromCodeID, walletA, privKeyA, `{"nop":{}}`, true, from.IsCosmWasmV1, defaultGasForTests)
												require.Empty(t, err)
												_, _, _, _, _, err = execHelper(t, keeper, ctx, fromAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_%s":{"code_id": %d, "addr": "%s", "code_hash": "%s", "label": "%s", "msg": "%s"}}`, secondCallType, toCodeID, toAddress, toCodeHash, "blabla", test.msg), test.isErrorEncrypted, true, math.MaxUint64, 0)
											}

											if test.isSuccess {
												require.Empty(t, err)
											} else {
												require.NotEmpty(t, err)
												require.Contains(t, fmt.Sprintf("%+v", err), test.expectedError)
											}
										})
									}
								})
							}
						})
					}
				})
			}
		})
	}
}

func TestCosmosMsgCustom(t *testing.T) {
	for _, contract := range testContracts {
		t.Run(contract.CosmWasmVersion, func(t *testing.T) {
			for _, callType := range []string{"init", "exec"} {
				t.Run(callType, func(t *testing.T) {
					ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, contract.WasmFilePath, sdk.NewCoins())

					var err cosmwasm.StdError
					var contractAddress sdk.AccAddress

					if callType == "init" {
						_, _, contractAddress, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"cosmos_msg_custom":{}}`), false, contract.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins())
					} else {
						_, _, contractAddress, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, contract.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins())

						_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"cosmos_msg_custom":{}}`), false, contract.IsCosmWasmV1, math.MaxUint64, 0)
					}

					require.NotEmpty(t, err)
					// if contract.IsCosmWasmV1 {
					require.Contains(t, err.Error(), "Custom variant not supported: invalid CosmosMsg from the contract")
					// } else {
					// 	require.Equal(t, err.Error(), "Custom variant not supported: invalid CosmosMsg from the contract")

					// }
				})
			}
		})
	}
}

func TestV1SendsFundsWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, true, defaultGasForTests, 200)
	require.Empty(t, err)

	_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"send_funds_with_reply":{}}`, true, true, math.MaxUint64, 0)

	require.Empty(t, err)
}

func TestV1SendsFundsWithErrorWithReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)

	_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"send_funds_with_error_with_reply":{}}`, false, true, math.MaxUint64, 0)

	require.NotEmpty(t, err)
	require.Contains(t, fmt.Sprintf("%+v", err), "an sdk error occoured while sending a sub-message")
}

func TestCallbackSanity(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			requireEvents(t,
				initEvents,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "init", Value: "🌈"},
					},
				},
			)

			_, _, data, execEvents, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":2,"y":3}}`, contractAddress.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "banana", Value: "🍌"},
					},
					{
						{Key: "kiwi", Value: "🥝"},
						{Key: "contract_address", Value: contractAddress.String()},
					},
					{
						{Key: "watermelon", Value: "🍉"},
						{Key: "contract_address", Value: contractAddress.String()},
					},
				},
				execEvents,
			)

			require.Equal(t, []byte{2, 3}, data)
		})
	}
}

func TestCodeHashExecCallExec(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.Empty(t, err)

				var newContractBech32 string
				for _, v := range events[1] {
					if v.Key == "contract_address" {
						newContractBech32 = v.Value
						break
					}
				}
				require.NotEmpty(t, newContractBech32)

				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: addr.String()},
							{Key: "b", Value: "b"},
						},
						{
							{Key: "contract_address", Value: newContractBech32},
							{Key: "watermelon", Value: "🍉"},
						},
					},
					events,
				)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr, codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				if testContract.IsCosmWasmV1 {
					require.Contains(t,
						err.Error(),
						"v1_sanity_contract::msg::ExecuteMsg: Expected to parse either a `true`, `false`, or a `null`.",
					)
				} else {
					require.Contains(t,
						err.Error(),
						"parsing test_contract::contract::HandleMsg: Expected to parse either a `true`, `false`, or a `null`.",
					)
				}
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, codeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestGasIsChargedForExecCallbackToExec(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// exec callback to exec
			_, _, _, _, _, err := execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":1,"y":2}}`, addr, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 3)
			require.Empty(t, err)
		})
	}
}

func TestMsgSenderInCallback(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"callback_to_log_msg_sender":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			requireEvents(t, []ContractEvent{
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "hi", Value: "hey"},
				},
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "msg.sender", Value: addr.String()},
				},
			}, events)
		})
	}
}

func TestDepositToContract(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())

			require.Equal(t, `[{"denom":"denom","amount":"17"}]`, string(data))
		})
	}
}

func TestContractSendFunds(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsBefore.String())
			require.Equal(t, "199983denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, addr.String(), walletA.String(), "denom", 17), false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "200000denom", walletCoinsAfter.String())

			require.Empty(t, execErr)
		})
	}
}

func TestContractSendFundsToExecCallback(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 17, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "17denom", contract2CoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
		})
	}
}

func TestContractSendFundsToExecCallbackNotEnough(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 19, codeHash), false, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "insufficient funds")

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			// The state here should have been reverted by the APP but in go-tests we create our own keeper
			// so it is not reverted in this case.
			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "", contract2CoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
		})
	}
}

func TestExecCallbackContractError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s","code_hash":"%s"}}`, contractAddress, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "la la 🤯")
			// require.Empty(t, execEvents)
			require.Empty(t, data)
		})
	}
}

func TestExecCallbackBadParam(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, execErr.GenericErr)
				require.Contains(t, execErr.GenericErr.Msg, "v1_sanity_contract::msg::ExecuteMsg")
				require.Contains(t, execErr.GenericErr.Msg, "unknown variant `callback_contract_bad_param`")
			} else {
				require.NotNil(t, execErr.ParseErr)
				require.Equal(t, "test_contract::contract::HandleMsg", execErr.ParseErr.Target)
				require.Contains(t, execErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
			}
			// require.Empty(t, execEvents)
			require.Empty(t, data)
		})
	}
}

func TestCallbackExecuteParamError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			msg := fmt.Sprintf(`{"a":{"code_hash":"%s","contract_addr":"notanaddress","x":2,"y":3}}`, codeHash)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Contains(t, err.Error(), "invalid address")
		})
	}
}

func TestExecuteIllegalInputError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `bad input`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, execErr.GenericErr)
				require.Contains(t, execErr.GenericErr.Msg, "Error parsing")
			} else {
				require.NotNil(t, execErr.ParseErr)
			}
		})
	}
}

func TestExecContractError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			t.Run("generic_err", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"generic_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				require.NotNil(t, err.GenericErr)
				require.Contains(t, err.GenericErr.Msg, "la la 🤯")
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ra ra 🤯")
				} else {
					require.NotNil(t, err.InvalidBase64)
					require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
				}
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ka ka 🤯")
				} else {
					require.NotNil(t, err.InvalidUtf8)
					require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
				}
			})
			t.Run("not_found", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"not_found"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "za za 🤯")
				} else {
					require.NotNil(t, err.NotFound)
					require.Equal(t, "za za 🤯", err.NotFound.Kind)
				}
			})
			t.Run("parse_err", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"parse_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "na na 🤯")
					require.Contains(t, err.GenericErr.Msg, "pa pa 🤯")
				} else {
					require.NotNil(t, err.ParseErr)
					require.Equal(t, "na na 🤯", err.ParseErr.Target)
					require.Equal(t, "pa pa 🤯", err.ParseErr.Msg)
				}
			})
			t.Run("serialize_err", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"serialize_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ba ba 🤯")
					require.Contains(t, err.GenericErr.Msg, "ga ga 🤯")
				} else {
					require.NotNil(t, err.SerializeErr)
					require.Equal(t, "ba ba 🤯", err.SerializeErr.Source)
					require.Equal(t, "ga ga 🤯", err.SerializeErr.Msg)
				}
			})
			t.Run("unauthorized", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"unauthorized"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Unauthorized)
				}
			})
			t.Run("underflow", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"underflow"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1 {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Underflow)
					require.Equal(t, "minuend 🤯", err.Underflow.Minuend)
					require.Equal(t, "subtrahend 🤯", err.Underflow.Subtrahend)
				}
			})
		})
	}
}

func TestExecPanic(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"panic":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestCanonicalizeAddressErrors(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			// this function should handle errors internally and return gracefully
			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"test_canonicalize_address_errors":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🤟", string(data))
		})
	}
}

func TestV1ReplyChainAllSuccess(t *testing.T) {
	amountOfContracts := uint64(5)
	ctx, keeper, codeIds, codeHashes, walletA, privKeyA, _, _ := setupChainTest(t, TestContractPaths[v1Contract], sdk.NewCoins(), amountOfContracts)
	contractAddresses := make([]sdk.AccAddress, amountOfContracts)

	for i := uint64(0); i < amountOfContracts; i++ {
		_, _, contractAddresses[i], _, _ = initHelper(t, keeper, ctx, codeIds[i], walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	}

	executeDetails := make([]ExecuteDetails, amountOfContracts-1)
	for i := uint64(1); i < amountOfContracts; i++ {
		executeDetails[i-1] = ExecuteDetails{
			ContractAddress: contractAddresses[i].String(),
			ContractHash:    codeHashes[i],
			ShouldError:     false,
			MsgId:           9000,
			Data:            fmt.Sprintf("%d", i),
		}
	}

	marshaledDetails, err := json.Marshal(executeDetails)
	require.Empty(t, err)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddresses[0], walletA, privKeyA, fmt.Sprintf(`{"execute_multiple_contracts":{"details": %s}}`, string(marshaledDetails)), true, true, math.MaxUint64, 0)
	require.Empty(t, err)

	expectedFlow := ""
	for i := uint64(amountOfContracts - 1); i > 0; i-- {
		expectedFlow += contractAddresses[i].String() + " -> "
	}

	expectedFlow += contractAddresses[0].String()

	require.Equal(t, expectedFlow, string(data))
}

func TestV1ReplyChainPartiallyReplied(t *testing.T) {
	amountOfContracts := uint64(10)
	amountOfContractToBeReplied := uint64(5)

	ctx, keeper, codeIds, codeHashes, walletA, privKeyA, _, _ := setupChainTest(t, TestContractPaths[v1Contract], sdk.NewCoins(), amountOfContracts)
	contractAddresses := make([]sdk.AccAddress, amountOfContracts)

	for i := uint64(0); i < amountOfContracts; i++ {
		_, _, contractAddresses[i], _, _ = initHelper(t, keeper, ctx, codeIds[i], walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	}

	executeDetails := make([]ExecuteDetails, amountOfContracts-1)
	for i := uint64(1); i < amountOfContracts; i++ {
		msgId := uint64(9000)
		if i >= amountOfContractToBeReplied {
			msgId = 0
		}

		executeDetails[i-1] = ExecuteDetails{
			ContractAddress: contractAddresses[i].String(),
			ContractHash:    codeHashes[i],
			ShouldError:     false,
			MsgId:           msgId,
			Data:            fmt.Sprintf("%d", i),
		}
	}

	marshaledDetails, err := json.Marshal(executeDetails)
	require.Empty(t, err)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddresses[0], walletA, privKeyA, fmt.Sprintf(`{"execute_multiple_contracts":{"details": %s}}`, string(marshaledDetails)), true, true, math.MaxUint64, 0)
	require.Empty(t, err)

	expectedFlow := ""

	expectedFlow += fmt.Sprintf("%d", amountOfContractToBeReplied) + " -> "

	for i := uint64(amountOfContractToBeReplied - 2); i > 0; i-- {
		expectedFlow += contractAddresses[i].String() + " -> "
	}

	expectedFlow += contractAddresses[0].String()

	require.Equal(t, expectedFlow, string(data))
}

func TestV1ReplyChainWithError(t *testing.T) {
	amountOfContracts := uint64(5)
	ctx, keeper, codeIds, codeHashes, walletA, privKeyA, _, _ := setupChainTest(t, TestContractPaths[v1Contract], sdk.NewCoins(), amountOfContracts)
	contractAddresses := make([]sdk.AccAddress, amountOfContracts)

	for i := uint64(0); i < amountOfContracts; i++ {
		_, _, contractAddresses[i], _, _ = initHelper(t, keeper, ctx, codeIds[i], walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	}

	executeDetails := make([]ExecuteDetails, amountOfContracts-1)
	for i := uint64(1); i < amountOfContracts; i++ {
		executeDetails[i-1] = ExecuteDetails{
			ContractAddress: contractAddresses[i].String(),
			ContractHash:    codeHashes[i],
			ShouldError:     false,
			MsgId:           9000,
			Data:            fmt.Sprintf("%d", i),
		}
	}

	executeDetails[amountOfContracts-2].ShouldError = true

	marshaledDetails, err := json.Marshal(executeDetails)
	require.Empty(t, err)

	_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddresses[0], walletA, privKeyA, fmt.Sprintf(`{"execute_multiple_contracts":{"details": %s}}`, string(marshaledDetails)), true, true, math.MaxUint64, 0)
	require.Empty(t, err)

	expectedFlow := "err -> "
	for i := uint64(amountOfContracts - 4); i > 0; i-- {
		expectedFlow += contractAddresses[i].String() + " -> "
	}

	expectedFlow += contractAddresses[0].String()

	require.Equal(t, expectedFlow, string(data))
}

func TestEvaporateGas(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[evaporateContract], sdk.NewCoins())
	_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, _, _, baseGasUsed, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"evaporate":{"amount":0}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	for _, test := range []struct {
		description   string
		msg           string
		outOfGas      bool
		gasExpected   uint64
		expectedError string
		gasForTest    uint64
	}{
		{
			description: "Evaporate 1 gas",
			msg:         `{"evaporate":{"amount": 2}}`,
			outOfGas:    false,
			// less than the minimum of 8 should just return the base amount - though it turns out that
			// for some reason calling evaporate with a value > 0 costs 1 more gas
			gasExpected: 0,
			gasForTest:  defaultGasForTests,
		},
		{
			description: "Evaporate 9 gas",
			msg:         `{"evaporate":{"amount": 9}}`,
			outOfGas:    false,
			// 9 - (base = 8)
			gasExpected: 1,
			gasForTest:  defaultGasForTests,
		},
		{
			description: "Evaporate 1200 gas",
			msg:         `{"evaporate":{"amount": 1200}}`,
			outOfGas:    false,
			// 1200 - (base = 8) + 1 (see above) = 1193
			gasExpected: 1192,
			gasForTest:  defaultGasForTests,
		},
		{
			description: "Evaporate 400000 gas",
			msg:         `{"evaporate":{"amount": 400000}}`,
			outOfGas:    false,
			// evaporate accepts sdk gas, which makes sense. However, I think this causes the amount
			// to be offset by 1 sometimes - do we really care?
			gasExpected: 399993,
			gasForTest:  defaultGasForTests,
		},
		{
			description: "Evaporate 500000 gas",
			msg:         `{"evaporate":{"amount": 500000}}`,
			outOfGas:    true,
			gasExpected: 0,
			gasForTest:  defaultGasForTests,
		},
	} {
		t.Run(test.description, func(t *testing.T) {
			if test.outOfGas {
				defer func() {
					r := recover()
					require.NotNil(t, r)
					_, ok := r.(sdk.ErrorOutOfGas)
					require.True(t, ok, "%+v", r)
				}()
			}

			_, _, _, _, actualUsedGas, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, test.msg, true, true, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t, baseGasUsed+test.gasExpected, actualUsedGas)
		})
	}
}

func TestCheckGas(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[evaporateContract], sdk.NewCoins())
	_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, initErr)

	// 995 is the sum of all the overhead that goes into a contract call beyond the base cost (reading keys, calculations, etc)
	baseContractUsage := types.InstanceCost + 995

	_, _, _, events, baseGasUsed, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"check_gas":{}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, err)

	execEvent := events[0]

	gasUsed, err2 := strconv.ParseUint(execEvent[1].Value, 10, 64)
	require.Empty(t, err2)

	require.Equal(t, gasUsed+1, baseGasUsed-baseContractUsage)
}

func TestConsumeExact(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[evaporateContract], sdk.NewCoins())
	_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, initErr)

	// not sure where the 16 extra gas comes vs the previous check_gas test, but it makes everything play nice, so....
	baseContractUsage := types.InstanceCost + 995 - 16

	for _, test := range []struct {
		description   string
		msg           string
		outOfGas      bool
		gasExpected   uint64
		expectedError string
		gasForTest    uint64
	}{
		{
			description: "Use exactly 1000 gas",
			msg:         `{"use_exact":{"amount": 1000}}`,
			outOfGas:    false,
			// less than the minimum of 8 should just return the base amount - though it turns out that
			// for some reason calling evaporate with a value > 0 costs 1 more gas
			gasExpected: 1000,
			gasForTest:  defaultGasForTests,
		},
		{
			description: "Use exactly 400000",
			// evaporate and check_gas use rounding to convert sdk to cosmwasm gas - sometimes this causes the amount
			// to be offset by 1 - do we really care?
			msg:         `{"use_exact":{"amount": 399999}}`,
			outOfGas:    false,
			gasExpected: 400000,
			gasForTest:  defaultGasForTests,
		},
	} {
		t.Run(test.description, func(t *testing.T) {
			if test.outOfGas {
				defer func() {
					r := recover()
					require.NotNil(t, r)
					_, ok := r.(sdk.ErrorOutOfGas)
					require.True(t, ok, "%+v", r)
				}()
			}

			_, _, _, _, actualUsedGas, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, test.msg, true, true, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t, baseContractUsage+test.gasExpected, actualUsedGas)
		})
	}
}

func TestLastMsgMarkerMultipleMsgsInATx(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop": {}}`, true, true, defaultGasForTests)

	msgs := []string{`{"last_msg_marker_nop":{}}`, `{"last_msg_marker_nop":{}}`}

	results, err := execHelperMultipleMsgs(t, keeper, ctx, contractAddress, walletA, privKeyA, msgs, true, true, math.MaxUint64, 0)
	require.NotEqual(t, nil, err)
	require.Equal(t, 1, len(results))
}

func TestIBCHooksIncomingTransfer(t *testing.T) {
	for _, test := range []struct {
		name string
		// remoteDenom: "port_on_other_chain/channel_on_other_chain/base_denom" (e.g. transfer/channel-0/uscrt) or base_denom (e.g. uatom)
		remoteDenom string
		// localDenom: denom on Secret ("denom" or "ibc/...")
		localDenom string
	}{
		{
			name:        "denom originated from Secret",
			remoteDenom: "transfer/channel-0/denom",
			localDenom:  "denom",
		},
		{
			name:        "denom is base denom of the other chain",
			remoteDenom: "uatom",
			localDenom:  "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2",
		},
	} {
		for _, testContract := range testContracts {
			t.Run(test.name, func(t *testing.T) {
				t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
					ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins(sdk.NewInt64Coin(test.localDenom, 1)))

					_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
					require.Empty(t, initErr)

					data := ibctransfertypes.FungibleTokenPacketData{
						Denom:    test.remoteDenom,
						Amount:   "1",
						Sender:   "ignored",
						Receiver: contractAddress.String(), // must be the contract address, like in the memo
						Memo:     fmt.Sprintf(`{"wasm":{"contract":"%s","msg":{"log_msg_sender":{}}}}`, contractAddress.String()),
					}
					dataBytes, err := json.Marshal(data)
					require.NoError(t, err)

					sdkMsg := ibcchanneltypes.MsgRecvPacket{
						Packet: ibcchanneltypes.Packet{
							Sequence:           0,
							SourcePort:         "transfer",  // port on the other chain
							SourceChannel:      "channel-0", // channel on the other chain
							DestinationPort:    "transfer",  // port on Secret
							DestinationChannel: "channel-0", // channel on Secret
							Data:               dataBytes,
							TimeoutHeight:      ibcclienttypes.Height{},
							TimeoutTimestamp:   0,
						},
						ProofCommitment: []byte{},
						ProofHeight:     ibcclienttypes.Height{},
						Signer:          walletA.String(),
					}

					ctx = PrepareSignedTx(t, keeper, ctx, walletA, privKeyA, &sdkMsg)

					_, execErr := keeper.Execute(ctx, contractAddress, walletA, []byte(`{"log_msg_sender":{}}`), sdk.NewCoins(sdk.NewInt64Coin(test.localDenom, 1)), nil, cosmwasm.HandleTypeIbcWasmHooksIncomingTransfer)

					require.Empty(t, execErr)

					events := tryDecryptWasmEvents(ctx, nil)

					requireEvents(t,
						[]ContractEvent{
							{
								{Key: "contract_address", Value: contractAddress.String()},
								{
									Key:   "msg.sender",
									Value: "",
								},
							},
						},
						events,
					)
				})
			})
		}
	}
}

func TestIBCHooksOutgoingTransferAck(t *testing.T) {
	for _, test := range []struct {
		name                string
		sdkMsgSrcPort       string
		sdkMsgSrcChannel    string
		sdkMsgDestPort      string
		sdkMsgDestChannel   string
		sdkMsgAck           string
		wasmInputSrcChannel string
		wasmInputAck        string
		wasmInputCoin       sdk.Coins
		err                 string
	}{
		{
			name:                "happy path",
			sdkMsgSrcPort:       "transfer",
			sdkMsgSrcChannel:    "channel-0",
			sdkMsgDestPort:      "transfer",
			sdkMsgDestChannel:   "channel-1",
			sdkMsgAck:           "blabla",
			wasmInputSrcChannel: "channel-0",
			wasmInputAck:        "blabla",
			wasmInputCoin:       sdk.NewCoins(),
			err:                 "",
		},
		{
			name:                "channel mismatch",
			sdkMsgSrcPort:       "transfer",
			sdkMsgSrcChannel:    "channel-0",
			sdkMsgDestPort:      "transfer",
			sdkMsgDestChannel:   "channel-1",
			sdkMsgAck:           "blabla",
			wasmInputSrcChannel: "channel-1",
			wasmInputAck:        "blabla",
			wasmInputCoin:       sdk.NewCoins(),
			err:                 "failed to verify transaction",
		},
		{
			name:                "no coins should be sent",
			sdkMsgSrcPort:       "transfer",
			sdkMsgSrcChannel:    "channel-0",
			sdkMsgDestPort:      "transfer",
			sdkMsgDestChannel:   "channel-1",
			sdkMsgAck:           "blabla",
			wasmInputSrcChannel: "channel-0",
			wasmInputAck:        "blabla",
			wasmInputCoin:       sdk.NewCoins(sdk.NewInt64Coin("denom", 1)),
			err:                 "failed to verify transaction",
		},
		{
			name:                "ack mismatch",
			sdkMsgSrcPort:       "transfer",
			sdkMsgSrcChannel:    "channel-0",
			sdkMsgDestPort:      "transfer",
			sdkMsgDestChannel:   "channel-1",
			sdkMsgAck:           "yadayada",
			wasmInputSrcChannel: "channel-0",
			wasmInputAck:        "blabla",
			wasmInputCoin:       sdk.NewCoins(),
			err:                 "failed to verify transaction",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
			require.Empty(t, initErr)

			data := ibctransfertypes.FungibleTokenPacketData{
				Denom:    "ignored",
				Amount:   "1",
				Sender:   contractAddress.String(), // must be the contract address, like in the memo
				Receiver: "ignored",
				Memo:     fmt.Sprintf(`{"ibc_callback":"%s"}`, contractAddress.String()),
			}
			dataBytes, err := json.Marshal(data)
			require.NoError(t, err)

			sdkMsg := ibcchanneltypes.MsgAcknowledgement{
				Packet: ibcchanneltypes.Packet{
					Sequence:           0,
					SourcePort:         test.sdkMsgSrcPort,     // port on Secret
					SourceChannel:      test.sdkMsgSrcChannel,  // channel on Secret
					DestinationPort:    test.sdkMsgDestPort,    // port on the other chain
					DestinationChannel: test.sdkMsgDestChannel, // channel on the other chain
					Data:               dataBytes,
					TimeoutHeight:      ibcclienttypes.Height{},
					TimeoutTimestamp:   0,
				},
				Acknowledgement: []byte(test.sdkMsgAck),
				ProofAcked:      []byte{},
				ProofHeight:     ibcclienttypes.Height{},
				Signer:          walletA.String(),
			}

			ctx = PrepareSignedTx(t, keeper, ctx, walletA, privKeyA, &sdkMsg)

			_, execErr := keeper.Execute(ctx,
				contractAddress,
				walletA,
				[]byte(
					fmt.Sprintf(`{"ibc_lifecycle_complete":{"ibc_ack":{"channel":"%s","sequence":0,"ack":"%s","success":true}}}`,
						test.wasmInputSrcChannel,
						test.wasmInputAck,
					)),
				test.wasmInputCoin,
				nil,
				cosmwasm.HandleTypeIbcWasmHooksOutgoingTransferAck,
			)

			if test.err == "" {
				require.Empty(t, execErr)
				events := tryDecryptWasmEvents(ctx, nil)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "ibc_lifecycle_complete.ibc_ack.channel", Value: test.sdkMsgSrcChannel},
							{Key: "ibc_lifecycle_complete.ibc_ack.sequence", Value: "0"},
							{Key: "ibc_lifecycle_complete.ibc_ack.ack", Value: test.sdkMsgAck},
							{Key: "ibc_lifecycle_complete.ibc_ack.success", Value: "true"},
						},
					},
					events,
				)
			} else {
				require.Contains(t, execErr.Error(), test.err)
			}
		})
	}
}
