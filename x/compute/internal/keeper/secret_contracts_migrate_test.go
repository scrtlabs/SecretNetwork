package keeper

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestMigrateContract(t *testing.T) {
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

func TestMigrateWithStorage(t *testing.T) {
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

func TestMigrateContractFromDifferentAccount(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, WalletB, privKeyB := setupTest(t, TestContractPaths[migrateContractV1], sdk.NewCoins())

	newCodeId, _ := uploadCode(ctx, t, keeper, TestContractPaths[migrateContractV2], walletA)

	_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, walletA, privKeyA, `{"Nop":{}}`, true, true, defaultGasForTests)

	_, _, data, _, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"test":{}}`, true, true, defaultGasForTests, 0)
	require.Empty(t, execErr)
	require.Empty(t, data)

	_, err := migrateHelper(t, keeper, ctx, newCodeId, contractAddress, WalletB, privKeyB, `{"migrate":{}}`, false, true, math.MaxUint64)
	require.NotNil(t, err)
}
