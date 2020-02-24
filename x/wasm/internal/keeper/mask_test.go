package keeper

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wasmTypes "github.com/confio/go-cosmwasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
)

// MaskInitMsg is {}

type MaskHandleMsg struct {
	Reflect *reflectPayload `json:"reflectmsg,omitempty"`
	Change  *ownerPayload   `json:"changeowner,omitempty"`
}

type ownerPayload struct {
	Owner sdk.Address `json:"owner"`
}

type reflectPayload struct {
	Msg wasmTypes.CosmosMsg `json:"msg"`
}

func TestMaskReflectOpaque(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit)
	bob := createFakeFundedAccount(ctx, accKeeper, deposit)
	_, _, fred := keyPubAddr()

	// upload code
	maskCode, err := ioutil.ReadFile("./testdata/mask.wasm")
	require.NoError(t, err)
	codeID, err := keeper.Create(ctx, creator, maskCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), codeID)

	// creator instantiates a contract and gives it tokens
	contractStart := sdk.NewCoins(sdk.NewInt64Coin("denom", 40000))
	contractAddr, err := keeper.Instantiate(ctx, codeID, creator, []byte("{}"), contractStart)
	require.NoError(t, err)
	require.NotEmpty(t, contractAddr)

	// set owner to bob
	transfer := MaskHandleMsg{
		Change: &ownerPayload{
			Owner: bob,
		},
	}
	transferBz, err := json.Marshal(transfer)
	require.NoError(t, err)
	_, err = keeper.Execute(ctx, contractAddr, creator, transferBz, nil)
	require.NoError(t, err)

	// check some account values
	checkAccount(t, ctx, accKeeper, contractAddr, contractStart)
	checkAccount(t, ctx, accKeeper, bob, deposit)
	checkAccount(t, ctx, accKeeper, fred, nil)

	// bob can send contract's tokens to fred (using SendMsg)
	msg := wasmTypes.CosmosMsg{
		Send: &wasmTypes.SendMsg{
			FromAddress: contractAddr.String(),
			ToAddress:   fred.String(),
			Amount: []wasmTypes.Coin{{
				Denom:  "denom",
				Amount: "15000",
			}},
		},
	}
	reflectSend := MaskHandleMsg{
		Reflect: &reflectPayload{
			Msg: msg,
		},
	}
	reflectSendBz, err := json.Marshal(reflectSend)
	require.NoError(t, err)
	_, err = keeper.Execute(ctx, contractAddr, bob, reflectSendBz, nil)
	require.NoError(t, err)

	// fred got coins
	checkAccount(t, ctx, accKeeper, fred, sdk.NewCoins(sdk.NewInt64Coin("denom", 15000)))
	// contract lost them
	checkAccount(t, ctx, accKeeper, contractAddr, sdk.NewCoins(sdk.NewInt64Coin("denom", 25000)))
	checkAccount(t, ctx, accKeeper, bob, deposit)

	// construct an opaque message
	var sdkSendMsg sdk.Msg = &bank.MsgSend{
		FromAddress: contractAddr,
		ToAddress:   fred,
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("denom", 23000)),
	}
	opaque, err := ToOpaqueMsg(keeper.cdc, sdkSendMsg)
	require.NoError(t, err)
	reflectOpaque := MaskHandleMsg{
		Reflect: &reflectPayload{
			Msg: wasmTypes.CosmosMsg{
				Opaque: opaque,
			},
		},
	}
	reflectOpaqueBz, err := json.Marshal(reflectOpaque)
	require.NoError(t, err)

	_, err = keeper.Execute(ctx, contractAddr, bob, reflectOpaqueBz, nil)
	require.NoError(t, err)

	// fred got more coins
	checkAccount(t, ctx, accKeeper, fred, sdk.NewCoins(sdk.NewInt64Coin("denom", 38000)))
	// contract lost them
	checkAccount(t, ctx, accKeeper, contractAddr, sdk.NewCoins(sdk.NewInt64Coin("denom", 2000)))
	checkAccount(t, ctx, accKeeper, bob, deposit)
}

func TestMaskReflectContractSend(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, accKeeper, keeper := CreateTestInput(t, false, tempDir)

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator := createFakeFundedAccount(ctx, accKeeper, deposit)
	_, _, bob := keyPubAddr()

	// upload mask code
	maskCode, err := ioutil.ReadFile("./testdata/mask.wasm")
	require.NoError(t, err)
	maskID, err := keeper.Create(ctx, creator, maskCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), maskID)

	// upload hackatom escrow code
	escrowCode, err := ioutil.ReadFile("./testdata/contract.wasm")
	require.NoError(t, err)
	escrowID, err := keeper.Create(ctx, creator, escrowCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(2), escrowID)

	// creator instantiates a contract and gives it tokens
	maskStart := sdk.NewCoins(sdk.NewInt64Coin("denom", 40000))
	maskAddr, err := keeper.Instantiate(ctx, maskID, creator, []byte("{}"), maskStart)
	require.NoError(t, err)
	require.NotEmpty(t, maskAddr)

	// now we set contract as verifier of an escrow
	initMsg := InitMsg{
		Verifier:    maskAddr,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)
	escrowStart := sdk.NewCoins(sdk.NewInt64Coin("denom", 25000))
	escrowAddr, err := keeper.Instantiate(ctx, escrowID, creator, initMsgBz, escrowStart)
	require.NoError(t, err)
	require.NotEmpty(t, escrowAddr)

	// let's make sure all balances make sense
	checkAccount(t, ctx, accKeeper, creator, sdk.NewCoins(sdk.NewInt64Coin("denom", 35000))) // 100k - 40k - 25k
	checkAccount(t, ctx, accKeeper, maskAddr, maskStart)
	checkAccount(t, ctx, accKeeper, escrowAddr, escrowStart)
	checkAccount(t, ctx, accKeeper, bob, nil)

	// now for the trick.... we reflect a message through the mask to call the escrow
	// we also send an additional 14k tokens there.
	// this should reduce the mask balance by 14k (to 26k)
	// this 14k is added to the escrow, then the entire balance is sent to bob (total: 39k)
	approveMsg := EncodeCosmosMsgContract(`{"release":{}}`)
	msg := wasmTypes.CosmosMsg{
		Contract: &wasmTypes.ContractMsg{
			ContractAddr: escrowAddr.String(),
			Msg:          approveMsg,
			Send: []wasmTypes.Coin{{
				Denom:  "denom",
				Amount: "14000",
			}},
		},
	}
	reflectSend := MaskHandleMsg{
		Reflect: &reflectPayload{
			Msg: msg,
		},
	}
	reflectSendBz, err := json.Marshal(reflectSend)
	require.NoError(t, err)
	_, err = keeper.Execute(ctx, maskAddr, creator, reflectSendBz, nil)
	require.NoError(t, err)

	// did this work???
	checkAccount(t, ctx, accKeeper, creator, sdk.NewCoins(sdk.NewInt64Coin("denom", 35000)))  // same as before
	checkAccount(t, ctx, accKeeper, maskAddr, sdk.NewCoins(sdk.NewInt64Coin("denom", 26000))) // 40k - 14k (from send)
	checkAccount(t, ctx, accKeeper, escrowAddr, sdk.Coins{})                                  // emptied reserved
	checkAccount(t, ctx, accKeeper, bob, sdk.NewCoins(sdk.NewInt64Coin("denom", 39000)))      // all escrow of 25k + 14k

}

func checkAccount(t *testing.T, ctx sdk.Context, accKeeper auth.AccountKeeper, addr sdk.AccAddress, expected sdk.Coins) {
	acct := accKeeper.GetAccount(ctx, addr)
	if expected == nil {
		assert.Nil(t, acct)
	} else {
		assert.NotNil(t, acct)
		if expected.Empty() {
			// there is confusion between nil and empty slice... let's just treat them the same
			assert.True(t, acct.GetCoins().Empty())
		} else {
			assert.Equal(t, acct.GetCoins(), expected)
		}
	}
}
