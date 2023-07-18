package keeper

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	v010types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"

	"golang.org/x/exp/slices"

	cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	"github.com/stretchr/testify/require"

	ibctransfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	ibcchanneltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
)

type MigrateTestContract struct {
	CosmWasmVersionBefore string
	CosmWasmVersionAfter  string
	IsCosmWasmV1Before    bool
	IsCosmWasmV1After     bool
	WasmFilePathBefore    string
	WasmFilePathAfter     string
}

var migrateTestContracts = []MigrateTestContract{
	{
		CosmWasmVersionBefore: "v0.10",
		CosmWasmVersionAfter:  "v0.10",
		IsCosmWasmV1Before:    false,
		IsCosmWasmV1After:     false,
		WasmFilePathBefore:    TestContractPaths[v010Contract],
		WasmFilePathAfter:     TestContractPaths[v010MigratedContract],
	},
	{
		CosmWasmVersionBefore: "v1",
		CosmWasmVersionAfter:  "v1",
		IsCosmWasmV1Before:    true,
		IsCosmWasmV1After:     true,
		WasmFilePathBefore:    TestContractPaths[v1Contract],
		WasmFilePathAfter:     TestContractPaths[v1MigratedContract],
	},
	{
		CosmWasmVersionBefore: "v0.10",
		CosmWasmVersionAfter:  "v1",
		IsCosmWasmV1Before:    false,
		IsCosmWasmV1After:     true,
		WasmFilePathBefore:    TestContractPaths[v010Contract],
		WasmFilePathAfter:     TestContractPaths[v1MigratedContract],
	},
	{
		CosmWasmVersionBefore: "v1",
		CosmWasmVersionAfter:  "v0.10",
		IsCosmWasmV1Before:    true,
		IsCosmWasmV1After:     false,
		WasmFilePathBefore:    TestContractPaths[v1Contract],
		WasmFilePathAfter:     TestContractPaths[v010MigratedContract],
	},
}

func TestContractAfterMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[migrateContractV1], sdk.NewCoins())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[migrateContractV2], walletA)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)

	_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"test":{}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, data)

	_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"migrate":{}}`, true, true, math.MaxUint64)
	require.Empty(t, err)

	_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"new_function":{}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, data)
}

func TestContractInfoAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[migrateContractV1], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)

	info := keeper.GetContractInfo(ctx, contractAddress)
	require.Equal(t, walletA.String(), info.Admin)
}

func TestContractInfoNullAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[migrateContractV1], sdk.NewCoins())

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)

	info := keeper.GetContractInfo(ctx, contractAddress)
	require.Equal(t, sdk.AccAddress{}.String(), info.Admin)
}

func TestWithStorageAfterMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[migrateContractV1], sdk.NewCoins())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[migrateContractV2], walletA)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)

	_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"test":{}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, data)

	_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"migrate":{}}`, true, true, math.MaxUint64)
	require.Empty(t, err)

	_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"new_function_with_storage":{}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, data)
}

func TestMigrateContractFromNonAdminAccount(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, WalletB, privKeyB := setupTest(t, TestContractPaths[migrateContractV1], sdk.NewCoins())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[migrateContractV2], walletA)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)

	_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, WalletB, privKeyB, `{"migrate":{}}`, false, true, math.MaxUint64, 0)
	require.Contains(t, err.Error(), "requires migrate from admin: migrate contract failed")
}

func TestVmErrorDuringMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[migrateContractV1], sdk.NewCoins())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[migrateContractV2], walletA)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)

	_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"yolo":{}}`, true, true, math.MaxUint64)
	require.Contains(t, err.Error(), "Error parsing into type migrate_contract_v2::msg::MigrateMsg: unknown variant `yolo`")
}

func TestStdErrorAfterMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[migrateContractV1], sdk.NewCoins())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[migrateContractV2], walletA)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)

	_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"test":{}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, data)

	_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"std_error":{}}`, true, true, math.MaxUint64)
	require.Equal(t, err.Error(), "encrypted: Generic error: this is an std error")
}

/// copy of exec tests but doing it after a migration:

func TestStateAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(data))

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, err)

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(data))

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"remove_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Empty(t, data)

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(data))
		})
	}
}

func TestAddrValidateFunctionAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}

		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"validate_address":{"addr":"%s"}}`, contractAddress), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			resp, aErr := sdk.AccAddressFromBech32(string(data))
			require.Empty(t, aErr)

			require.Equal(t, resp, contractAddress)

			_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"validate_address":{"addr":"secret18vd8fpwxzck93qlwghaj6arh4p7c5nyf7hmag8"}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Equal(t, string(data), "\"Apple\"")
		})
	}
}

func TestEnvAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_env":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 1)
			require.Empty(t, execErr)

			execEvent := execEvents[0]
			envAttributeIndex := slices.IndexFunc(execEvent, func(c v010types.LogAttribute) bool { return c.Key == "env" })
			envAttribute := execEvent[envAttributeIndex]

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
				calcCodeHash(TestContractPaths[v1MigratedContract]),
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
		})
	}
}

func TestNestedAttributeAfterMigrate(t *testing.T) {
	// For more reference: https://github.com/scrtlabs/SecretNetwork/issues/1235
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attribute_step1":{}}`, true, testContract.IsCosmWasmV1After, 10*defaultGasForTests, 0)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr2", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr4", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr_reply", Value: "🦄"},
					},
				},
				events,
			)

			require.Equal(t, string(data), "\"reply\"")
		})
	}
}

func TestEmptyLogKeyValueAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"empty_log_key_value":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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

func TestExecNoLogsAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"no_logs":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.Empty(t, err)
		})
	}
}

func TestEmptyDataAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"empty_data":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Empty(t, data)
		})
	}
}

func TestNoDataAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"no_data":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Empty(t, data)
		})
	}
}

func TestUnicodeDataAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.Empty(t, err)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestSecp256k1VerifyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			// https://paulmillr.com/noble/

			t.Run("CorrectCompactPubkey", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"AoSdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXc","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BISdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXcFWl11YCgu65hzvNDQE2Qo1hwTMQ/42Xif8O/MrxzvxI=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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

func TestEd25519VerifyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmUK"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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

func TestEd25519BatchVerifyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":[],"msgs":[]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":[]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg==","uuNxLEzAYDbuJ+BiYN94pTqhD7UhvCJNbxAbnWz0B9DivkPXmqIULko0DddP2/tVXPtjJ90J20faiWCEC3QkDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","cGVhY2Ugb3V0"]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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

func TestSecp256k1RecoverPubkeyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			// https://paulmillr.com/noble/
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":0,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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

			_, _, _, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":1,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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

func TestSecp256k1SignAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			// priv iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao=
			// pub ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			signature := events[0][1].Value

			_, _, _, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"secp256k1_verify":{"iterations":1,"pubkey":"ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS","sig":"%s","msg_hash":"K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA="}}`, signature), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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

func TestEd25519SignAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			// priv z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo=
			// pub jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"ed25519_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			signature := events[0][1].Value

			_, _, _, events, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"ed25519_verify":{"iterations":1,"pubkey":"jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=","sig":"%s","msg":"d2VuIG1vb24="}}`, signature), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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

func TestSleepAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"sleep":{"ms":3000}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.Error(t, execErr)
			require.Error(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseMemoryLimitAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"allocate_on_heap":{"bytes":12582913}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			// this should fail with memory error because 12MiB+1 is more than the allowed 12MiB

			require.Empty(t, data)

			require.NotNil(t, execErr)
			require.Contains(t, execErr.Error(), "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseGasLimitAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			// ensure we get an out of gas panic
			defer func() {
				r := recover()
				require.NotNil(t, r)
				_, ok := r.(sdk.ErrorOutOfGas)
				require.True(t, ok, "%+v", r)
			}()

			_, _, _, _, _, _ = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, testContract.IsCosmWasmV1After, 100_000, 0)

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

func TestAllocateOnHeapMoreThanSGXHasFailBecauseMemoryLimitAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, testContract.IsCosmWasmV1After, 9_000_000, 0)

			// this should fail with memory error because 1GiB is more
			// than the allowed 12MiB, gas is 9mn so WASM gas is 900mn
			// which is bigger than the 134mn from the previous test

			require.Empty(t, data)

			require.NotNil(t, execErr)
			require.Contains(t, execErr.Error(), "the contract panicked")
		})
	}
}

func TestPassNullPointerToImportsAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

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
					_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"pass_null_pointer_to_imports_should_throw":{"pass_type":"%s"}}`, passType), false, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

					require.NotNil(t, execErr)
					require.Contains(t, execErr.Error(), "execute contract failed")
				})
			}
		})
	}
}

func TestV1ReplyLoopAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"sub_msg_loop":{"iter": 10}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(20), binary.BigEndian.Uint32(data))
		})
	}
}

func TestBankMsgSendAfterMigrate(t *testing.T) {
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
			for _, testContract := range migrateTestContracts {
				t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
					ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins(sdk.NewInt64Coin("assaf", 5000)))

					walletACoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)
					walletBCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletB)

					require.Equal(t, test.balancesBefore, walletACoinsBefore.String()+" "+walletBCoinsBefore.String())

					var err cosmwasm.StdError
					var contractAddress sdk.AccAddress

					_, _, contractAddress, _, _ = initHelperImpl(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1Before, defaultGasForTests, -1, sdk.NewCoins(sdk.NewInt64Coin("denom", 2), sdk.NewInt64Coin("assaf", 2)))

					newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
					_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
					require.Empty(t, migrateErr)

					_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","amount":%s}}`, walletB.String(), test.input), false, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

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
}

func TestSendFundsAfterMigrate(t *testing.T) {
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
									_, _, destinationAddr, _, _ = initHelperImpl(t, keeper, ctx, destinationCodeId, helperWallet, helperWallet, helperPrivKey, `{"nop":{}}`, false, destinationVersion.IsCosmWasmV1, defaultGasForTests, -1, sdk.NewCoins())

									if destinationVersion.IsCosmWasmV1 && destinationAddr != nil {
										newCodeId, newDestinationHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], helperWallet)
										destinationHash = newDestinationHash
										_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, destinationAddr, helperWallet, helperPrivKey, `{"nop":{}}`, true, true, math.MaxUint64)
										require.Empty(t, migrateErr)
									}
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
								_, _, originAddress, wasmEvents, err = initHelperImpl(t, keeper, ctx, originCodeId, fundingWallet, nil, fundingWalletPrivKey, msg, false, originVersion.IsCosmWasmV1, defaultGasForTests, -1, stringToCoins(test.balancesBefore))
							} else if originType == "exec" {
								_, _, originAddress, _, _ = initHelper(t, keeper, ctx, originCodeId, helperWallet, helperWallet, helperPrivKey, `{"nop":{}}`, false, originVersion.IsCosmWasmV1, defaultGasForTests)

								if originVersion.IsCosmWasmV1 {
									newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], helperWallet)
									_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, originAddress, helperWallet, helperPrivKey, `{"nop":{}}`, true, true, math.MaxUint64)
									require.Empty(t, migrateErr)
								}

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
									_, _, destinationAddr, _, err = initHelperImpl(t, keeper, ctx, destinationCodeId, fundingWallet, fundingWallet, fundingWalletPrivKey, `{"nop":{}}`, false, destinationVersion.IsCosmWasmV1, math.MaxUint64, wasmCount, stringToCoins(test.coinsToSend))

									if destinationVersion.IsCosmWasmV1 && destinationAddr != nil {
										newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], helperWallet)
										_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, destinationAddr, fundingWallet, fundingWalletPrivKey, `{"nop":{}}`, true, true, math.MaxUint64)
										require.Empty(t, migrateErr)
									}
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

func TestWasmMsgStructureAfterMigrate(t *testing.T) {
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
												_, _, toAddress, _, err = initHelper(t, keeper, ctx, toCodeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, to.IsCosmWasmV1, defaultGasForTests)
												require.Empty(t, err)

												if to.IsCosmWasmV1 && toAddress != nil {
													newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
													toCodeID = newCodeId
													toCodeHash = newCodeHash
													_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, toAddress, walletA, privKeyA, `{"nop":{}}`, true, true, math.MaxUint64)
													require.Empty(t, migrateErr)
												}
											}

											fromAddress := sdk.AccAddress{}
											if firstCallType == "init" {
												_, _, _, _, err = initHelper(t, keeper, ctx, fromCodeID, walletA, walletA, privKeyA, fmt.Sprintf(`{"call_to_%s":{"code_id": %d, "addr": "%s", "code_hash": "%s", "label": "%s", "msg": "%s"}}`, secondCallType, toCodeID, toAddress, toCodeHash, "blabla", test.msg), test.isErrorEncrypted, true, defaultGasForTests)
											} else if firstCallType == "exec" {
												_, _, fromAddress, _, err = initHelper(t, keeper, ctx, fromCodeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, from.IsCosmWasmV1, defaultGasForTests)
												require.Empty(t, err)
												if from.IsCosmWasmV1 && fromAddress != nil {
													newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
													_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, fromAddress, walletA, privKeyA, `{"nop":{}}`, true, true, math.MaxUint64)
													require.Empty(t, migrateErr)
												}
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

func TestCosmosMsgCustomAfterMigrate(t *testing.T) {
	for _, callType := range []string{"init", "exec"} {
		t.Run(callType, func(t *testing.T) {
			for _, testContract := range migrateTestContracts {
				t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
					ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

					var err cosmwasm.StdError
					var contractAddress sdk.AccAddress

					if callType == "init" {
						_, _, contractAddress, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, walletA, privKeyA, fmt.Sprintf(`{"cosmos_msg_custom":{}}`), false, testContract.IsCosmWasmV1Before, defaultGasForTests, -1, sdk.NewCoins())
					} else {
						_, _, contractAddress, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1Before, defaultGasForTests, -1, sdk.NewCoins())

						newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
						_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
						require.Empty(t, migrateErr)

						_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"cosmos_msg_custom":{}}`), false, testContract.IsCosmWasmV1After, math.MaxUint64, 0)
					}

					require.NotEmpty(t, err)
					require.Contains(t, err.Error(), "Custom variant not supported: invalid CosmosMsg from the contract")
				})
			}
		})
	}
}

func TestV1SendsFundsWithReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests, 200)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"send_funds_with_reply":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
		})
	}
}

func TestV1SendsFundsWithErrorWithReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"send_funds_with_error_with_reply":{}}`, false, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.NotEmpty(t, err)
			require.Contains(t, fmt.Sprintf("%+v", err), "an sdk error occoured while sending a sub-message")
		})
	}
}

func TestCallbackSanityAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
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

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, execEvents, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":2,"y":3}}`, contractAddress.String(), newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
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

func TestCodeHashExecCallExecAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, newCodeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr, newCodeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				if testContract.IsCosmWasmV1After {
					require.Contains(t,
						err.Error(),
						"Expected to parse either a `true`, `false`, or a `null`.",
					)
				} else {
					require.Contains(t,
						err.Error(),
						"Expected to parse either a `true`, `false`, or a `null`.",
					)
				}
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, newCodeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestGasIsChargedForExecCallbackToExecAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			// exec callback to exec
			_, _, _, _, _, err := execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":1,"y":2}}`, addr, newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0, 3)
			require.Empty(t, err)
		})
	}
}

func TestMsgSenderInCallbackAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"callback_to_log_msg_sender":{"to":"%s","code_hash":"%s"}}`, addr.String(), newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

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

func TestDepositToContractAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())

			require.Equal(t, `[{"denom":"denom","amount":"17"}]`, string(data))
		})
	}
}

func TestContractSendFundsAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsBefore.String())
			require.Equal(t, "199983denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, addr.String(), walletA.String(), "denom", 17), false, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "200000denom", walletCoinsAfter.String())

			require.Empty(t, execErr)
		})
	}
}

func TestContractSendFundsToExecCallbackAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			_, migrateErr = migrateHelper(t, keeper, ctx, newCodeId, addr2, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 17, newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 17)

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

func TestContractSendFundsToExecCallbackNotEnoughAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			_, migrateErr = migrateHelper(t, keeper, ctx, newCodeId, addr2, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 19, newCodeHash), false, testContract.IsCosmWasmV1After, defaultGasForTests, 17)

			require.NotNil(t, execErr)
			require.Contains(t, execErr.Error(), "insufficient funds")

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

func TestExecCallbackContractErrorAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s","code_hash":"%s"}}`, contractAddress, newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.NotNil(t, execErr)
			require.Contains(t, execErr.Error(), "la la 🤯")
			// require.Empty(t, execEvents)
			require.Empty(t, data)
		})
	}
}

func TestExecCallbackBadParamAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.NotEmpty(t, execErr)
			require.Contains(t, execErr.Error(), "unknown variant `callback_contract_bad_param`")
			require.Empty(t, data)
		})
	}
}

func TestCallbackExecuteParamErrorAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			msg := fmt.Sprintf(`{"a":{"code_hash":"%s","contract_addr":"notanaddress","x":2,"y":3}}`, newCodeHash)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, false, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.Contains(t, err.Error(), "invalid address")
		})
	}
}

func TestExecuteIllegalInputErrorAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `bad input`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.NotEmpty(t, execErr)
			require.Contains(t, execErr.Error(), "Expected to parse either a `true`, `false`, or a `null`")
		})
	}
}

func TestExecContractErrorAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			t.Run("generic_err", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"generic_err"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.NotNil(t, err.GenericErr)
				require.Contains(t, err.GenericErr.Msg, "la la 🤯")
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1After {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ra ra 🤯")
				} else {
					require.NotNil(t, err.InvalidBase64)
					require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
				}
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1After {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ka ka 🤯")
				} else {
					require.NotNil(t, err.InvalidUtf8)
					require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
				}
			})
			t.Run("not_found", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"not_found"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1After {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "za za 🤯")
				} else {
					require.NotNil(t, err.NotFound)
					require.Equal(t, "za za 🤯", err.NotFound.Kind)
				}
			})
			t.Run("parse_err", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"parse_err"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1After {
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
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"serialize_err"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1After {
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
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"unauthorized"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1After {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Unauthorized)
				}
			})
			t.Run("underflow", func(t *testing.T) {
				_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"underflow"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				if testContract.IsCosmWasmV1After {
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

func TestExecPanicAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"panic":{}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestCanonicalizeAddressErrorsAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			// this function should handle errors internally and return gracefully
			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"test_canonicalize_address_errors":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🤟", string(data))
		})
	}
}

func TestV1ReplyChainAllSuccessAfterMigrate(t *testing.T) {
	amountOfContracts := uint64(5)
	ctx, keeper, codeIds, codeHashes, walletA, privKeyA, _, _ := setupChainTest(t, TestContractPaths[v1Contract], sdk.NewCoins(), amountOfContracts)
	contractAddresses := make([]sdk.AccAddress, amountOfContracts)

	for i := uint64(0); i < amountOfContracts; i++ {
		_, _, contractAddresses[i], _, _ = initHelper(t, keeper, ctx, codeIds[i], walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)

		newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
		_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddresses[i], walletA, privKeyA, `{"nop":{}}`, true, true, math.MaxUint64)
		require.Empty(t, migrateErr)

		codeHashes[i] = newCodeHash
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

func TestV1ReplyChainPartiallyRepliedAfterMigrate(t *testing.T) {
	amountOfContracts := uint64(10)
	amountOfContractToBeReplied := uint64(5)

	ctx, keeper, codeIds, codeHashes, walletA, privKeyA, _, _ := setupChainTest(t, TestContractPaths[v1Contract], sdk.NewCoins(), amountOfContracts)
	contractAddresses := make([]sdk.AccAddress, amountOfContracts)

	for i := uint64(0); i < amountOfContracts; i++ {
		_, _, contractAddresses[i], _, _ = initHelper(t, keeper, ctx, codeIds[i], walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)

		newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
		_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddresses[i], walletA, privKeyA, `{"nop":{}}`, true, true, math.MaxUint64)
		require.Empty(t, migrateErr)

		codeHashes[i] = newCodeHash
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

func TestV1ReplyChainWithErrorAfterMigrate(t *testing.T) {
	amountOfContracts := uint64(5)
	ctx, keeper, codeIds, codeHashes, walletA, privKeyA, _, _ := setupChainTest(t, TestContractPaths[v1Contract], sdk.NewCoins(), amountOfContracts)
	contractAddresses := make([]sdk.AccAddress, amountOfContracts)

	for i := uint64(0); i < amountOfContracts; i++ {
		_, _, contractAddresses[i], _, _ = initHelper(t, keeper, ctx, codeIds[i], walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)

		newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
		_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddresses[i], walletA, privKeyA, `{"nop":{}}`, true, true, math.MaxUint64)
		require.Empty(t, migrateErr)

		codeHashes[i] = newCodeHash
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

func TestLastMsgMarkerMultipleMsgsInATxAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop": {}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			msgs := []string{`{"last_msg_marker_nop":{}}`, `{"last_msg_marker_nop":{}}`}

			results, err := execHelperMultipleMsgs(t, keeper, ctx, contractAddress, walletA, privKeyA, msgs, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)
			require.NotEqual(t, nil, err)
			require.Equal(t, 1, len(results))
		})
	}
}

func TestIBCHooksIncomingTransferAfterMigrate(t *testing.T) {
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
		for _, testContract := range migrateTestContracts {
			t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
				ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins(sdk.NewInt64Coin(test.localDenom, 1)))

				_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, initErr)

				newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
				_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
				require.Empty(t, migrateErr)

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
		}
	}
}

func TestIBCHooksOutgoingTransferAckAfterMigrate(t *testing.T) {
	for _, test := range []struct {
		name                  string
		sdkMsgSrcPort         string
		sdkMsgSrcChannel      string
		sdkMsgDestPort        string
		sdkMsgDestChannel     string
		sdkMsgAck             string
		wasmInputSrcChannel   string
		wasmInputAck          string
		wasmInputCoin         sdk.Coins
		ics20PacketSender     string
		ics20PacketMemoSender string
		err                   string
	}{
		{
			name:                  "happy path",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			sdkMsgAck:             `{"result":"AQ=="}`,
			wasmInputSrcChannel:   "channel-0",
			wasmInputAck:          "\\\"eyJyZXN1bHQiOiJBUT09In0=\\\"",
			wasmInputCoin:         sdk.NewCoins(),
			ics20PacketSender:     "",
			ics20PacketMemoSender: "",
			err:                   "",
		},
		{
			name:                  "contract address mismatch",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			sdkMsgAck:             `{"result":"AQ=="}`,
			wasmInputSrcChannel:   "channel-0",
			wasmInputAck:          "\\\"eyJyZXN1bHQiOiJBUT09In0=\\\"",
			wasmInputCoin:         sdk.NewCoins(),
			ics20PacketSender:     "secret1e8fnfznmgm67nud2uf2lrcvuy40pcdhrerph7v",
			ics20PacketMemoSender: "",
			err:                   "failed to verify transaction",
		},
		{
			name:                  "contract address mismatch 2",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			sdkMsgAck:             `{"result":"AQ=="}`,
			wasmInputSrcChannel:   "channel-0",
			wasmInputAck:          "\\\"eyJyZXN1bHQiOiJBUT09In0=\\\"",
			wasmInputCoin:         sdk.NewCoins(),
			ics20PacketSender:     "",
			ics20PacketMemoSender: "secret1e8fnfznmgm67nud2uf2lrcvuy40pcdhrerph7v",
			err:                   "failed to verify transaction",
		},
		{
			name:                  "contract address mismatch 3",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			sdkMsgAck:             `{"result":"AQ=="}`,
			wasmInputSrcChannel:   "channel-0",
			wasmInputAck:          "\\\"eyJyZXN1bHQiOiJBUT09In0=\\\"",
			wasmInputCoin:         sdk.NewCoins(),
			ics20PacketSender:     "secret19e75l25r6sa6nhdf4lggjmgpw0vmpfvsw5cnpe",
			ics20PacketMemoSender: "secret1e8fnfznmgm67nud2uf2lrcvuy40pcdhrerph7v",
			err:                   "failed to verify transaction",
		},
		{
			name:                  "channel mismatch",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			sdkMsgAck:             `{"result":"AQ=="}`,
			wasmInputSrcChannel:   "channel-1",
			wasmInputAck:          "\\\"eyJyZXN1bHQiOiJBUT09In0=\\\"",
			wasmInputCoin:         sdk.NewCoins(),
			ics20PacketSender:     "",
			ics20PacketMemoSender: "",
			err:                   "failed to verify transaction",
		},
		{
			name:                  "no coins should be sent",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			sdkMsgAck:             `{"result":"AQ=="}`,
			wasmInputSrcChannel:   "channel-0",
			wasmInputAck:          "\\\"eyJyZXN1bHQiOiJBUT09In0=\\\"",
			wasmInputCoin:         sdk.NewCoins(sdk.NewInt64Coin("denom", 1)),
			ics20PacketSender:     "",
			ics20PacketMemoSender: "",
			err:                   "failed to verify transaction",
		},
		{
			name:                  "ack mismatch",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			sdkMsgAck:             "yadayada",
			wasmInputSrcChannel:   "channel-0",
			wasmInputAck:          "\\\"eyJyZXN1bHQiOiJBUT09In0=\\\"",
			wasmInputCoin:         sdk.NewCoins(),
			ics20PacketSender:     "",
			ics20PacketMemoSender: "",
			err:                   "failed to verify transaction",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, testContract := range migrateTestContracts {
				if !testContract.IsCosmWasmV1After {
					continue
				}
				t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
					ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

					_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
					require.Empty(t, initErr)

					newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
					_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
					require.Empty(t, migrateErr)

					testIcs20PacketSender := test.ics20PacketSender
					if testIcs20PacketSender == "" {
						testIcs20PacketSender = contractAddress.String()
					}

					testIcs20PacketMemoSender := test.ics20PacketMemoSender
					if testIcs20PacketMemoSender == "" {
						testIcs20PacketMemoSender = contractAddress.String()
					}

					data := ibctransfertypes.FungibleTokenPacketData{
						Denom:    "ignored",
						Amount:   "1",
						Sender:   testIcs20PacketSender, // must be the contract address, like in the memo
						Receiver: "ignored",
						Memo:     fmt.Sprintf(`{"ibc_callback":"%s"}`, testIcs20PacketMemoSender),
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
									{Key: "ibc_lifecycle_complete.ibc_ack.ack", Value: strings.ReplaceAll(test.wasmInputAck, "\\", "")},
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
		})
	}
}

func TestIBCHooksOutgoingTransferTimeoutAfterMigrate(t *testing.T) {
	for _, test := range []struct {
		name                  string
		sdkMsgSrcPort         string
		sdkMsgSrcChannel      string
		sdkMsgDestPort        string
		sdkMsgDestChannel     string
		wasmInputSrcChannel   string
		wasmInputCoin         sdk.Coins
		ics20PacketSender     string
		ics20PacketMemoSender string
		err                   string
	}{
		{
			name:                "happy path",
			sdkMsgSrcPort:       "transfer",
			sdkMsgSrcChannel:    "channel-0",
			sdkMsgDestPort:      "transfer",
			sdkMsgDestChannel:   "channel-1",
			wasmInputSrcChannel: "channel-0",
			wasmInputCoin:       sdk.NewCoins(),
			err:                 "",
		},
		{
			name:                  "contract address mismatch",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			wasmInputSrcChannel:   "channel-0",
			wasmInputCoin:         sdk.NewCoins(),
			ics20PacketSender:     "secret1e8fnfznmgm67nud2uf2lrcvuy40pcdhrerph7v",
			ics20PacketMemoSender: "",
			err:                   "failed to verify transaction",
		},
		{
			name:                  "contract address mismatch 2",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			wasmInputSrcChannel:   "channel-0",
			wasmInputCoin:         sdk.NewCoins(),
			ics20PacketSender:     "",
			ics20PacketMemoSender: "secret1e8fnfznmgm67nud2uf2lrcvuy40pcdhrerph7v",
			err:                   "failed to verify transaction",
		},
		{
			name:                  "contract address mismatch 3",
			sdkMsgSrcPort:         "transfer",
			sdkMsgSrcChannel:      "channel-0",
			sdkMsgDestPort:        "transfer",
			sdkMsgDestChannel:     "channel-1",
			wasmInputSrcChannel:   "channel-0",
			wasmInputCoin:         sdk.NewCoins(),
			ics20PacketSender:     "secret19e75l25r6sa6nhdf4lggjmgpw0vmpfvsw5cnpe",
			ics20PacketMemoSender: "secret1e8fnfznmgm67nud2uf2lrcvuy40pcdhrerph7v",
			err:                   "failed to verify transaction",
		},
		{
			name:                "channel mismatch",
			sdkMsgSrcPort:       "transfer",
			sdkMsgSrcChannel:    "channel-0",
			sdkMsgDestPort:      "transfer",
			sdkMsgDestChannel:   "channel-1",
			wasmInputSrcChannel: "channel-1",
			wasmInputCoin:       sdk.NewCoins(),
			err:                 "failed to verify transaction",
		},
		{
			name:                "no coins should be sent",
			sdkMsgSrcPort:       "transfer",
			sdkMsgSrcChannel:    "channel-0",
			sdkMsgDestPort:      "transfer",
			sdkMsgDestChannel:   "channel-1",
			wasmInputSrcChannel: "channel-0",
			wasmInputCoin:       sdk.NewCoins(sdk.NewInt64Coin("denom", 1)),
			err:                 "failed to verify transaction",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, testContract := range migrateTestContracts {
				if !testContract.IsCosmWasmV1After {
					continue
				}
				t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
					ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

					_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
					require.Empty(t, initErr)

					newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
					_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
					require.Empty(t, migrateErr)

					testIcs20PacketSender := test.ics20PacketSender
					if testIcs20PacketSender == "" {
						testIcs20PacketSender = contractAddress.String()
					}

					testIcs20PacketMemoSender := test.ics20PacketMemoSender
					if testIcs20PacketMemoSender == "" {
						testIcs20PacketMemoSender = contractAddress.String()
					}

					data := ibctransfertypes.FungibleTokenPacketData{
						Denom:    "ignored",
						Amount:   "1",
						Sender:   testIcs20PacketSender, // must be the contract address, like in the memo
						Receiver: "ignored",
						Memo:     fmt.Sprintf(`{"ibc_callback":"%s"}`, testIcs20PacketMemoSender),
					}
					dataBytes, err := json.Marshal(data)
					require.NoError(t, err)

					sdkMsg := ibcchanneltypes.MsgTimeout{
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
						ProofUnreceived: []byte{},
						ProofHeight:     ibcclienttypes.Height{},
						Signer:          walletA.String(),
					}

					ctx = PrepareSignedTx(t, keeper, ctx, walletA, privKeyA, &sdkMsg)

					_, execErr := keeper.Execute(ctx,
						contractAddress,
						walletA,
						[]byte(
							fmt.Sprintf(`{"ibc_lifecycle_complete":{"ibc_timeout":{"channel":"%s","sequence":0}}}`,
								test.wasmInputSrcChannel,
							)),
						test.wasmInputCoin,
						nil,
						cosmwasm.HandleTypeIbcWasmHooksOutgoingTransferTimeout,
					)

					if test.err == "" {
						require.Empty(t, execErr)
						events := tryDecryptWasmEvents(ctx, nil)
						requireEvents(t,
							[]ContractEvent{
								{
									{Key: "contract_address", Value: contractAddress.String()},
									{Key: "ibc_lifecycle_complete.ibc_timeout.channel", Value: test.sdkMsgSrcChannel},
									{Key: "ibc_lifecycle_complete.ibc_timeout.sequence", Value: "0"},
								},
							},
							events,
						)
					} else {
						require.Contains(t, execErr.Error(), test.err)
					}
				})
			}
		})
	}
}

/// copy of exec tests but doing it during a migration:

func TestStateDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(data))

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"remove_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Empty(t, data)

			_, _, _, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(data))

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			migrateResult, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, err)
			require.Equal(t, "🍌", string(migrateResult.Data))

			migrateResult, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.Empty(t, execErr)

			migrateResult, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(migrateResult.Data))

			migrateResult, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"remove_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.Empty(t, execErr)

			migrateResult, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.Empty(t, execErr)
			require.Empty(t, string(migrateResult.Data))

			migrateResult, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"set_state":{"key":"banana","value":"🍌"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.Empty(t, execErr)

			migrateResult, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(migrateResult.Data))

			_, _, data, _, _, execErr = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"get_state":{"key":"banana"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Equal(t, "🍌", string(data))
		})
	}
}

func TestAddrValidateFunctionDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			migrateResult, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"validate_address":{"addr":"%s"}}`, contractAddress), true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			resp, aErr := sdk.AccAddressFromBech32(string(migrateResult.Data))
			require.Empty(t, aErr)
			require.Equal(t, resp, contractAddress)

			migrateResult, migrateErr = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"validate_address":{"addr":"secret18vd8fpwxzck93qlwghaj6arh4p7c5nyf7hmag8"}}`), true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)
			require.Equal(t, string(migrateResult.Data), "\"Apple\"")
		})
	}
}

func TestEnvDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			migrateResult, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"get_env":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			migrateEvent := migrateResult.WasmEvents[0]
			envAttributeIndex := slices.IndexFunc(migrateEvent, func(c v010types.LogAttribute) bool { return c.Key == "env" })
			envAttribute := migrateEvent[envAttributeIndex]

			var actualMigrateEnv cosmwasm.Env
			json.Unmarshal([]byte(envAttribute.Value), &actualMigrateEnv)

			expectedV1EnvMigrate := fmt.Sprintf(
				`{"block":{"height":%d,"time":"%d","chain_id":"%s","random":"%s"},"transaction":null,"contract":{"address":"%s","code_hash":"%s"}}`,
				ctx.BlockHeight(),
				// env.block.time is nanoseconds since unix epoch
				ctx.BlockTime().UnixNano(),
				ctx.ChainID(),
				base64.StdEncoding.EncodeToString(actualMigrateEnv.Block.Random),
				contractAddress.String(),
				calcCodeHash(testContract.WasmFilePathAfter),
			)

			requireEventsInclude(t,
				migrateResult.WasmEvents,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{
							Key:   "env",
							Value: expectedV1EnvMigrate,
						},
					},
				},
			)
		})
	}
}

func TestNestedAttributeDuringMigrate(t *testing.T) {
	// For more info: https://github.com/scrtlabs/SecretNetwork/issues/1235
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			migrateResult, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_attribute_step1":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr2", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr4", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr_reply", Value: "🦄"},
					},
				},
				migrateResult.WasmEvents,
			)

			require.Equal(t, string(migrateResult.Data), "\"reply\"")
		})
	}
}

func TestEmptyLogKeyValueDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			migrateResult, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"empty_log_key_value":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "my value is empty", Value: ""},
						{Key: "", Value: "my key is empty"},
					},
				},
				migrateResult.WasmEvents,
			)
		})
	}
}

func TestExecNoLogsDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			migrateResult, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"no_logs":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
					},
				},
				migrateResult.WasmEvents,
			)
		})
	}
}

func TestEmptyDataDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			migrateResult, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"empty_data":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)
			require.Empty(t, migrateResult.Data)
		})
	}
}

func TestNoDataDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			migrateResult, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"no_data":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)
			require.Empty(t, migrateResult.Data)
		})
	}
}

func TestUnicodeDataDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			migrateResult, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)
			require.Equal(t, "🍆🥑🍄", string(migrateResult.Data))
		})
	}
}

func TestSecp256k1VerifyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			// https://paulmillr.com/noble/

			t.Run("CorrectCompactPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("CorrectLongPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectMsgHashCompactPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectMsgHashLongPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzas="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectSigCompactPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectSigLongPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BEZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo///ne03QpL+5WFHztzVceB3WD4QY/Ipl0UkHr/R8kDpVk=","sig":"rhZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectCompactPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"AoSdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXc","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectLongPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":1,"pubkey":"BISdDHH9J0Bfb9pT8GFn+bW9cEVkgIh4bFsepMWmczXcFWl11YCgu65hzvNDQE2Qo1hwTMQ/42Xif8O/MrxzvxI=","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
		})
	}
}

func TestEd25519VerifyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectMsg", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmUK"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectSig", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_verify":{"iterations":1,"pubkey":"DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs=","sig":"8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","msg":"YXNzYWYgd2FzIGhlcmU="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
		})
	}
}

func TestEd25519BatchVerifyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			// https://paulmillr.com/noble/
			t.Run("Correct", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("100Correct", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU=","YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectPubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["DV1lgRdKw7nt4hvl8XkGZXMzU9S3uM9NLTK0h0qMbUs="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectMsg", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg=="],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("IncorrectSig", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDw=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "false"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("CorrectEmptySigsEmptyMsgsOnePubkey", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA="],"sigs":[],"msgs":[]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("CorrectEmpty", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":[]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("CorrectEmptyPubkeysEmptySigsOneMsg", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":[],"sigs":[],"msgs":["YXNzYWYgd2FzIGhlcmUK"]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("CorrectMultisig", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["LO2+Bt+/FIjomSaPB+I++LXkxgxwfnrKHLyvCic72rA=","2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["8O7nwhM71/B9srKwe8Ps39z5lAsLMMs6LxdvoPk0HXjEM97TNhKbdU6gEePT2MaaIUSiMEmoG28HIZMgMRTCDg==","bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg=="],"msgs":["YXNzYWYgd2FzIGhlcmU="]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("CorrectMultiMsgOneSigner", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_batch_verify":{"iterations":1,"pubkeys":["2ukhmWRNmcgCrB9fpLP9/HZVuJn6AhpITf455F4GsbM="],"sigs":["bp/N4Ub2WFk9SE9poZVEanU1l46WMrFkTd5wQIXi6QJKjvZUi7+GTzmTe8y2yzgpBI+GWQmt0/QwYbnSVxq/Cg==","uuNxLEzAYDbuJ+BiYN94pTqhD7UhvCJNbxAbnWz0B9DivkPXmqIULko0DddP2/tVXPtjJ90J20faiWCEC3QkDg=="],"msgs":["YXNzYWYgd2FzIGhlcmU=","cGVhY2Ugb3V0"]}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "result", Value: "true"},
						},
					},
					res.WasmEvents,
				)
			})
		})
	}
}

func TestSecp256k1RecoverPubkeyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			// https://paulmillr.com/noble/
			res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":0,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.Empty(t, err)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//"},
					},
				},
				res.WasmEvents,
			)

			res, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_recover_pubkey":{"iterations":1,"recovery_param":1,"sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.Empty(t, err)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "Ams198xOCEVnc/ESvxF2nxnE3AVFO8ahB22S1ZgX2vSR"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestSecp256k1SignDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			// priv iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao=
			// pub ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"secp256k1_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"iadRiuRKNZvAXwolxqzJvr60uiMDJTxOEzEwV8OK2ao="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.Empty(t, err)

			signature := res.WasmEvents[0][1].Value

			res, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"secp256k1_verify":{"iterations":1,"pubkey":"ArQojoh5TVlSSNA1HFlH5HcQsv0jnrpeE7hgwR/N46nS","sig":"%s","msg_hash":"K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA="}}`, signature), true, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.Empty(t, err)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "true"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestEd25519SignDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			// priv z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo=
			// pub jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=
			// msg d2VuIG1vb24=
			// msg_hash K9vGEuzCYCUcIXlhMZu20ke2K4mJhreguYct5MqAzhA=

			// https://paulmillr.com/noble/
			res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"ed25519_sign":{"iterations":1,"msg":"d2VuIG1vb24=","privkey":"z01UNefH2yjRslwZMmcHssdHmdEjzVvbxjr+MloUEYo="}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.Empty(t, err)

			signature := res.WasmEvents[0][1].Value

			res, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"ed25519_verify":{"iterations":1,"pubkey":"jh58UkC0FDsiupZBLdaqKUqYubJbk3LDaruZiJiy0Po=","sig":"%s","msg":"d2VuIG1vb24="}}`, signature), true, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.Empty(t, err)
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "result", Value: "true"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestSleepDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			_, execErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"sleep":{"ms":3000}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.Error(t, execErr)
			require.Error(t, execErr.CosmWasm.GenericErr)
			require.Contains(t, execErr.CosmWasm.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseMemoryLimitDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"allocate_on_heap":{"bytes":12582913}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests)

			// this should fail with memory error because 12MiB+1 is more than the allowed 12MiB

			require.Empty(t, res.Data)

			require.NotNil(t, err.CosmWasm.GenericErr)
			require.Contains(t, err.CosmWasm.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestAllocateOnHeapFailBecauseGasLimitDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			// ensure we get an out of gas panic
			defer func() {
				r := recover()
				require.NotNil(t, r)
				_, ok := r.(sdk.ErrorOutOfGas)
				require.True(t, ok, "%+v", r)
			}()

			_, _ = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, testContract.IsCosmWasmV1After, 100_000)

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

func TestAllocateOnHeapMoreThanSGXHasFailBecauseMemoryLimitDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			res, execErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"allocate_on_heap":{"bytes":1073741824}}`, false, testContract.IsCosmWasmV1After, 9_000_000)

			// this should fail with memory error because 1GiB is more
			// than the allowed 12MiB, gas is 9mn so WASM gas is 900mn
			// which is bigger than the 134mn from the previous test

			require.Empty(t, res.Data)

			require.NotNil(t, execErr.CosmWasm.GenericErr)
			require.Contains(t, execErr.CosmWasm.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestPassNullPointerToImportsDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

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
					_, execErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"pass_null_pointer_to_imports_should_throw":{"pass_type":"%s"}}`, passType), false, testContract.IsCosmWasmV1After, defaultGasForTests)

					require.NotNil(t, execErr.CosmWasm.GenericErr)
					require.Contains(t, execErr.CosmWasm.GenericErr.Msg, "migrate contract failed")
				})
			}
		})
	}
}

func TestV1ReplyLoopDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"sub_msg_loop":{"iter": 10}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, err)
			require.Equal(t, uint32(20), binary.BigEndian.Uint32(res.Data))
		})
	}
}

func TestBankMsgSendDuringMigrate(t *testing.T) {
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
			for _, testContract := range migrateTestContracts {
				t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
					ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins(sdk.NewInt64Coin("assaf", 5000)))

					walletACoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)
					walletBCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletB)

					require.Equal(t, test.balancesBefore, walletACoinsBefore.String()+" "+walletBCoinsBefore.String())

					var contractAddress sdk.AccAddress

					_, _, contractAddress, _, _ = initHelperImpl(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1Before, defaultGasForTests, -1, sdk.NewCoins(sdk.NewInt64Coin("denom", 2), sdk.NewInt64Coin("assaf", 2)))

					newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

					_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","amount":%s}}`, walletB.String(), test.input), false, testContract.IsCosmWasmV1After, math.MaxUint64)

					if test.isSuccuss {
						require.Empty(t, migrateErr)
					} else {
						require.NotEmpty(t, migrateErr)
						require.Equal(t, migrateErr.Error(), test.errorMsg)
					}

					walletACoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)
					walletBCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletB)

					require.Equal(t, test.balancesAfter, walletACoinsAfter.String()+" "+walletBCoinsAfter.String())
				})
			}
		})
	}
}

func TestWasmMsgStructureDuringMigrate(t *testing.T) {
	for _, from := range migrateTestContracts {
		t.Run(fmt.Sprintf("from %s", from.CosmWasmVersionBefore), func(t *testing.T) {
			for _, to := range migrateTestContracts {
				if !to.IsCosmWasmV1After {
					continue
				}
				t.Run(fmt.Sprintf("to %s", to.CosmWasmVersionAfter), func(t *testing.T) {
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
											ctx, keeper, fromCodeID, _, walletA, privKeyA, _, _ := setupTest(t, from.WasmFilePathBefore, sdk.NewCoins())

											wasmCode, err := os.ReadFile(to.WasmFilePathBefore)
											require.NoError(t, err)

											toCodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
											codeInfo, err := keeper.GetCodeInfo(ctx, toCodeID)
											require.NoError(t, err)
											toCodeHash := hex.EncodeToString(codeInfo.CodeHash)
											require.NoError(t, err)

											toAddress := sdk.AccAddress{}
											if secondCallType != "init" {
												_, _, toAddress, _, err = initHelper(t, keeper, ctx, toCodeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, to.IsCosmWasmV1Before, defaultGasForTests)
												require.Empty(t, err)
											}

											fromAddress := sdk.AccAddress{}
											if firstCallType == "init" {
												_, _, _, _, err = initHelper(t, keeper, ctx, fromCodeID, walletA, walletA, privKeyA, fmt.Sprintf(`{"call_to_%s":{"code_id": %d, "addr": "%s", "code_hash": "%s", "label": "%s", "msg": "%s"}}`, secondCallType, toCodeID, toAddress, toCodeHash, "blabla", test.msg), test.isErrorEncrypted, true, defaultGasForTests)
											} else if firstCallType == "exec" {
												_, _, fromAddress, _, err = initHelper(t, keeper, ctx, fromCodeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, from.IsCosmWasmV1Before, defaultGasForTests)
												require.Empty(t, err)

												newCodeId, _ := uploadCode(ctx, t, keeper, to.WasmFilePathAfter, walletA)
												_, err = migrateHelper(t, keeper, ctx, newCodeId, fromAddress, walletA, privKeyA, fmt.Sprintf(`{"call_to_%s":{"code_id": %d, "addr": "%s", "code_hash": "%s", "label": "%s", "msg": "%s"}}`, secondCallType, toCodeID, toAddress, toCodeHash, "blabla", test.msg), test.isErrorEncrypted, true, math.MaxUint64)
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

func TestCosmosMsgCustomDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1Before, defaultGasForTests, -1, sdk.NewCoins())
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"cosmos_msg_custom":{}}`, false, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Contains(t, migrateErr.Error(), "Custom variant not supported: invalid CosmosMsg from the contract")
		})
	}
}

func TestV1SendsFundsWithReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1Before, defaultGasForTests, 200)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"send_funds_with_reply":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, migErr)
		})
	}
}

func TestV1SendsFundsWithErrorWithReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"send_funds_with_error_with_reply":{}}`, false, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.NotEmpty(t, err)
			require.Contains(t, fmt.Sprintf("%+v", err), "an sdk error occoured while sending a sub-message")
		})
	}
}

func TestCallbackSanityDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, initEvents, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
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

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":2,"y":3}}`, contractAddress.String(), newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.Empty(t, migErr)

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
				res.WasmEvents,
			)

			require.Equal(t, []byte{2, 3}, res.Data)
		})
	}
}

func TestCodeHashExecCallExecDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			t.Run("GoodCodeHash", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, newCodeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)

				var newContractBech32 string
				for _, v := range res.WasmEvents[1] {
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
					res.WasmEvents,
				)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr, newCodeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr, newCodeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr, `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestGasIsChargedForExecCallbackToExecDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"a":{"contract_addr":"%s","code_hash":"%s","x":1,"y":2}}`, addr, newCodeHash), true, testContract.IsCosmWasmV1After, math.MaxUint64, 3)
			require.Empty(t, migrateErr)
		})
	}
}

func TestMsgSenderInCallbackDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"callback_to_log_msg_sender":{"to":"%s","code_hash":"%s"}}`, addr.String(), newCodeHash), true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			requireEvents(t, []ContractEvent{
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "hi", Value: "hey"},
				},
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "msg.sender", Value: addr.String()},
				},
			}, res.WasmEvents)
		})
	}
}

func TestContractSendFundsDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1Before, defaultGasForTests, 17)

			require.Empty(t, execErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "17denom", contractCoinsBefore.String())
			require.Equal(t, "199983denom", walletCoinsBefore.String())

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, addr.String(), walletA.String(), "denom", 17), true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "200000denom", walletCoinsAfter.String())
		})
	}
}

func TestContractSendFundsToExecCallbackDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1Before, defaultGasForTests, 17)
			require.Empty(t, execErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 17, codeHash), true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "17denom", contract2CoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
		})
	}
}

func TestContractSendFundsToExecCallbackNotEnoughDuringMigrate(t *testing.T,
) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, addr2, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "", contract2CoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, testContract.IsCosmWasmV1Before, defaultGasForTests, 17)
			require.Empty(t, execErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			_, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_exec_callback":{"to":"%s","denom":"%s","amount":%d,"code_hash":"%s"}}`, addr2.String(), "denom", 19, codeHash), false, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.NotNil(t, migrateErr.CosmWasm.GenericErr)
			require.Contains(t, migrateErr.CosmWasm.GenericErr.Msg, "insufficient funds")

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			contract2CoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr2)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			// The state here should have been reverted by the app but in go-tests we create our own keeper
			// so it is not reverted in this case.
			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "", contract2CoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
		})
	}
}

func TestExecCallbackContractErrorDuring(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s","code_hash":"%s"}}`, contractAddress, newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests) // using the new code hash because migration is done by the time the callback is executed

			require.NotNil(t, migErr.CosmWasm.GenericErr)
			require.Contains(t, migErr.CosmWasm.GenericErr.Msg, "la la 🤯")
			require.Empty(t, res.Data)
		})
	}
}

func TestExecCallbackBadParamDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.NotEmpty(t, err)
			require.Contains(t, err.Error(), "unknown variant `callback_contract_bad_param`")
			require.Empty(t, res.Data)
		})
	}
}

func TestCallbackExecuteParamErrorDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			msg := fmt.Sprintf(`{"a":{"code_hash":"%s","contract_addr":"notanaddress","x":2,"y":3}}`, newCodeHash)

			_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, msg, false, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.Contains(t, err.Error(), "invalid address")
		})
	}
}

func TestExecuteIllegalInputErrorDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `bad input`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.NotEmpty(t, err)
			require.Contains(t, err.Error(), "Expected to parse either a `true`, `false`, or a `null`.")
		})
	}
}

func TestExecContractErrorDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			t.Run("generic_err", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"generic_err"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotNil(t, err.CosmWasm.GenericErr)
				require.Contains(t, err.CosmWasm.GenericErr.Msg, "la la 🤯")
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				if testContract.IsCosmWasmV1After {
					require.NotNil(t, err.CosmWasm.GenericErr)
					require.Contains(t, err.CosmWasm.GenericErr.Msg, "ra ra 🤯")
				} else {
					require.NotNil(t, err.CosmWasm.InvalidBase64)
					require.Equal(t, "ra ra 🤯", err.CosmWasm.InvalidBase64.Msg)
				}
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				if testContract.IsCosmWasmV1After {
					require.NotNil(t, err.CosmWasm.GenericErr)
					require.Contains(t, err.CosmWasm.GenericErr.Msg, "ka ka 🤯")
				} else {
					require.NotNil(t, err.CosmWasm.InvalidUtf8)
					require.Equal(t, "ka ka 🤯", err.CosmWasm.InvalidUtf8.Msg)
				}
			})
			t.Run("not_found", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"not_found"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				if testContract.IsCosmWasmV1After {
					require.NotNil(t, err.CosmWasm.GenericErr)
					require.Contains(t, err.CosmWasm.GenericErr.Msg, "za za 🤯")
				} else {
					require.NotNil(t, err.CosmWasm.NotFound)
					require.Equal(t, "za za 🤯", err.CosmWasm.NotFound.Kind)
				}
			})
			t.Run("parse_err", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"parse_err"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				if testContract.IsCosmWasmV1After {
					require.NotNil(t, err.CosmWasm.GenericErr)
					require.Contains(t, err.CosmWasm.GenericErr.Msg, "na na 🤯")
					require.Contains(t, err.CosmWasm.GenericErr.Msg, "pa pa 🤯")
				} else {
					require.NotNil(t, err.CosmWasm.ParseErr)
					require.Equal(t, "na na 🤯", err.CosmWasm.ParseErr.Target)
					require.Equal(t, "pa pa 🤯", err.CosmWasm.ParseErr.Msg)
				}
			})
			t.Run("serialize_err", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"serialize_err"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				if testContract.IsCosmWasmV1After {
					require.NotNil(t, err.CosmWasm.GenericErr)
					require.Contains(t, err.CosmWasm.GenericErr.Msg, "ba ba 🤯")
					require.Contains(t, err.CosmWasm.GenericErr.Msg, "ga ga 🤯")
				} else {
					require.NotNil(t, err.CosmWasm.SerializeErr)
					require.Equal(t, "ba ba 🤯", err.CosmWasm.SerializeErr.Source)
					require.Equal(t, "ga ga 🤯", err.CosmWasm.SerializeErr.Msg)
				}
			})
			t.Run("unauthorized", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"unauthorized"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				if testContract.IsCosmWasmV1After {
					// Not supported in V1
					require.NotNil(t, err.CosmWasm.GenericErr)
					require.Contains(t, err.CosmWasm.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.CosmWasm.Unauthorized)
				}
			})
			t.Run("underflow", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddr, walletA, privKeyA, `{"contract_error":{"error_type":"underflow"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				if testContract.IsCosmWasmV1After {
					// Not supported in V1
					require.NotNil(t, err.CosmWasm.GenericErr)
					require.Contains(t, err.CosmWasm.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.CosmWasm.Underflow)
					require.Equal(t, "minuend 🤯", err.CosmWasm.Underflow.Minuend)
					require.Equal(t, "subtrahend 🤯", err.CosmWasm.Underflow.Subtrahend)
				}
			})
		})
	}
}

func TestExecPanicDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			_, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"panic":{}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.NotNil(t, err.CosmWasm.GenericErr)
			require.Contains(t, err.CosmWasm.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestCanonicalizeAddressErrorsDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			// this function should handle errors internally and return gracefully
			res, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"test_canonicalize_address_errors":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migrateErr)

			require.Equal(t, "🤟", string(res.Data))
		})
	}
}

func TestV1ReplyChainAllSuccessDuringMigrate(t *testing.T) {
	amountOfContracts := uint64(5)
	ctx, keeper, codeIds, codeHashes, walletA, privKeyA, _, _ := setupChainTest(t, TestContractPaths[v1Contract], sdk.NewCoins(), amountOfContracts)
	contractAddresses := make([]sdk.AccAddress, amountOfContracts)

	for i := uint64(0); i < amountOfContracts; i++ {
		_, _, contractAddresses[i], _, _ = initHelper(t, keeper, ctx, codeIds[i], walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	}

	executeDetails := make([]ExecuteDetails, amountOfContracts-1)
	for i := uint64(1); i < amountOfContracts; i++ {
		executeDetails[i-1] = ExecuteDetails{
			ContractAddress: contractAddresses[i].String(),
			ContractHash:    codeHashes[i], // using the original code hash as only contractAddresses[0] is being migrated
			ShouldError:     false,
			MsgId:           9000,
			Data:            fmt.Sprintf("%d", i),
		}
	}

	marshaledDetails, err := json.Marshal(executeDetails)
	require.Empty(t, err)

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)

	res, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddresses[0], walletA, privKeyA, fmt.Sprintf(`{"execute_multiple_contracts":{"details":%s}}`, string(marshaledDetails)), true, true, math.MaxUint64)
	require.Empty(t, migrateErr)

	expectedFlow := ""
	for i := uint64(amountOfContracts - 1); i > 0; i-- {
		expectedFlow += contractAddresses[i].String() + " -> "
	}

	expectedFlow += contractAddresses[0].String()

	require.Equal(t, expectedFlow, string(res.Data))
}

func TestV1ReplyChainPartiallyRepliedDuringMigrate(t *testing.T) {
	amountOfContracts := uint64(10)
	amountOfContractToBeReplied := uint64(5)

	ctx, keeper, codeIds, codeHashes, walletA, privKeyA, _, _ := setupChainTest(t, TestContractPaths[v1Contract], sdk.NewCoins(), amountOfContracts)
	contractAddresses := make([]sdk.AccAddress, amountOfContracts)

	for i := uint64(0); i < amountOfContracts; i++ {
		_, _, contractAddresses[i], _, _ = initHelper(t, keeper, ctx, codeIds[i], walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	}

	executeDetails := make([]ExecuteDetails, amountOfContracts-1)
	for i := uint64(1); i < amountOfContracts; i++ {
		msgId := uint64(9000)
		if i >= amountOfContractToBeReplied {
			msgId = 0
		}

		executeDetails[i-1] = ExecuteDetails{
			ContractAddress: contractAddresses[i].String(),
			ContractHash:    codeHashes[i], // using the original code hash as only contractAddresses[0] is being migrated
			ShouldError:     false,
			MsgId:           msgId,
			Data:            fmt.Sprintf("%d", i),
		}
	}

	marshaledDetails, err := json.Marshal(executeDetails)
	require.Empty(t, err)

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	res, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddresses[0], walletA, privKeyA, fmt.Sprintf(`{"execute_multiple_contracts":{"details": %s}}`, string(marshaledDetails)), true, true, math.MaxUint64)
	require.Empty(t, migrateErr)

	expectedFlow := ""

	expectedFlow += fmt.Sprintf("%d", amountOfContractToBeReplied) + " -> "

	for i := uint64(amountOfContractToBeReplied - 2); i > 0; i-- {
		expectedFlow += contractAddresses[i].String() + " -> "
	}

	expectedFlow += contractAddresses[0].String()

	require.Equal(t, expectedFlow, string(res.Data))
}

func TestV1ReplyChainWithErrorDuringMigrate(t *testing.T) {
	amountOfContracts := uint64(5)
	ctx, keeper, codeIds, codeHashes, walletA, privKeyA, _, _ := setupChainTest(t, TestContractPaths[v1Contract], sdk.NewCoins(), amountOfContracts)
	contractAddresses := make([]sdk.AccAddress, amountOfContracts)

	for i := uint64(0); i < amountOfContracts; i++ {
		_, _, contractAddresses[i], _, _ = initHelper(t, keeper, ctx, codeIds[i], walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	}

	executeDetails := make([]ExecuteDetails, amountOfContracts-1)
	for i := uint64(1); i < amountOfContracts; i++ {
		executeDetails[i-1] = ExecuteDetails{
			ContractAddress: contractAddresses[i].String(),
			ContractHash:    codeHashes[i], // using the original code hash as only contractAddresses[0] is being migrated
			ShouldError:     false,
			MsgId:           9000,
			Data:            fmt.Sprintf("%d", i),
		}
	}

	executeDetails[amountOfContracts-2].ShouldError = true

	marshaledDetails, err := json.Marshal(executeDetails)
	require.Empty(t, err)

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	res, migrateErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddresses[0], walletA, privKeyA, fmt.Sprintf(`{"execute_multiple_contracts":{"details": %s}}`, string(marshaledDetails)), true, true, math.MaxUint64)
	require.Empty(t, migrateErr)

	expectedFlow := "err -> "
	for i := uint64(amountOfContracts - 4); i > 0; i-- {
		expectedFlow += contractAddresses[i].String() + " -> "
	}

	expectedFlow += contractAddresses[0].String()

	require.Equal(t, expectedFlow, string(res.Data))
}

func TestLastMsgMarkerDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			// reset value before test
			keeper.LastMsgManager.SetMarker(false)

			_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"last_msg_marker_nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, err)

			require.True(t, keeper.LastMsgManager.GetMarker())
		})
	}
}

func TestQueryInputParamErrorAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"call_to_query":{"addr":"blabla","code_hash":"yadayada","msg":"hi"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.NotNil(t, qErr.GenericErr)
			require.Contains(t, qErr.Error(), "blabla: invalid address")
		})
	}
}

func TestQueryContractErrorAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			t.Run("generic_err", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddress, `{"contract_error":{"error_type":"generic_err"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "la la 🤯")
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddress, `{"contract_error":{"error_type":"invalid_base64"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "ra ra 🤯")
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddress, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "ka ka 🤯")
			})
			t.Run("not_found", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddress, `{"contract_error":{"error_type":"not_found"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "za za 🤯")
			})
			t.Run("parse_err", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddress, `{"contract_error":{"error_type":"parse_err"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "na na 🤯")
				require.Contains(t, err.Error(), "pa pa 🤯")
			})
			t.Run("serialize_err", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, contractAddress, `{"contract_error":{"error_type":"serialize_err"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "ba ba 🤯")
				require.Contains(t, err.Error(), "ga ga 🤯")
			})
		})
	}
}

func TestQueryInputStructureErrorAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"call_to_query":{"invalidkey":"invalidval"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests)

			require.NotEmpty(t, qErr)
			require.Contains(t, qErr.Error(), "missing field `addr`")
		})
	}
}

func TestQueryPanicAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, queryErr := queryHelper(t, keeper, ctx, addr, `{"panic":{}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.NotEmpty(t, queryErr.GenericErr)
			require.Contains(t, queryErr.Error(), "the contract panicked")
		})
	}
}

func TestExternalQueryWorksAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, data, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query":{"to":"%s","code_hash":"%s"}}`, addr.String(), newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.Empty(t, execErr)
			require.Equal(t, []byte{3}, data)
		})
	}
}

func TestExternalQueryWorksDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, migErr)
			require.Equal(t, []byte{3}, res.Data)
		})
	}
}

func TestExternalQueryCalleePanicAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_panic":{"to":"%s","code_hash":"%s"}}`, addr.String(), newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestExternalQueryCalleePanicDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_panic":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.NotEmpty(t, migErr)
			require.NotNil(t, migErr.CosmWasm)
			require.NotNil(t, migErr.CosmWasm.GenericErr)
			require.Contains(t, migErr.CosmWasm.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestExternalQueryCalleeStdErrorAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_error":{"to":"%s","code_hash":"%s"}}`, addr.String(), newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "la la 🤯")
		})
	}
}

func TestExternalQueryCalleeStdErrorDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_error":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.NotNil(t, migErr.CosmWasm)
			require.NotNil(t, migErr.CosmWasm.GenericErr)
			require.Contains(t, migErr.CosmWasm.GenericErr.Msg, "la la 🤯")
		})
	}
}

func TestExternalQueryCalleeDoesntExistAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"send_external_query_error":{"to":"secret13l72vhjngmg55ykajxdnlalktwglyqjqv9pkq4","code_hash":"bla bla"}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "contract: not found")
		})
	}
}

func TestExternalQueryCalleeDoesntExistDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"send_external_query_error":{"to":"secret13l72vhjngmg55ykajxdnlalktwglyqjqv9pkq4","code_hash":"bla bla"}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.NotNil(t, migErr.CosmWasm)
			require.NotNil(t, migErr.CosmWasm.GenericErr)
			require.Contains(t, migErr.CosmWasm.GenericErr.Msg, "contract: not found")
		})
	}
}

func TestExternalQueryBadSenderAbiAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_bad_abi":{"to":"%s","code_hash":"%s"}}`, addr.String(), newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.NotEmpty(t, err)
			require.Contains(t, err.Error(), "Invalid type")
		})
	}
}

func TestExternalQueryBadSenderAbiDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_bad_abi":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.NotEmpty(t, migErr)
			require.Contains(t, migErr.Error(), "Invalid type")
		})
	}
}

func TestExternalQueryBadReceiverAbiAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_bad_abi_receiver":{"to":"%s","code_hash":"%s"}}`, addr.String(), newCodeHash), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

			require.NotEmpty(t, err)
			require.Contains(t, err.Error(), "alloc::string::String")
			require.Contains(t, err.Error(), "Invalid type")
		})
	}
}

func TestExternalQueryBadReceiverAbiDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_bad_abi_receiver":{"to":"%s","code_hash":"%s"}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.NotEmpty(t, migErr)
			require.Contains(t, migErr.Error(), "alloc::string::String")
			require.Contains(t, migErr.Error(), "Invalid type")
		})
	}
}

func TestQueryRecursionLimitEnforcedInQueriesAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			data, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1After, 10*defaultGasForTests)

			require.NotEmpty(t, data)
			require.Equal(t, data, `"Recursion limit was correctly enforced"`)

			require.Nil(t, err.GenericErr)
		})
	}
}

func TestQueryRecursionLimitEnforcedInHandlesAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), newCodeHash), true, testContract.IsCosmWasmV1After, 10*defaultGasForTests, 0)

			require.NotEmpty(t, data)
			require.Equal(t, string(data), `"Recursion limit was correctly enforced"`)

			require.Nil(t, err.GenericErr)
		})
	}
}

func TestQueryRecursionLimitEnforcedInHandlesDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), codeHash), true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			require.NotEmpty(t, res.Data)
			require.Equal(t, string(res.Data), `"Recursion limit was correctly enforced"`)
		})
	}
}

func TestQueryRecursionLimitEnforcedInInitsAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			// Initialize a contract that we will be querying
			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			// Initialize the contract that will be running the test
			_, _, addr, events, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, fmt.Sprintf(`{"send_external_query_recursion_limit":{"to":"%s","code_hash":"%s", "depth":1}}`, addr.String(), newCodeHash), true, testContract.IsCosmWasmV1Before, 10*defaultGasForTests)
			require.Empty(t, err)

			require.Nil(t, err.GenericErr)

			requireEvents(t, []ContractEvent{
				{
					{Key: "contract_address", Value: addr.String()},
					{Key: "message", Value: "Recursion limit was correctly enforced"},
				},
			}, events)
		})
	}
}

func TestWriteToStorageDuringQueryAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, queryErr := queryHelper(t, keeper, ctx, addr, `{"write_to_storage": {}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.NotEmpty(t, queryErr)
			require.Contains(t, queryErr.Error(), "contract tried to write to storage during a query")
		})
	}
}

func TestRemoveFromStorageDuringQueryAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, initErr)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, queryErr := queryHelper(t, keeper, ctx, addr, `{"remove_from_storage": {}}`, false, testContract.IsCosmWasmV1After, defaultGasForTests)
			require.NotNil(t, queryErr)
			require.Contains(t, queryErr.Error(), "contract tried to write to storage during a query")
		})
	}
}

func TestQueryGasPriceAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, _, gasUsed, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), newCodeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)
			// require that more gas was used than the base 20K (10K for execute, another 10K for query)
			require.Greater(t, gasUsed, uint64(20_000))
		})
	}
}

func TestQueryGasPriceDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			// require that more gas was used than the base 20K (10K for execute, another 10K for query)
			require.Greater(t, res.GasUsed, uint64(20_000))
		})
	}
}

func TestCodeHashExecCallQueryAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, _, events, _, err := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), newCodeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: addr.String()},
							{Key: "c", Value: "2"},
						},
					},
					events,
				)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), newCodeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				if testContract.IsCosmWasmV1After {
					require.Contains(t,
						err.Error(),
						"Expected to parse either a `true`, `false`, or a `null`",
					)
				} else {
					require.Contains(t,
						err.Error(),
						"Expected to parse either a `true`, `false`, or a `null`.",
					)
				}
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), newCodeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, _, err = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestCodeHashExecCallQueryDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			t.Run("GoodCodeHash", func(t *testing.T) {
				res, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, math.MaxUint64)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: addr.String()},
							{Key: "c", Value: "2"},
						},
					},
					res.WasmEvents,
				)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, math.MaxUint64)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "failed to validate transaction")
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, math.MaxUint64)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "failed to validate transaction")
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, math.MaxUint64)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, err := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, math.MaxUint64)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestCodeHashQueryCallQueryAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, newCodeHash := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, addr, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			t.Run("GoodCodeHash", func(t *testing.T) {
				output, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), newCodeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.Empty(t, err)
				require.Equal(t, "2", output)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "failed to validate transaction")
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), newCodeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				if testContract.IsCosmWasmV1After {
					require.Contains(t,
						err.Error(),
						"Expected to parse either a `true`, `false`, or a `null`",
					)
				} else {
					require.Contains(t,
						err.Error(),
						"Expected to parse either a `true`, `false`, or a `null`.",
					)
				}
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), newCodeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "failed to validate transaction")
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, err := queryHelper(t, keeper, ctx, addr, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1After, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t, err.Error(), "failed to validate transaction")
			})
		})
	}
}

func TestV1EndpointsSanityAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment":{"addition": 13}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(23), binary.BigEndian.Uint32(data))

			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, qErr)

			// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(23), resp.Get.Count)
		})
	}
}

func TestV1EndpointsSanityDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"increment":{"addition": 13}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			require.Equal(t, uint32(23), binary.BigEndian.Uint32(res.Data))

			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, qErr)

			// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(23), resp.Get.Count)
		})
	}
}

func TestLastMsgMarkerAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, err := execHelperMultipleMsgs(t, keeper, ctx, contractAddress, walletA, privKeyA, []string{`{"last_msg_marker":{}}`}, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)
			require.Empty(t, err)

			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, qErr)

			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)

			require.Equal(t, uint32(14), resp.Get.Count)
		})
	}
}

func TestLastMsgMarkerWithMoreThanOneTxAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"last_msg_marker":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment_times":{"times": 5}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)
			require.Empty(t, err)

			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, qErr)

			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)

			require.Equal(t, uint32(19), resp.Get.Count)
		})
	}
}

func TestV1QueryWorksWithEnvAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":0}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 10)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, qErr)

			// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(0), resp.Get.Count)
		})
	}
}

func TestV1ReplySanityAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"increment":{"addition": 13}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(23), binary.BigEndian.Uint32(data))

			_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"transfer_money":{"amount": 10213}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(23), binary.BigEndian.Uint32(data))

			_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"recursive_reply":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(25), binary.BigEndian.Uint32(data))

			_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"recursive_reply_fail":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(10), binary.BigEndian.Uint32(data))

			_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"init_new_contract":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(150), binary.BigEndian.Uint32(data))

			_, _, data, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"init_new_contract_with_error":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(1337), binary.BigEndian.Uint32(data))

			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, qErr)

			// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(1337), resp.Get.Count)
		})
	}
}

func TestV1ReplySanityDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)

			res, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"increment":{"addition": 13}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, err)
			require.Equal(t, uint32(23), binary.BigEndian.Uint32(res.Data))

			res, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"transfer_money":{"amount": 10213}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, err)
			require.Equal(t, uint32(23), binary.BigEndian.Uint32(res.Data))

			res, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"recursive_reply":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, err)
			require.Equal(t, uint32(25), binary.BigEndian.Uint32(res.Data))

			res, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"recursive_reply_fail":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, err)
			require.Equal(t, uint32(10), binary.BigEndian.Uint32(res.Data))

			res, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"init_new_contract":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, err)
			require.Equal(t, uint32(150), binary.BigEndian.Uint32(res.Data))

			res, err = migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"init_new_contract_with_error":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, err)
			require.Equal(t, uint32(1337), binary.BigEndian.Uint32(res.Data))

			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, qErr)

			// assert result is 32 byte sha256 hash (if hashed), or contractAddr if not
			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(1337), resp.Get.Count)
		})
	}
}

func TestV1QueryV010ContractAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
			require.NoError(t, err)

			v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
			require.NoError(t, err)

			codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
			require.NoError(t, err)
			v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, walletA, privKeyA, `{"init_from_v1":{"counter":190}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			msg := fmt.Sprintf(`{"query_v10":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, msg, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(190), binary.BigEndian.Uint32(data))
		})
	}
}

func TestV1QueryV010ContractDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			wasmCode, err := os.ReadFile(TestContractPaths[v010Contract])
			require.NoError(t, err)

			v010CodeID, err := keeper.Create(ctx, walletA, wasmCode, "", "")
			require.NoError(t, err)

			codeInfo, err := keeper.GetCodeInfo(ctx, v010CodeID)
			require.NoError(t, err)
			v010CodeHash := hex.EncodeToString(codeInfo.CodeHash)

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			_, _, v010ContractAddress, _, err := initHelper(t, keeper, ctx, v010CodeID, walletA, walletA, privKeyA, `{"init_from_v1":{"counter":190}}`, true, false, defaultGasForTests)
			require.Empty(t, err)

			msg := fmt.Sprintf(`{"query_v10":{"address":"%s", "code_hash":"%s"}}`, v010ContractAddress, v010CodeHash)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, msg, true, testContract.IsCosmWasmV1After, math.MaxUint64)

			require.Empty(t, migErr)
			require.Equal(t, uint32(190), binary.BigEndian.Uint32(res.Data))
		})
	}
}

func TestV1ReplyOnMultipleSubmessagesAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"multiple_sub_messages":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(102), binary.BigEndian.Uint32(data))
		})
	}
}

func TestV1ReplyOnMultipleSubmessagesDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"multiple_sub_messages":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			require.Equal(t, uint32(102), binary.BigEndian.Uint32(res.Data))
		})
	}
}

func TestV1MultipleSubmessagesNoReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"multiple_sub_messages_no_reply":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64, 0)

			require.Empty(t, err)
			require.Equal(t, uint32(10), binary.BigEndian.Uint32(data))

			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, qErr)

			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(16), resp.Get.Count)
		})
	}
}

func TestV1MultipleSubmessagesNoReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After || !testContract.IsCosmWasmV1Before {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"multiple_sub_messages_no_reply":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			require.Empty(t, migErr)
			require.Equal(t, uint32(10), binary.BigEndian.Uint32(res.Data))

			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, qErr)

			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(16), resp.Get.Count)
		})
	}
}

func TestSendEncryptedAttributesFromExecuteWithoutSubmessageWithoutReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attributes":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
				},
				events,
			)
		})
	}
}

func TestSendEncryptedAttributesFromExecuteWithoutSubmessageWithoutReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_attributes":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestSendEncryptedAttributesFromExecuteWithSubmessageWithoutReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
				},
				events,
			)
		})
	}
}

func TestSendEncryptedAttributesFromExecuteWithSubmessageWithoutReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestV1SendsEncryptedAttributesFromExecuteWithSubmessageWithReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":2200}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr5", Value: "🤯"},
						{Key: "attr6", Value: "🦄"},
					},
				},
				events,
			)
		})
	}
}

func TestV1SendsEncryptedAttributesFromExecuteWithSubmessageWithReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_attributes_with_submessage":{"id":2200}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr5", Value: "🤯"},
						{Key: "attr6", Value: "🦄"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestSendPlaintextAttributesFromExecuteWithoutSubmessageWithoutReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0, true)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
				},
				events,
			)
		})
	}
}

func TestSendPlaintextAttributesFromExecuteWithoutSubmessageWithoutReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestSendPlaintextAttributesFromExecuteWithSubmessageWithoutReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0, true)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
				},
				events,
			)
		})
	}
}

func TestSendPlaintextAttributesFromExecuteWithSubmessageWithoutReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestV1SendsPlaintextAttributesFromExecuteWithSubmessageWithReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			_, _, _, events, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":2300}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0, true)
			require.Empty(t, err)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr5", Value: "🤯"},
						{Key: "attr6", Value: "🦄"},
					},
				},
				events,
			)
		})
	}
}

func TestV1SendsPlaintextAttributesFromExecuteWithSubmessageWithReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_plaintext_attributes_with_submessage":{"id":2300}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr1", Value: "🦄"},
						{Key: "attr2", Value: "🌈"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🍉"},
						{Key: "attr4", Value: "🥝"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr5", Value: "🤯"},
						{Key: "attr6", Value: "🦄"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestV1SendsEncryptedEventsFromExecuteWithoutSubmessageWithoutReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			nonce, ctx, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_events":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			events := ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "🐙"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)
		})
	}
}

func TestV1SendsEncryptedEventsFromExecuteWithoutSubmessageWithoutReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_events":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			events := res.Ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "🐙"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)
		})
	}
}

func TestV1SendsEncryptedEventsFromExecuteWithSubmessageWithoutReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			nonce, ctx, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_events_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			events := ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false
			hadCyber3 := false
			hadCyber4 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "🐙"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}

				if e.Type == "wasm-cyber3" {
					require.False(t, hadCyber3)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🤯"},
							{Key: "attr2", Value: "🤟"},
						},
						attrs,
					)

					hadCyber3 = true
				}

				if e.Type == "wasm-cyber4" {
					require.False(t, hadCyber4)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "😅"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber4 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)
			require.True(t, hadCyber3)
			require.True(t, hadCyber4)
		})
	}
}

func TestV1SendsEncryptedEventsFromExecuteWithSubmessageWithoutReplyuringDMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_events_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			events := res.Ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false
			hadCyber3 := false
			hadCyber4 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "🐙"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}

				if e.Type == "wasm-cyber3" {
					require.False(t, hadCyber3)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🤯"},
							{Key: "attr2", Value: "🤟"},
						},
						attrs,
					)

					hadCyber3 = true
				}

				if e.Type == "wasm-cyber4" {
					require.False(t, hadCyber4)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "😅"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber4 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)
			require.True(t, hadCyber3)
			require.True(t, hadCyber4)
		})
	}
}

func TestV1SendsEncryptedEventsFromExecuteWithSubmessageWithReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)
			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)
			nonce, ctx, _, _, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_events_with_submessage":{"id":2400}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			events := ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false
			hadCyber3 := false
			hadCyber4 := false
			hadCyber5 := false
			hadCyber6 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "🐙"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}

				if e.Type == "wasm-cyber3" {
					require.False(t, hadCyber3)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🤯"},
							{Key: "attr2", Value: "🤟"},
						},
						attrs,
					)

					hadCyber3 = true
				}

				if e.Type == "wasm-cyber4" {
					require.False(t, hadCyber4)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "😅"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber4 = true
				}

				if e.Type == "wasm-cyber5" {
					require.False(t, hadCyber5)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "😗"},
							{Key: "attr2", Value: "😋"},
						},
						attrs,
					)

					hadCyber5 = true
				}

				if e.Type == "wasm-cyber6" {
					require.False(t, hadCyber6)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "😉"},
							{Key: "attr4", Value: "😊"},
						},
						attrs,
					)

					hadCyber6 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)
			require.True(t, hadCyber3)
			require.True(t, hadCyber4)
			require.True(t, hadCyber5)
			require.True(t, hadCyber6)
		})
	}
}

func TestV1SendsEncryptedEventsFromExecuteWithSubmessageWithReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)
			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_events_with_submessage":{"id":2400}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			events := res.Ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false
			hadCyber3 := false
			hadCyber4 := false
			hadCyber5 := false
			hadCyber6 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "🐙"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}

				if e.Type == "wasm-cyber3" {
					require.False(t, hadCyber3)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🤯"},
							{Key: "attr2", Value: "🤟"},
						},
						attrs,
					)

					hadCyber3 = true
				}

				if e.Type == "wasm-cyber4" {
					require.False(t, hadCyber4)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "😅"},
							{Key: "attr4", Value: "🦄"},
						},
						attrs,
					)

					hadCyber4 = true
				}

				if e.Type == "wasm-cyber5" {
					require.False(t, hadCyber5)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "😗"},
							{Key: "attr2", Value: "😋"},
						},
						attrs,
					)

					hadCyber5 = true
				}

				if e.Type == "wasm-cyber6" {
					require.False(t, hadCyber6)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr3", Value: "😉"},
							{Key: "attr4", Value: "😊"},
						},
						attrs,
					)

					hadCyber6 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)
			require.True(t, hadCyber3)
			require.True(t, hadCyber4)
			require.True(t, hadCyber5)
			require.True(t, hadCyber6)
		})
	}
}

func TestV1SendsMixedAttributesAndEventsFromExecuteWithoutSubmessageWithoutReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)
			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)
			nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_mixed_attributes_and_events":{}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0, true)
			require.Empty(t, err)

			events := ctx.EventManager().Events()

			hadCyber1 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, false)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
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
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🐙"},
						{Key: "attr4", Value: "🦄"},
					},
				},
				logs,
			)
		})
	}
}

func TestV1SendsMixedAttributesAndEventsFromExecuteWithoutSubmessageWithoutReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)
			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_mixed_attributes_and_events":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			events := res.Ctx.EventManager().Events()

			hadCyber1 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, false)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
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
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🐙"},
						{Key: "attr4", Value: "🦄"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestV1SendsMixedAttributesAndEventsFromExecuteWithSubmessageWithoutReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)
			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)
			nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			events := ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr5", Value: "🐙"},
							{Key: "attr6", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🐙"},
						{Key: "attr4", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr7", Value: "😅"},
						{Key: "attr8", Value: "🦄"},
					},
				},
				logs,
			)
		})
	}
}

func TestV1SendsMixedAttributesAndEventsFromExecuteWithSubmessageWithoutReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)
			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":0}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			events := res.Ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr5", Value: "🐙"},
							{Key: "attr6", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🐙"},
						{Key: "attr4", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr7", Value: "😅"},
						{Key: "attr8", Value: "🦄"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestV1SendsMixedAttributesAndEventsFromExecuteWithSubmessageWithReplyAfterMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)
			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)
			nonce, ctx, _, logs, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":2500}}`, true, testContract.IsCosmWasmV1After, defaultGasForTests, 0)
			require.Empty(t, err)

			events := ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false
			hadCyber3 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr5", Value: "🐙"},
							{Key: "attr6", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}

				if e.Type == "wasm-cyber3" {
					require.False(t, hadCyber3)
					attrs, err := parseAndDecryptAttributes(e.Attributes, nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr9", Value: "🤯"},
							{Key: "attr10", Value: "🤟"},
						},
						attrs,
					)

					hadCyber3 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)
			require.True(t, hadCyber3)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🐙"},
						{Key: "attr4", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr7", Value: "😅"},
						{Key: "attr8", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr11", Value: "😉"},
						{Key: "attr12", Value: "😊"},
					},
				},
				logs,
			)
		})
	}
}

func TestV1SendsMixedAttributesAndEventsFromExecuteWithSubmessageWithReplyDuringMigrate(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		if !testContract.IsCosmWasmV1After {
			continue
		}
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)
			newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
			res, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"add_mixed_attributes_and_events_with_submessage":{"id":2500}}`, true, testContract.IsCosmWasmV1After, math.MaxUint64)
			require.Empty(t, migErr)

			events := res.Ctx.EventManager().Events()

			hadCyber1 := false
			hadCyber2 := false
			hadCyber3 := false

			for _, e := range events {
				if e.Type == "wasm-cyber1" {
					require.False(t, hadCyber1)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr1", Value: "🦄"},
							{Key: "attr2", Value: "🌈"},
						},
						attrs,
					)

					hadCyber1 = true
				}

				if e.Type == "wasm-cyber2" {
					require.False(t, hadCyber2)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr5", Value: "🐙"},
							{Key: "attr6", Value: "🦄"},
						},
						attrs,
					)

					hadCyber2 = true
				}

				if e.Type == "wasm-cyber3" {
					require.False(t, hadCyber3)
					attrs, err := parseAndDecryptAttributes(e.Attributes, res.Nonce, true)
					require.Empty(t, err)

					requireLogAttributes(t,
						[]v010types.LogAttribute{
							{Key: "contract_address", Value: contractAddress.String()},
							{Key: "attr9", Value: "🤯"},
							{Key: "attr10", Value: "🤟"},
						},
						attrs,
					)

					hadCyber3 = true
				}
			}

			require.True(t, hadCyber1)
			require.True(t, hadCyber2)
			require.True(t, hadCyber3)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr3", Value: "🐙"},
						{Key: "attr4", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr7", Value: "😅"},
						{Key: "attr8", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "attr11", Value: "😉"},
						{Key: "attr12", Value: "😊"},
					},
				},
				res.WasmEvents,
			)
		})
	}
}

func TestCannotChangeAdminIfNotAdmin(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, walletB, privKeyB := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			t.Run("update: not admin", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletB, privKeyB, walletA, defaultGasForTests)

				require.Equal(t, updateErr.Error(), "caller is not the admin: unauthorized")
			})

			t.Run("update: null admin", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, "")

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletB, privKeyB, walletA, defaultGasForTests)

				require.Equal(t, updateErr.Error(), "caller is not the admin: unauthorized")
			})

			t.Run("clear: not admin", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletB, privKeyB, nil, defaultGasForTests)

				require.Equal(t, updateErr.Error(), "caller is not the admin: unauthorized")
			})

			t.Run("clear: null admin", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, "")

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletB, privKeyB, nil, defaultGasForTests)

				require.Equal(t, updateErr.Error(), "caller is not the admin: unauthorized")
			})
		})
	}
}

func TestAdminCanChangeAdmin(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			t.Run("update", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, walletB, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletB.String())
			})

			t.Run("clear", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, nil, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, "")
			})
		})
	}
}

func TestFailMigrateAndChangeAfterClearAdmin(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			info := keeper.GetContractInfo(ctx, contractAddress)
			require.Equal(t, info.Admin, walletA.String())

			_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, nil, defaultGasForTests)
			require.Nil(t, updateErr)

			info = keeper.GetContractInfo(ctx, contractAddress)
			require.Equal(t, info.Admin, "")

			t.Run("migrate", func(t *testing.T) {
				newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
				_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1After, math.MaxUint64, 0)
				require.Contains(t, migErr.Error(), "requires migrate from admin: migrate contract failed")
			})

			t.Run("update", func(t *testing.T) {
				_, updateErr = updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, walletB, defaultGasForTests)
				require.Equal(t, updateErr.Error(), "caller is not the admin: unauthorized")
			})

			t.Run("clear", func(t *testing.T) {
				_, updateErr = updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, nil, defaultGasForTests)
				require.Equal(t, updateErr.Error(), "caller is not the admin: unauthorized")
			})
		})
	}
}

func TestFailMigrateAndChangeAdminByOldAdmin(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
			require.Empty(t, err)

			info := keeper.GetContractInfo(ctx, contractAddress)
			require.Equal(t, info.Admin, walletA.String())

			_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, walletB, defaultGasForTests)
			require.Nil(t, updateErr)

			info = keeper.GetContractInfo(ctx, contractAddress)
			require.Equal(t, info.Admin, walletB.String())

			t.Run("migrate", func(t *testing.T) {
				newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
				_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1After, math.MaxUint64, 0)
				require.Contains(t, migErr.Error(), "requires migrate from admin: migrate contract failed")
			})

			t.Run("update", func(t *testing.T) {
				_, updateErr = updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, walletB, defaultGasForTests)
				require.Equal(t, updateErr.Error(), "caller is not the admin: unauthorized")
			})

			t.Run("clear", func(t *testing.T) {
				_, updateErr = updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, nil, defaultGasForTests)
				require.Equal(t, updateErr.Error(), "caller is not the admin: unauthorized")
			})
		})
	}
}

func TestOldAdminCanChangeAdminByPassingOldProof(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			t.Run("update", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				oldAdminProof := info.AdminProof

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, walletB, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletB.String())

				_, updateErr = fakeUpdateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, walletA, defaultGasForTests, walletA, oldAdminProof)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())
			})

			t.Run("clear", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				oldAdminProof := info.AdminProof

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, walletB, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletB.String())

				_, updateErr = fakeUpdateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, nil, defaultGasForTests, walletA, oldAdminProof)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, "")
			})

			t.Run("update after clear", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				oldAdminProof := info.AdminProof

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, nil, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, "")

				_, updateErr = fakeUpdateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, walletA, defaultGasForTests, walletA, oldAdminProof)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())
			})

			t.Run("clear after clear", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				oldAdminProof := info.AdminProof

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, nil, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, "")

				_, updateErr = fakeUpdateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, nil, defaultGasForTests, walletA, oldAdminProof)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, "")
			})
		})
	}
}

func TestOldAdminCanMigrateChangeAdminByPassingOldProof(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			t.Run("migrate", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				oldAdminProof := info.AdminProof

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, walletB, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletB.String())

				newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
				_, migErr := fakeMigrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1After, math.MaxUint64, walletA, oldAdminProof)
				require.Empty(t, migErr)

				// admin is still walletB
				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletB.String())

				// but code is migrated
				history := keeper.GetContractHistory(ctx, contractAddress)
				require.Len(t, history, 2)
				require.Equal(t, history[0].CodeID, codeID)
				require.Equal(t, history[1].CodeID, newCodeId)
			})

			t.Run("migrate after clear", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				oldAdminProof := info.AdminProof

				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, nil, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, "")

				newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
				_, migErr := fakeMigrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1After, math.MaxUint64, walletA, oldAdminProof)
				require.Empty(t, migErr)

				// admin is still nil
				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, "")

				// but code is migrated
				history := keeper.GetContractHistory(ctx, contractAddress)
				require.Len(t, history, 2)
				require.Equal(t, history[0].CodeID, codeID)
				require.Equal(t, history[1].CodeID, newCodeId)
			})
		})
	}
}

func TestEnclaveFailsAdminIsNotSender(t *testing.T) {
	for _, testContract := range migrateTestContracts {
		t.Run(testContract.CosmWasmVersionBefore+"->"+testContract.CosmWasmVersionAfter, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privkeyA, walletB, privkeyB := setupTest(t, testContract.WasmFilePathBefore, sdk.NewCoins())

			t.Run("migrate fails msg verify params", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletB, privkeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				// B is the admin
				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletB.String())

				bAdminProof := info.AdminProof

				// now A is the admin
				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletB, privkeyB, walletA, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				// A is the admin but B is the sender
				// B passes old B's proof which is valid and should pass the proof check
				// however the sender==admin check later on should fail
				newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
				_, migErr := fakeMigrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletB, privkeyB, `{"nop":{}}`, false, testContract.IsCosmWasmV1After, math.MaxUint64, walletA, bAdminProof)
				require.Contains(t, migErr.Error(), "Enclave: failed to verify transaction signature: migrate contract failed")
			})

			t.Run("migrate fails admin proof check", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privkeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				// A is the admin but B is the sender
				// B passes A's proof
				newCodeId, _ := uploadCode(ctx, t, keeper, testContract.WasmFilePathAfter, walletA)
				_, migErr := fakeMigrateHelper(t, keeper, ctx, newCodeId, contractAddress, walletB, privkeyB, `{"nop":{}}`, false, testContract.IsCosmWasmV1After, math.MaxUint64, walletA, info.AdminProof)
				require.Contains(t, migErr.Error(), "Enclave: failed to validate transaction: migrate contract failed")
			})

			t.Run("change fails msg verify params", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletB, privkeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				// B is the admin
				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletB.String())

				bAdminProof := info.AdminProof

				// now A is the admin
				_, updateErr := updateAdminHelper(t, keeper, ctx, contractAddress, walletB, privkeyB, walletA, defaultGasForTests)
				require.Nil(t, updateErr)

				info = keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				// A is the admin but B is the sender
				// B passes old B's proof which is valid and should pass the proof check
				// however the sender==admin check later on should fail
				t.Run("update", func(t *testing.T) {
					_, updateErr = fakeUpdateAdminHelper(t, keeper, ctx, contractAddress, walletB, privkeyB, walletB, math.MaxUint64, walletA, bAdminProof)
					require.Contains(t, updateErr.Error(), "Enclave: failed to verify transaction signature")
				})
				t.Run("clear", func(t *testing.T) {
					_, updateErr = fakeUpdateAdminHelper(t, keeper, ctx, contractAddress, walletB, privkeyB, nil, math.MaxUint64, walletA, bAdminProof)
					require.Contains(t, updateErr.Error(), "Enclave: failed to verify transaction signature")
				})
			})

			t.Run("change fails admin proof check", func(t *testing.T) {
				_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privkeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1Before, defaultGasForTests)
				require.Empty(t, err)

				info := keeper.GetContractInfo(ctx, contractAddress)
				require.Equal(t, info.Admin, walletA.String())

				// A is the admin but B is the sender
				// B passes A's proof
				t.Run("update", func(t *testing.T) {
					_, updateErr := fakeUpdateAdminHelper(t, keeper, ctx, contractAddress, walletB, privkeyB, walletB, math.MaxUint64, walletA, info.AdminProof)
					require.Contains(t, updateErr.Error(), "Enclave: failed to validate transaction")
				})
				t.Run("clear", func(t *testing.T) {
					_, updateErr := fakeUpdateAdminHelper(t, keeper, ctx, contractAddress, walletB, privkeyB, nil, math.MaxUint64, walletA, info.AdminProof)
					require.Contains(t, updateErr.Error(), "Enclave: failed to validate transaction")
				})
			})
		})
	}
}

func TestContractIsAdminOfAnotherContractMigrateFromExec(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractB.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(`{"nop":{}}`))), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	history := keeper.GetContractHistory(ctx, contractB)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestContractIsAdminOfAnotherContractMigrateFromReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"reply":true,"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractB.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(`{"nop":{}}`))), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	history := keeper.GetContractHistory(ctx, contractB)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestContractIsAdminOfAnotherContractMigrateFromMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1MigratedContract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, execErr := migrateHelper(t, keeper, ctx, newCodeId, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractB.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(`{"nop":{}}`))), true, true, math.MaxUint64)
	require.Empty(t, execErr)

	history := keeper.GetContractHistory(ctx, contractB)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)

	history = keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestContractIsAdminOfAnotherContractUpdateAdminFromExec(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_update_admin":{"contract_addr":"%s","new_admin":"%s"}}`, contractB.String(), walletA.String()), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, walletA.String())
}

func TestContractIsAdminOfAnotherContractUpdateAdminFromReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_update_admin":{"contract_addr":"%s","new_admin":"%s","reply":true}}`, contractB.String(), walletA.String()), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, walletA.String())
}

func TestContractIsAdminOfAnotherContractUpdateAdminFromMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1MigratedContract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, execErr := migrateHelper(t, keeper, ctx, newCodeId, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_update_admin":{"contract_addr":"%s","new_admin":"%s"}}`, contractB.String(), walletA.String()), true, true, math.MaxUint64)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, walletA.String())

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestContractIsAdminOfAnotherContractClearAdminFromExec(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_clear_admin":{"contract_addr":"%s"}}`, contractB.String()), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, "")
}

func TestContractIsAdminOfAnotherContractClearAdminFromReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_clear_admin":{"contract_addr":"%s","reply":true}}`, contractB.String()), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, "")
}

func TestContractIsAdminOfAnotherContractClearAdminFromMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1MigratedContract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, execErr := migrateHelper(t, keeper, ctx, newCodeId, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_clear_admin":{"contract_addr":"%s"}}`, contractB.String()), true, true, math.MaxUint64)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, "")

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestContractFailsToMigrateAnotherContractBecauseNotAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, walletA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractB.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(`{"nop":{}}`))), false, true, math.MaxUint64, 0)
	require.NotEmpty(t, execErr)
	require.Contains(t, execErr.Error(), "requires migrate from admin: migrate contract failed")

	history := keeper.GetContractHistory(ctx, contractB)
	require.Len(t, history, 1)
	require.Equal(t, history[0].CodeID, codeID)
}

func TestContractFailsToMigrateAnotherContractBecauseStdError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, contractA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractB.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(`{"std_error":{}}`))), true, true, math.MaxUint64, 0)
	require.NotEmpty(t, execErr)
	require.Contains(t, execErr.Error(), "encrypted: Generic error: this is an std error")

	history := keeper.GetContractHistory(ctx, contractB)
	require.Len(t, history, 1)
	require.Equal(t, history[0].CodeID, codeID)
}

func TestContractFailsToUpdateAdminOfAnotherContractBecauseNotAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, walletA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_update_admin":{"contract_addr":"%s","new_admin":"%s"}}`, contractB.String(), contractB.String()), false, true, math.MaxUint64, 0)
	require.NotEmpty(t, execErr)
	require.Equal(t, execErr.Error(), "encrypted: dispatch: submessages: caller is not the admin: unauthorized")

	info = keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, walletA.String())
}

func TestContractFailsToClearAdminOfAnotherContractBecauseNotAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, contractB, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, walletA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_clear_admin":{"contract_addr":"%s"}}`, contractB.String()), false, true, math.MaxUint64, 0)
	require.NotEmpty(t, execErr)
	require.Equal(t, execErr.Error(), "encrypted: dispatch: submessages: caller is not the admin: unauthorized")

	info = keeper.GetContractInfo(ctx, contractB)
	require.Equal(t, info.Admin, walletA.String())
}

func TestContractIsAdminOfItselfMigrateFromExec(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractA.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(`{"nop":{}}`))), false, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestContractIsAdminOfItselfMigrateFromReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"reply":true,"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractA.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(`{"nop":{}}`))), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestContractIsAdminOfItselfMigrateFromMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1MigratedContract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	newCodeId2, newCodeHash2 := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractA.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractA.String(), newCodeId2, newCodeHash2, base64.RawStdEncoding.EncodeToString([]byte(`{"nop":{}}`)))))), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 3)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[0].Operation, types.ContractCodeHistoryOperationTypeInit)
	require.Equal(t, history[1].CodeID, newCodeId)
	require.Equal(t, history[1].Operation, types.ContractCodeHistoryOperationTypeMigrate)
	require.Equal(t, history[2].CodeID, newCodeId2)
	require.Equal(t, history[2].Operation, types.ContractCodeHistoryOperationTypeMigrate)
}

func TestContractIsAdminOfItselfUpdateAdminFromExec(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, contractA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_update_admin":{"contract_addr":"%s","new_admin":"%s"}}`, contractA.String(), walletA.String()), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletA.String())
}

func TestContractIsAdminOfItselfUpdateAdminFromReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, contractA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_update_admin":{"contract_addr":"%s","new_admin":"%s","reply":true}}`, contractA.String(), walletA.String()), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletA.String())
}

func TestContractIsAdminOfItselfUpdateAdminFromMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1MigratedContract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractA.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"send_msg_update_admin":{"contract_addr":"%s","new_admin":"%s"}}`, contractA.String(), walletA.String())))), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletA.String())

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestContractIsAdminOfItselfClearAdminFromExec(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, contractA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_clear_admin":{"contract_addr":"%s"}}`, contractA.String()), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, "")
}

func TestContractIsAdminOfItselfClearAdminFromReply(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, contractA.String())

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_clear_admin":{"contract_addr":"%s","reply":true}}`, contractA.String()), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, "")
}

func TestContractIsAdminOfItselfClearAdminFromMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1MigratedContract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, contractA.String())

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractA.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"send_msg_clear_admin":{"contract_addr":"%s"}}`, contractA.String())))), true, true, math.MaxUint64, 0)
	require.Empty(t, execErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, "")

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestContractFailsToMigrateItselfBecauseNotAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractA.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(`{"nop":{}}`))), false, true, math.MaxUint64, 0)
	require.NotEmpty(t, execErr)
	require.Contains(t, execErr.Error(), "requires migrate from admin: migrate contract failed")

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 1)
	require.Equal(t, history[0].CodeID, codeID)
}

func TestContractFailsToMigrateItselfBecauseStdError(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, contractA, defaultGasForTests)
	require.Empty(t, updateErr)

	newCodeId, newCodeHash := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_migrate_contract":{"contract_addr":"%s","new_code_id":"%d","callback_code_hash":"%s","msg":"%s"}}`, contractA.String(), newCodeId, newCodeHash, base64.RawStdEncoding.EncodeToString([]byte(`{"std_error":{}}`))), true, true, math.MaxUint64, 0)
	require.NotEmpty(t, execErr)
	require.Contains(t, execErr.Error(), "encrypted: Generic error: this is an std error")

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 1)
	require.Equal(t, history[0].CodeID, codeID)
}

func TestContractFailsToUpdateAdminOfItselfBecauseNotAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_update_admin":{"contract_addr":"%s","new_admin":"%s"}}`, contractA.String(), walletA.String()), false, true, math.MaxUint64, 0)
	require.NotEmpty(t, execErr)
	require.Equal(t, execErr.Error(), "encrypted: dispatch: submessages: caller is not the admin: unauthorized")

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletA.String())
}

func TestContractFailsToClearAdminOfItselfBecauseNotAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, contractA, walletA, privKeyA, fmt.Sprintf(`{"send_msg_clear_admin":{"contract_addr":"%s"}}`, contractA.String()), false, true, math.MaxUint64, 0)
	require.NotEmpty(t, execErr)
	require.Equal(t, execErr.Error(), "encrypted: dispatch: submessages: caller is not the admin: unauthorized")

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletA.String())
}

func TestMigrateAfterUpdateAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, privKeyB := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletA.String())

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, walletB, defaultGasForTests)
	require.Empty(t, updateErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletB.String())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, execErr := migrateHelper(t, keeper, ctx, newCodeId, contractA, walletB, privKeyB, `{"nop":{}}`, true, true, math.MaxUint64)
	require.Empty(t, execErr)

	history := keeper.GetContractHistory(ctx, contractA)
	require.Len(t, history, 2)
	require.Equal(t, history[0].CodeID, codeID)
	require.Equal(t, history[1].CodeID, newCodeId)
}

func TestUpdateAdminAfterUpdateAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, privKeyB := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletA.String())

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, walletB, defaultGasForTests)
	require.Empty(t, updateErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletB.String())

	_, updateErr = updateAdminHelper(t, keeper, ctx, contractA, walletB, privKeyB, walletA, defaultGasForTests)
	require.Empty(t, updateErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletA.String())
}

func TestClearAdminAfterUpdateAdmin(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, privKeyB := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletA.String())

	_, updateErr := updateAdminHelper(t, keeper, ctx, contractA, walletA, privKeyA, walletB, defaultGasForTests)
	require.Empty(t, updateErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, walletB.String())

	_, updateErr = updateAdminHelper(t, keeper, ctx, contractA, walletB, privKeyB, nil, defaultGasForTests)
	require.Empty(t, updateErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.Admin, "")
}

func TestContractBecmesIbcEnabledAfterMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"nop":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.IBCPortID, "")

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[ibcContract], walletA)
	_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractA, walletA, privKeyA, `{}`, true, true, math.MaxUint64)
	require.Empty(t, migErr)

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.IBCPortID, "wasm."+contractA.String())
}

func TestContractNoLongerIbcEnabledAfterMigrate(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[ibcContract], sdk.NewCoins())

	_, _, contractA, _, err := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"init":{}}`, true, true, defaultGasForTests)
	require.Empty(t, err)

	info := keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.IBCPortID, "wasm."+contractA.String())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[v1MigratedContract], walletA)
	_, migErr := migrateHelper(t, keeper, ctx, newCodeId, contractA, walletA, privKeyA, `{"nop":{}}`, false, true, math.MaxUint64, 0)
	require.NotEmpty(t, migErr)
	require.Contains(t, migErr.Error(), "requires ibc callbacks: migrate contract failed")

	info = keeper.GetContractInfo(ctx, contractA)
	require.Equal(t, info.IBCPortID, "wasm."+contractA.String())
}
