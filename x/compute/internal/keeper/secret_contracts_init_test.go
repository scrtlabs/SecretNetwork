package keeper

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	v010cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v010"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

func TestInitLogs(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))
			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "init", Value: "🌈"},
					},
				},
				initEvents,
			)
		})
	}
}

func TestInitContractError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			t.Run("generic_err", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"generic_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotNil(t, err.GenericErr)
				require.Contains(t, err.GenericErr.Msg, "la la 🤯")
			})
			t.Run("invalid_base64", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_base64"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ra ra 🤯")
				} else {
					require.NotNil(t, err.InvalidBase64)
					require.Equal(t, "ra ra 🤯", err.InvalidBase64.Msg)
				}
			})
			t.Run("invalid_utf8", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"invalid_utf8"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "ka ka 🤯")
				} else {
					require.NotNil(t, err.InvalidUtf8)
					require.Equal(t, "ka ka 🤯", err.InvalidUtf8.Msg)
				}
			})
			t.Run("not_found", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"not_found"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "za za 🤯")
				} else {
					require.NotNil(t, err.NotFound)
					require.Equal(t, "za za 🤯", err.NotFound.Kind)
				}
			})
			t.Run("parse_err", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"parse_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

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
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"serialize_err"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

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
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"unauthorized"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

				if testContract.IsCosmWasmV1 {
					// Not supported in V1
					require.NotNil(t, err.GenericErr)
					require.Contains(t, err.GenericErr.Msg, "catch-all 🤯")
				} else {
					require.NotNil(t, err.Unauthorized)
				}
			})
			t.Run("underflow", func(t *testing.T) {
				_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"contract_error":{"error_type":"underflow"}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

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

func TestInitParamError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			codeHash := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
			msg := fmt.Sprintf(`{"callback":{"contract_addr":"notanaddress", "code_hash":"%s"}}`, codeHash)

			_, _, _, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, msg, false, testContract.IsCosmWasmV1, defaultGasForTests)

			require.Contains(t, initErr.Error(), "invalid address")
		})
	}
}

func TestInitNotEncryptedInputError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKey, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			//ctx = sdk.NewContext(
			//	ctx.MultiStore(),
			//	ctx.BlockHeader(),
			//	ctx.IsCheckTx(),
			//	log.NewNopLogger(),
			//).WithGasMeter(sdk.NewGasMeter(defaultGas))

			initMsg := []byte(`{"nop":{}`)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privKey, initMsg, codeID, nil)

			// init
			_, _, err := keeper.Instantiate(ctx, codeID, walletA, nil, initMsg, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)

			require.Contains(t, err.Error(), "failed to decrypt data")
		})
	}
}

func TestQueryNotEncryptedInputError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, err := keeper.QuerySmart(ctx, contractAddress, []byte(`{"owner":{}}`), false)
			require.Error(t, err)

			require.Contains(t, err.Error(), "failed to decrypt data")
		})
	}
}

func TestInitNoLogs(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init
			_, _, _, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"no_logs":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			require.Empty(t, initErr)
			////require.Empty(t, initEvents)
		})
	}
}

func TestInitPanic(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, _, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"panic":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests)

			require.NotNil(t, initErr.GenericErr)
			require.Contains(t, initErr.GenericErr.Msg, "the contract panicked")
		})
	}
}

func TestRemoveKeyInCache(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

	_, _, _, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"test_remove_db":{}}`, true, true, defaultGasForTests)

	require.Nil(t, initErr.GenericErr)
}

func TestGasIsChargedForInitCallbackToInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d,"code_hash":"%s"}}`, codeID, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())
			require.Empty(t, err)
		})
	}
}

func TestGasIsChargedForInitCallbackToExec(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback":{"contract_addr":"%s","code_hash":"%s"}}`, addr, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())
			require.Empty(t, err)
		})
	}
}

func TestInitCallbackBadParam(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init first
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, secondContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_bad_param":{"contract_addr":"%s"}}`, contractAddress), true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, secondContractAddress)
			// require.Empty(t, initEvents)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, initErr.GenericErr)
				require.Contains(t, initErr.GenericErr.Msg, "v1_sanity_contract::msg::InstantiateMsg")
				require.Contains(t, initErr.GenericErr.Msg, "unknown variant `callback_contract_bad_param`")
			} else {
				require.NotNil(t, initErr.ParseErr)
				require.Equal(t, "test_contract::contract::InitMsg", initErr.ParseErr.Target)
				require.Contains(t, initErr.ParseErr.Msg, "unknown variant `callback_contract_bad_param`")
			}
		})
	}
}

func TestInitCallbackContractError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)
			require.Equal(t, 1, len(initEvents))

			_, _, secondContractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_contract_error":{"contract_addr":"%s", "code_hash":"%s"}}`, contractAddress, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests)

			require.NotNil(t, initErr.GenericErr)
			require.Contains(t, initErr.GenericErr.Msg, "la la 🤯")
			require.Empty(t, secondContractAddress)
			// require.Empty(t, initEvents)
		})
	}
}

func TestContractSendFundsToInitCallbackNotEnough(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_init_callback":{"code_id":%d,"denom":"%s","amount":%d,"code_hash":"%s"}}`, codeID, "denom", 18, codeHash), false, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			// require.Empty(t, execEvents)

			require.NotNil(t, execErr.GenericErr)
			require.Contains(t, execErr.GenericErr.Msg, "insufficient funds")

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			// The state here should have been reverted by the APP but in go-tests we create our own keeper
			// so it is not reverted in this case.
			require.Equal(t, "17denom", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
		})
	}
}

func TestContractSendFundsToInitCallback(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			require.Equal(t, "", contractCoinsBefore.String())
			require.Equal(t, "200000denom", walletCoinsBefore.String())

			_, _, _, execEvents, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds_to_init_callback":{"code_id":%d,"denom":"%s","amount":%d,"code_hash":"%s"}}`, codeID, "denom", 17, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 17)

			require.Empty(t, execErr)
			require.NotEmpty(t, execEvents)

			contractCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, addr)
			walletCoinsAfter := keeper.bankKeeper.GetAllBalances(ctx, walletA)

			var newContractBech32 string
			for _, v := range execEvents[1] {
				if v.Key == "contract_address" {
					newContractBech32 = v.Value
					break
				}
			}
			require.NotEmpty(t, newContractBech32)

			newContract, err := sdk.AccAddressFromBech32(newContractBech32)
			require.NoError(t, err)
			newContractCoins := keeper.bankKeeper.GetAllBalances(ctx, newContract)

			require.Equal(t, "", contractCoinsAfter.String())
			require.Equal(t, "199983denom", walletCoinsAfter.String())
			require.Equal(t, "17denom", newContractCoins.String())
		})
	}
}

func TestInitCallbackToInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			require.Equal(t, 2, len(initEvents))
			requireLogAttributes(t,
				ContractEvent{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "instantiating a new contract from init!", Value: "🐙"},
				},
				initEvents[0],
			)

			require.Contains(t,
				initEvents[1],
				v010cosmwasm.LogAttribute{Key: "init", Value: "🌈"},
			)
			var secondContractAddressBech32 string
			for _, v := range initEvents[1] {
				if v.Key == "contract_address" {
					secondContractAddressBech32 = v.Value
					break
				}
			}
			require.NotEmpty(t, secondContractAddressBech32)
			secondContractAddress, err := sdk.AccAddressFromBech32(secondContractAddressBech32)
			require.NoError(t, err)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			// require.Empty(t, execEvents)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestExecCallbackToInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init first contract
			_, _, contractAddress, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// init second contract and callback to the first contract
			_, _, execData, execEvents, _, execErr := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d, "code_hash":"%s"}}`, codeID, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			require.Empty(t, execErr)
			require.Empty(t, execData)

			require.Equal(t, 2, len(execEvents))
			requireLogAttributes(t,
				ContractEvent{
					{Key: "contract_address", Value: contractAddress.String()},
					{Key: "instantiating a new contract", Value: "🪂"},
				},
				execEvents[0],
			)
			require.Contains(t,
				execEvents[1],
				v010cosmwasm.LogAttribute{Key: "init", Value: "🌈"},
			)
			var secondContractAddressBech32 string
			for _, v := range execEvents[1] {
				if v.Key == "contract_address" {
					secondContractAddressBech32 = v.Value
					break
				}
			}
			require.NotEmpty(t, secondContractAddressBech32)
			secondContractAddress, err := sdk.AccAddressFromBech32(secondContractAddressBech32)
			require.NoError(t, err)

			_, _, data, _, _, err := execHelper(t, keeper, ctx, secondContractAddress, walletA, privKeyA, `{"unicode_data":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)

			require.Empty(t, err)
			// require.Empty(t, execEvents)
			require.Equal(t, "🍆🥑🍄", string(data))
		})
	}
}

func TestCallbackFromInitAndCallbackEvents(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			// init first contract so we'd have someone to callback
			_, _, firstContractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: firstContractAddress.String()},
						{Key: "init", Value: "🌈"},
					},
				},
				initEvents,
			)

			// init second contract and callback to the first contract
			_, _, contractAddress, initEvents, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"callback":{"contract_addr":"%s", "code_hash": "%s"}}`, firstContractAddress.String(), codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			requireEvents(t,
				[]ContractEvent{
					{
						{Key: "contract_address", Value: contractAddress.String()},
						{Key: "init with a callback", Value: "🦄"},
					},
					{
						{Key: "contract_address", Value: firstContractAddress.String()},
						{Key: "watermelon", Value: "🍉"},
					},
				},
				initEvents,
			)
		})
	}
}

func TestGasIsChargedForExecCallbackToInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, initErr)

			// exec callback to init
			_, _, _, _, _, err := execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"callback_to_init":{"code_id":%d,"code_hash":"%s"}}`, codeID, codeHash), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)
			require.Empty(t, err)
		})
	}
}

func TestWasmTooHighInitialMemoryRuntimeFail(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[tooHighMemoryContract], sdk.NewCoins())

	_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, false, defaultGasForTests)
	require.NotNil(t, err.GenericErr)
	require.Contains(t, err.GenericErr.Msg, "failed to initialize wasm memory")
}

func TestWasmTooHighInitialMemoryStaticFail(t *testing.T) {
	encodingConfig := MakeEncodingConfig()
	var transferPortSource types.ICS20TransferPortSource
	transferPortSource = MockIBCTransferKeeper{GetPortFn: func(ctx sdk.Context) string {
		return "myTransferPort"
	}}
	encoders := DefaultEncoders(transferPortSource, encodingConfig.Marshaler)
	ctx, keepers := CreateTestInput(t, false, SupportedFeatures, &encoders, nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	walletA, _ := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, sdk.NewCoins(sdk.NewInt64Coin("denom", 1)))

	wasmCode, err := os.ReadFile(TestContractPaths[staticTooHighMemoryContract])
	require.NoError(t, err)

	_, err = keeper.Create(ctx, walletA, wasmCode, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Error during static Wasm validation: Wasm contract memory's minimum must not exceed 512 pages")
}

func TestWasmWithFloatingPoints(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v010WithFloats], sdk.NewCoins())

			_, _, _, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, false, testContract.IsCosmWasmV1, defaultGasForTests)
			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "found floating point operation in module code")
		})
	}
}

func TestCodeHashInvalid(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())
			initMsg := []byte(`AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashEmpty(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())
			initMsg := []byte(`{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashNotHex(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())
			initMsg := []byte(`🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉🍉{"nop":{}}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashTooSmall(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			initMsg := []byte(codeHash[0:63] + `{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashTooBig(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			initMsg := []byte(codeHash + "a" + `{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)

			initErr := extractInnerError(t, err, enc[0:32], true, testContract.IsCosmWasmV1)
			require.NotEmpty(t, initErr)
			require.Contains(t, initErr.Error(), "Expected to parse either a `true`, `false`, or a `null`.")
		})
	}
}

func TestCodeHashWrong(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privWalletA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			initMsg := []byte(`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855{"nop":{}`)

			enc, _ := wasmCtx.Encrypt(initMsg)

			ctx = PrepareInitSignedTx(t, keeper, ctx, walletA, privWalletA, enc, codeID, sdk.NewCoins(sdk.NewInt64Coin("denom", 0)))
			_, _, err := keeper.Instantiate(ctx, codeID, walletA, nil, enc, "some label", sdk.NewCoins(sdk.NewInt64Coin("denom", 0)), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to validate transaction")
		})
	}
}

func TestCodeHashInitCallInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, addr, events, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

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
							{Key: "a", Value: "a"},
						},
						{
							{Key: "contract_address", Value: newContractBech32},
							{Key: "init", Value: "🌈"},
						},
					},
					events,
				)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"","msg":"%s","label":"2"}}`, codeID, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%sa","msg":"%s","label":"3"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"4"}}`, codeID, codeHash[0:63], `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s","label":"5"}}`, codeID, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestCodeHashInitCallExec(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 1, sdk.NewCoins())
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, addr2, events, err := initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: addr2.String()},
							{Key: "b", Value: "b"},
						},
						{
							{Key: "contract_address", Value: addr.String()},
							{Key: "watermelon", Value: "🍉"},
						},
					},
					events,
				)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"c\":{\"x\":1,\"y\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_exec":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"c\":{\"x\":1,\"y\":1}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 2, sdk.NewCoins())

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestCodeHashInitCallQuery(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, addr2, events, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.Empty(t, err)
				requireEvents(t,
					[]ContractEvent{
						{
							{Key: "contract_address", Value: addr2.String()},
							{Key: "c", Value: "2"},
						},
					},
					events,
				)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%sa","msg":"%s"}}`, addr.String(), codeHash, `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"Expected to parse either a `true`, `false`, or a `null`.",
				)
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"%s","msg":"%s"}}`, addr.String(), codeHash[0:63], `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, err = initHelper(t, keeper, ctx, codeID, walletA, privKeyA, fmt.Sprintf(`{"call_to_query":{"addr":"%s","code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s"}}`, addr.String(), `{\"receive_external_query\":{\"num\":1}}`), true, testContract.IsCosmWasmV1, defaultGasForTests)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestCodeHashExecCallInit(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			t.Run("GoodCodeHash", func(t *testing.T) {
				_, _, _, events, _, err := execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

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
							{Key: "a", Value: "a"},
						},
						{
							{Key: "contract_address", Value: newContractBech32},
							{Key: "init", Value: "🌈"},
						},
					},
					events,
				)
			})
			t.Run("EmptyCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"","msg":"%s","label":"2"}}`, codeID, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("TooBigCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%sa","msg":"%s","label":"3"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				if testContract.IsCosmWasmV1 {
					require.Contains(t,
						err.Error(),
						"v1_sanity_contract::msg::InstantiateMsg: Expected to parse either a `true`, `false`, or a `null`.",
					)
				} else {
					require.Contains(t,
						err.Error(),
						"parsing test_contract::contract::InitMsg: Expected to parse either a `true`, `false`, or a `null`.",
					)
				}
			})
			t.Run("TooSmallCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"4"}}`, codeID, codeHash[0:63], `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
			t.Run("IncorrectCodeHash", func(t *testing.T) {
				_, _, _, _, _, err := execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","msg":"%s","label":"5"}}`, codeID, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)

				require.NotEmpty(t, err)
				require.Contains(t,
					err.Error(),
					"failed to validate transaction",
				)
			})
		})
	}
}

func TestLabelCollisionWhenMultipleCallbacksToInitFromSameContract(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, codeHash, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), true, testContract.IsCosmWasmV1, defaultGasForTests, 0, 2)
			require.Empty(t, err)

			_, _, _, _, _, err = execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"call_to_init":{"code_id":%d,"code_hash":"%s","msg":"%s","label":"1"}}`, codeID, codeHash, `{\"nop\":{}}`), false, testContract.IsCosmWasmV1, defaultGasForTests, 0, 1)
			require.NotEmpty(t, err)
			require.NotNil(t, err.GenericErr)
			require.Contains(t, err.GenericErr.Msg, "contract account already exists")
		})
	}
}

func TestInitIllegalInputError(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, _, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `bad input`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			if testContract.IsCosmWasmV1 {
				require.NotNil(t, initErr.GenericErr)
				require.Contains(t, initErr.GenericErr.Msg, "Error parsing")
			} else {
				require.NotNil(t, initErr.ParseErr)
			}
		})
	}
}

func TestBenchmarkSecp256k1VerifyAPI(t *testing.T) {
	t.SkipNow()
	// Assaf: I wrote the benchmark like this because the init functions take testing.T
	// and not testing.B and I just wanted to quickly get a feel for the perf improvements
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			start := time.Now()
			// https://paulmillr.com/noble/
			execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify":{"iterations":10,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, defaultGasForTests, 0)
			elapsed := time.Since(start)
			fmt.Printf("TestBenchmarkSecp256k1VerifyAPI took %s\n", elapsed)
		})
	}
}

func TestBenchmarkSecp256k1VerifyCrate(t *testing.T) {
	t.SkipNow()
	// Assaf: I wrote the benchmark like this because the init functions take testing.T
	// and not testing.B and I just wanted to quickly get a feel for the perf improvements
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, _, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"nop":{}}`, true, testContract.IsCosmWasmV1, defaultGasForTests)

			start := time.Now()
			// https://paulmillr.com/noble/
			execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"secp256k1_verify_from_crate":{"iterations":10,"pubkey":"A0ZGrlBHMWtCMNAIbIrOxofwCxzZ0dxjT2yzWKwKmo//","sig":"/hZeEYHs9trj+Akeb+7p3UAtXjcDNYP9/D/hj/ALIUAG9bfrJltxkfpMz/9Jn5K3c5QjLuvaNT2jgr7P/AEW8A==","msg_hash":"ARp3VEHssUlDEwoW8AzdQYGKg90ENy8yWePKcjfjzao="}}`, true, testContract.IsCosmWasmV1, 100_000_000, 0)
			elapsed := time.Since(start)
			fmt.Printf("TestBenchmarkSecp256k1VerifyCrate took %s\n", elapsed)
		})
	}
}

func TestInitCreateNewContract(t *testing.T) {
	for _, testContract := range testContracts {
		t.Run(testContract.CosmWasmVersion, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, testContract.WasmFilePath, sdk.NewCoins())

			_, _, contractAddress, ev, _ := initHelper(t, keeper, ctx, codeID, walletA, privKeyA, `{"counter":{"counter":10, "expires":100}}`, true, true, defaultGasForTests)
			_, _, _, ev, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, `{"init_new_contract":{}}`, true, testContract.IsCosmWasmV1, math.MaxUint64, 0)

			require.Empty(t, err)

			var newContractBech32 string
			for _, v := range ev[1] {
				if v.Key == "contract_address" {
					newContractBech32 = v.Value
					break
				}
			}
			require.NotEmpty(t, newContractBech32)

			newContractAddress, Aerr := sdk.AccAddressFromBech32(newContractBech32)
			require.Empty(t, Aerr)
			queryRes, qErr := queryHelper(t, keeper, ctx, contractAddress, `{"get":{}}`, true, true, math.MaxUint64)
			require.Empty(t, qErr)

			var resp v1QueryResponse
			e := json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(10), resp.Get.Count)

			queryRes, qErr = queryHelper(t, keeper, ctx, newContractAddress, `{"get":{}}`, true, true, math.MaxUint64)
			require.Empty(t, qErr)

			e = json.Unmarshal([]byte(queryRes), &resp)
			require.NoError(t, e)
			require.Equal(t, uint32(150), resp.Get.Count)
		})
	}
}
