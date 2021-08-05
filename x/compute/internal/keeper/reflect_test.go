package keeper

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	wasmTypes "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
)

// MaskInitMsg is {}

// MaskHandleMsg is used to encode handle messages
type MaskHandleMsg struct {
	Reflect *reflectPayload `json:"reflect_msg,omitempty"`
	Change  *ownerPayload   `json:"change_owner,omitempty"`
}

type ownerPayload struct {
	Owner sdk.Address `json:"owner"`
}

type reflectPayload struct {
	Msgs []wasmTypes.CosmosMsg `json:"msgs"`
}

// MaskQueryMsg is used to encode query messages
type MaskQueryMsg struct {
	Owner         *struct{} `json:"owner,omitempty"`
	ReflectCustom *Text     `json:"reflect_custom,omitempty"`
}

type Text struct {
	Text string `json:"text"`
}

type OwnerResponse struct {
	Owner string `json:"owner,omitempty"`
}

const MaskFeatures = "staking,mask"

func TestMaskReflectContractSend(t *testing.T) {
	ctx, keepers := CreateTestInput(t, false, MaskFeatures, reflectEncoders(MakeTestCodec()), nil)
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, privCreator := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)
	_, _, bob := keyPubAddr()

	// upload mask code
	maskCode, err := ioutil.ReadFile("./testdata/reflect.wasm")
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

	maskStart := sdk.NewCoins(sdk.NewInt64Coin("denom", 40000))

	initMsgBz, err := testEncrypt(t, keeper, ctx, nil, maskID, []byte("{}"))
	require.NoError(t, err)

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, privCreator, initMsgBz, maskID, maskStart)

	maskAddr, err := keeper.Instantiate(ctx, maskID, creator /* nil,*/, initMsgBz, "mask contract 2", maskStart, nil)
	require.NoError(t, err)
	require.NotEmpty(t, maskAddr)

	// now we set contract as verifier of an escrow
	initMsg := InitMsg{
		Verifier:    maskAddr,
		Beneficiary: bob,
	}

	initMsgBz, err = json.Marshal(initMsg)
	require.NoError(t, err)

	initMsgBz, err = testEncrypt(t, keeper, ctx, nil, escrowID, initMsgBz)
	require.NoError(t, err)

	escrowStart := sdk.NewCoins(sdk.NewInt64Coin("denom", 25000))

	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, privCreator, initMsgBz, escrowID, escrowStart)

	escrowAddr, err := keeper.Instantiate(ctx, escrowID, creator /* nil,*/, initMsgBz, "escrow contract 2", escrowStart, nil)

	require.NoError(t, err)
	require.NotEmpty(t, escrowAddr)

	// let's make sure all balances make sense
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, creator, sdk.NewCoins(sdk.NewInt64Coin("denom", 35000))) // 100k - 40k - 25k
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, maskAddr, maskStart)
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, escrowAddr, escrowStart)
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, bob, nil)

	// now for the trick.... we reflect a message through the mask to call the escrow
	// we also send an additional 14k tokens there.
	// this should reduce the mask balance by 14k (to 26k)
	// this 14k is added to the escrow, then the entire balance is sent to bob (total: 39k)

	contractCodeHash := hex.EncodeToString(keeper.GetContractHash(ctx, escrowAddr))
	// approveMsg := []byte(contractCodeHash + `{"release":{}}`)
	msgs := []wasmTypes.CosmosMsg{{
		Wasm: &wasmTypes.WasmMsg{
			Execute: &wasmTypes.ExecuteMsg{
				ContractAddr:     escrowAddr.String(),
				CallbackCodeHash: contractCodeHash,
				Msg:              []byte(`{"release":{}}`),
				Send: []wasmTypes.Coin{{
					Denom:  "denom",
					Amount: "14000",
				}},
			},
		},
	}}
	reflectSend := MaskHandleMsg{
		Reflect: &reflectPayload{
			Msgs: msgs,
		},
	}
	reflectSendBz, err := json.Marshal(reflectSend)
	require.NoError(t, err)

	reflectSendBz, err = testEncrypt(t, keeper, ctx, maskAddr, 0, reflectSendBz)
	require.NoError(t, err)

	ctx = PrepareExecSignedTx(t, keeper, ctx, creator, privCreator, reflectSendBz, maskAddr, nil)

	_, err = keeper.Execute(ctx, maskAddr, creator, reflectSendBz, nil, nil)
	require.NoError(t, err)

	// did this work???
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, creator, sdk.NewCoins(sdk.NewInt64Coin("denom", 35000)))  // same as before
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, maskAddr, sdk.NewCoins(sdk.NewInt64Coin("denom", 26000))) // 40k - 14k (from send)
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, escrowAddr, sdk.Coins{})                                  // emptied reserved
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, bob, sdk.NewCoins(sdk.NewInt64Coin("denom", 39000)))      // all escrow of 25k + 14k

}

func TestMaskReflectCustomMsg(t *testing.T) {
	ctx, keepers := CreateTestInput(t, false, MaskFeatures, reflectEncoders(MakeTestCodec()), reflectPlugins())
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, privCreator := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)
	bob, privBob := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)
	_, _, fred := keyPubAddr()

	// upload code
	maskCode, err := ioutil.ReadFile("./testdata/reflect.wasm")
	require.NoError(t, err)
	codeID, err := keeper.Create(ctx, creator, maskCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), codeID)

	// creator instantiates a contract and gives it tokens
	initMsgBz, err := testEncrypt(t, keeper, ctx, nil, codeID, []byte("{}"))
	require.NoError(t, err)
	contractStart := sdk.NewCoins(sdk.NewInt64Coin("denom", 40000))
	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, privCreator, initMsgBz, codeID, contractStart)
	contractAddr, err := keeper.Instantiate(ctx, codeID, creator /* nil,*/, initMsgBz, "mask contract 1", contractStart, nil)
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
	transferBz, err = testEncrypt(t, keeper, ctx, contractAddr, 0, transferBz)
	require.NoError(t, err)
	ctx = PrepareExecSignedTx(t, keeper, ctx, creator, privCreator, transferBz, contractAddr, nil)
	_, err = keeper.Execute(ctx, contractAddr, creator, transferBz, nil, nil)
	require.NoError(t, err)

	// check some account values
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, contractAddr, contractStart)
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, bob, deposit)
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, fred, nil)

	// bob can send contract's tokens to fred (using SendMsg)
	msgs := []wasmTypes.CosmosMsg{{
		Bank: &wasmTypes.BankMsg{
			Send: &wasmTypes.SendMsg{
				FromAddress: contractAddr.String(),
				ToAddress:   fred.String(),
				Amount: []wasmTypes.Coin{{
					Denom:  "denom",
					Amount: "15000",
				}},
			},
		},
	}}
	reflectSend := MaskHandleMsg{
		Reflect: &reflectPayload{
			Msgs: msgs,
		},
	}
	reflectSendBz, err := json.Marshal(reflectSend)
	require.NoError(t, err)
	reflectSendBz, err = testEncrypt(t, keeper, ctx, contractAddr, 0, reflectSendBz)
	require.NoError(t, err)
	ctx = PrepareExecSignedTx(t, keeper, ctx, bob, privBob, reflectSendBz, contractAddr, nil)
	_, err = keeper.Execute(ctx, contractAddr, bob, reflectSendBz, nil, nil)
	require.NoError(t, err)

	// fred got coins
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, fred, sdk.NewCoins(sdk.NewInt64Coin("denom", 15000)))
	// contract lost them
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, contractAddr, sdk.NewCoins(sdk.NewInt64Coin("denom", 25000)))
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, bob, deposit)

	// construct an opaque message
	var sdkSendMsg sdk.Msg = &banktypes.MsgSend{
		FromAddress: contractAddr.String(),
		ToAddress:   fred.String(),
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("denom", 23000)),
	}
	opaque, err := toReflectRawMsg(keeper.cdc, sdkSendMsg)
	require.NoError(t, err)
	reflectOpaque := MaskHandleMsg{
		Reflect: &reflectPayload{
			Msgs: []wasmTypes.CosmosMsg{opaque},
		},
	}
	reflectOpaqueBz, err := json.Marshal(reflectOpaque)
	require.NoError(t, err)
	reflectOpaqueBz, err = testEncrypt(t, keeper, ctx, contractAddr, 0, reflectOpaqueBz)
	require.NoError(t, err)

	ctx = PrepareExecSignedTx(t, keeper, ctx, bob, privBob, reflectOpaqueBz, contractAddr, nil)
	_, err = keeper.Execute(ctx, contractAddr, bob, reflectOpaqueBz, nil, nil)
	require.NoError(t, err)

	// fred got more coins
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, fred, sdk.NewCoins(sdk.NewInt64Coin("denom", 38000)))
	// contract lost them
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, contractAddr, sdk.NewCoins(sdk.NewInt64Coin("denom", 2000)))
	checkAccount(t, ctx, accKeeper, keeper.bankKeeper, bob, deposit)
}

func TestMaskReflectCustomQuery(t *testing.T) {
	ctx, keepers := CreateTestInput(t, false, MaskFeatures, reflectEncoders(MakeTestCodec()), reflectPlugins())
	accKeeper, keeper := keepers.AccountKeeper, keepers.WasmKeeper

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	creator, privCreator := CreateFakeFundedAccount(ctx, accKeeper, keeper.bankKeeper, deposit)

	// upload code
	maskCode, err := ioutil.ReadFile("./testdata/reflect.wasm")
	require.NoError(t, err)
	codeID, err := keeper.Create(ctx, creator, maskCode, "", "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), codeID)

	// creator instantiates a contract and gives it tokens
	initMsgBz, err := testEncrypt(t, keeper, ctx, nil, codeID, []byte("{}"))
	require.NoError(t, err)
	contractStart := sdk.NewCoins(sdk.NewInt64Coin("denom", 40000))
	ctx = PrepareInitSignedTx(t, keeper, ctx, creator, privCreator, initMsgBz, codeID, contractStart)
	contractAddr, err := keeper.Instantiate(ctx, codeID, creator /* nil,*/, initMsgBz, "mask contract 1", contractStart, nil)
	require.NoError(t, err)
	require.NotEmpty(t, contractAddr)

	// let's perform a normal query of state
	ownerQuery := MaskQueryMsg{
		Owner: &struct{}{},
	}
	ownerQueryBz, err := json.Marshal(ownerQuery)
	require.NoError(t, err)

	ownerRes, qErr := queryHelper(t, keeper, ctx, contractAddr, string(ownerQueryBz), true, defaultGasForTests)
	require.Empty(t, qErr)
	var res OwnerResponse
	err = json.Unmarshal([]byte(ownerRes), &res)
	require.NoError(t, err)
	assert.Equal(t, res.Owner, creator.String())

	// and now making use of the custom querier callbacks
	customQuery := MaskQueryMsg{
		ReflectCustom: &Text{
			Text: "all Caps noW",
		},
	}
	customQueryBz, err := json.Marshal(customQuery)
	require.NoError(t, err)

	custom, qErr := queryHelper(t, keeper, ctx, contractAddr, string(customQueryBz), true, defaultGasForTests)
	require.Empty(t, qErr)

	var resp customQueryResponse
	err = json.Unmarshal([]byte(custom), &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Msg, "ALL CAPS NOW")
}

func checkAccount(t *testing.T, ctx sdk.Context, accKeeper authkeeper.AccountKeeper, bankKeeper bankkeeper.Keeper, addr sdk.AccAddress, expected sdk.Coins) {
	acct := accKeeper.GetAccount(ctx, addr)
	if expected == nil {
		assert.Nil(t, acct)
	} else {
		assert.NotNil(t, acct)
		coins := bankKeeper.GetAllBalances(ctx, acct.GetAddress())
		if expected.Empty() {
			// there is confusion between nil and empty slice... let's just treat them the same
			assert.True(t, coins.Empty())
		} else {
			assert.Equal(t, coins, expected)
		}
	}
}

/**** Code to support custom messages *****/

type reflectCustomMsg struct {
	Debug string `json:"debug,omitempty"`
	Raw   []byte `json:"raw,omitempty"`
}

// toReflectRawMsg encodes an sdk msg using any type with json encoding.
// Then wraps it as an opaque message
func toReflectRawMsg(cdc codec.BinaryCodec, msg sdk.Msg) (wasmTypes.CosmosMsg, error) {
	any, err := codectypes.NewAnyWithValue(msg)
	if err != nil {
		return wasmTypes.CosmosMsg{}, err
	}
	rawBz, err := cdc.Marshal(any)
	if err != nil {
		return wasmTypes.CosmosMsg{}, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}
	customMsg, err := json.Marshal(reflectCustomMsg{
		Raw: rawBz,
	})
	res := wasmTypes.CosmosMsg{
		Custom: customMsg,
	}
	return res, nil
}

// reflectEncoders needs to be registered in test setup to handle custom message callbacks
func reflectEncoders(cdc codec.Codec) *MessageEncoders {
	return &MessageEncoders{
		Custom: fromReflectRawMsg(cdc),
	}
}

// fromReflectRawMsg decodes msg.Data to an sdk.Msg using proto Any and json encoding.
// this needs to be registered on the Encoders
func fromReflectRawMsg(cdc codec.Codec) CustomEncoder {
	return func(_sender sdk.AccAddress, msg json.RawMessage) ([]sdk.Msg, error) {
		var custom reflectCustomMsg
		err := json.Unmarshal(msg, &custom)
		if err != nil {
			return nil, sdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
		}
		if custom.Raw != nil {
			var any codectypes.Any
			if err := cdc.Unmarshal(custom.Raw, &any); err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
			}
			var msg sdk.Msg
			if err := cdc.UnpackAny(&any, &msg); err != nil {
				return nil, err
			}
			return []sdk.Msg{msg}, nil
		}
		if custom.Debug != "" {
			return nil, sdkerrors.Wrapf(types.ErrInvalidMsg, "Custom Debug: %s", custom.Debug)
		}
		return nil, sdkerrors.Wrap(types.ErrInvalidMsg, "Unknown Custom message variant")
	}
}

type reflectCustomQuery struct {
	Ping    *struct{} `json:"ping,omitempty"`
	Capital *Text     `json:"capital,omitempty"`
}

// this is from the go code back to the contract (capitalized or ping)
type customQueryResponse struct {
	Msg string `json:"msg"`
}

// these are the return values from contract -> go depending on type of query
type ownerResponse struct {
	Owner string `json:"owner"`
}

type capitalizedResponse struct {
	Text string `json:"text"`
}

type chainResponse struct {
	Data []byte `json:"data"`
}

// reflectPlugins needs to be registered in test setup to handle custom query callbacks
func reflectPlugins() *QueryPlugins {
	return &QueryPlugins{
		Custom: performCustomQuery,
	}
}

func performCustomQuery(_ sdk.Context, request json.RawMessage) ([]byte, error) {

	var custom reflectCustomQuery
	err := json.Unmarshal(request, &custom)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONUnmarshal, err.Error())
	}
	fmt.Println(fmt.Sprintf("0x%02x", request))
	if custom.Capital != nil {
		msg := strings.ToUpper(custom.Capital.Text)
		return json.Marshal(customQueryResponse{Msg: msg})
	}
	if custom.Ping != nil {
		return json.Marshal(customQueryResponse{Msg: "pong"})
	}

	return nil, sdkerrors.Wrap(types.ErrInvalidMsg, "Unknown Custom query variant")
}
