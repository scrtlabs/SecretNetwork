package keeper

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"

	cosmwasm "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestSanity(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, filepath.Join(".", contractPath, "erc20.wasm"), sdk.NewCoins())

	// init
	initMsg := fmt.Sprintf(`{"decimals":10,"initial_balances":[{"address":"%s","amount":"108"},{"address":"%s","amount":"53"}],"name":"ReuvenPersonalRustCoin","symbol":"RPRC"}`, walletA.String(), walletB.String())

	_, _, contractAddress, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, initMsg, true, false, defaultGasForTests)
	require.Empty(t, err)
	// require.Empty(t, initEvents)

	// check state after init
	qRes, qErr := queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"108"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"53"}`, qRes)

	// transfer 10 from A to B
	_, _, data, wasmEvents, _, err := execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA,
		fmt.Sprintf(`{"transfer":{"amount":"10","recipient":"%s"}}`, walletB.String()), true, false, defaultGasForTests, 0)

	require.Empty(t, err)
	require.Empty(t, data)
	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: contractAddress.String()},
				{Key: "action", Value: "transfer"},
				{Key: "sender", Value: walletA.String()},
				{Key: "recipient", Value: walletB.String()},
			},
		},
		wasmEvents,
	)

	// check state after transfer
	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletA.String()), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"98"}`, qRes)

	qRes, qErr = queryHelper(t, keeper, ctx, contractAddress, fmt.Sprintf(`{"balance":{"address":"%s"}}`, walletB.String()), true, false, defaultGasForTests)
	require.Empty(t, qErr)
	require.JSONEq(t, `{"balance":"63"}`, qRes)
}

func TestEncryptedAndPlaintextLogs(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[plaintextLogsContract], sdk.NewCoins())

	_, _, addr, _, err := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{}`, true, false, defaultGasForTests)
	require.Empty(t, err)

	_, _, _, events, _, err := execHelperCustomWasmCount(t, keeper, ctx, addr, walletA, privKeyA, "{}", true, false, defaultGasForTests, 0, 1)

	require.Empty(t, err)
	requireEvents(t,
		[]ContractEvent{
			{
				{Key: "contract_address", Value: addr.String()},
				{Key: "encrypted log", Value: "encrypted value"},
				{Key: "ZW5jb2RlZCBsb2cK", Value: "ZW5jb2RlZCB2YWx1ZQo="},
				{Key: "plaintext log", Value: "plaintext value"},
			},
		},
		events,
	)
}

// In V1 there is no "from" field in Bank message functionality which means it shouldn't be tested
func TestContractTryToSendFundsFromSomeoneElse(t *testing.T) {
	ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v010Contract], sdk.NewCoins())

	_, _, addr, _, initErr := initHelper(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, true, false, defaultGasForTests)
	require.Empty(t, initErr)

	_, _, _, _, _, execErr := execHelper(t, keeper, ctx, addr, walletA, privKeyA, `{"deposit_to_contract":{}}`, false, false, defaultGasForTests, 17)

	require.Empty(t, execErr)

	contractCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, addr)
	walletCoinsBefore := keeper.bankKeeper.GetAllBalances(ctx, walletA)

	require.Equal(t, "17denom", contractCoinsBefore.String())
	require.Equal(t, "199983denom", walletCoinsBefore.String())

	_, _, _, _, _, execErr = execHelper(t, keeper, ctx, addr, walletA, privKeyA, fmt.Sprintf(`{"send_funds":{"from":"%s","to":"%s","denom":"%s","amount":%d}}`, walletA.String(), addr.String(), "denom", 17), false, false, defaultGasForTests, 0)

	require.NotNil(t, execErr.GenericErr)
	require.Contains(t, execErr.GenericErr.Msg, "contract doesn't have permission")
}

func TestV010BankMsgSendFrom(t *testing.T) {
	for _, callType := range []string{"init", "exec"} {
		t.Run(callType, func(t *testing.T) {
			ctx, keeper, codeID, _, walletA, privKeyA, walletB, _ := setupTest(t, TestContractPaths[v010Contract], sdk.NewCoins())

			var err cosmwasm.StdError
			var contractAddress sdk.AccAddress

			if callType == "init" {
				_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, nil, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","from":"%s","amount":[{"amount":"1","denom":"denom"}]}}`, walletB.String(), walletA.String()), false, false, defaultGasForTests, -1, sdk.NewCoins())
			} else {
				_, _, contractAddress, _, _ = initHelperImpl(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, false, false, defaultGasForTests, -1, sdk.NewCoins())

				_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_send":{"to":"%s","from":"%s","amount":[{"amount":"1","denom":"denom"}]}}`, walletB.String(), walletA.String()), false, false, math.MaxUint64, 0)
			}

			require.NotEmpty(t, err)
			require.Contains(t, err.Error(), "contract doesn't have permission to send funds from another account")
		})
	}
}

func TestBankMsgBurn(t *testing.T) {
	t.Run("v1", func(t *testing.T) {
		for _, callType := range []string{"init", "exec"} {
			t.Run(callType, func(t *testing.T) {
				for _, test := range []struct {
					description string
					sentFunds   sdk.Coins
				}{
					{
						description: "try to burn coins it has",
						sentFunds:   sdk.NewCoins(sdk.NewInt64Coin("denom", 1)),
					},
					{
						description: "try to burn coins it doesnt have",
						sentFunds:   sdk.NewCoins(),
					},
				} {
					t.Run(test.description, func(t *testing.T) {
						ctx, keeper, codeID, _, walletA, privKeyA, _, _ := setupTest(t, TestContractPaths[v1Contract], sdk.NewCoins())

						var err cosmwasm.StdError
						var contractAddress sdk.AccAddress

						if callType == "init" {
							_, _, _, _, err = initHelperImpl(t, keeper, ctx, codeID, walletA, nil, privKeyA, fmt.Sprintf(`{"bank_msg_burn":{"amount":[{"amount":"1","denom":"denom"}]}}`), false, false, defaultGasForTests, -1, test.sentFunds)
						} else {
							_, _, contractAddress, _, _ = initHelperImpl(t, keeper, ctx, codeID, walletA, nil, privKeyA, `{"nop":{}}`, false, false, defaultGasForTests, -1, test.sentFunds)

							_, _, _, _, _, err = execHelper(t, keeper, ctx, contractAddress, walletA, privKeyA, fmt.Sprintf(`{"bank_msg_burn":{"amount":[{"amount":"1","denom":"denom"}]}}`), false, false, math.MaxUint64, 0)
						}

						require.NotEmpty(t, err)
						require.Contains(t, err.Error(), "Unknown variant of Bank: invalid CosmosMsg from the contract")
					})
				}
			})
		}
	})
}
