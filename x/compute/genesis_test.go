package compute

import (
	"encoding/json"
	compute "github.com/enigmampc/SecretNetwork/x/compute/internal/keeper"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type contractState struct {
}

func TestInitGenesis(t *testing.T) {
	data := setupTest(t)

	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
	creator, privCreator := CreateFakeFundedAccount(data.ctx, data.acctKeeper, data.bankKeeper, deposit.Add(deposit...))
	fred, _ := CreateFakeFundedAccount(data.ctx, data.acctKeeper, data.bankKeeper, topUp)

	h := TestHandler(data.keeper)
	q := NewLegacyQuerier(data.keeper)

	t.Log("fail with invalid source url")
	msg := MsgStoreCode{
		Sender:       creator,
		WASMByteCode: testContract,
		Source:       "someinvalidurl",
		Builder:      "",
	}

	err := msg.ValidateBasic()
	require.Error(t, err)

	_, err = h(data.ctx, &msg)
	require.Error(t, err)

	t.Log("fail with relative source url")
	msg = MsgStoreCode{
		Sender:       creator,
		WASMByteCode: testContract,
		Source:       "./testdata/escrow.wasm",
		Builder:      "",
	}

	err = msg.ValidateBasic()
	require.Error(t, err)

	_, err = h(data.ctx, &msg)
	require.Error(t, err)

	t.Log("fail with invalid build tag")
	msg = MsgStoreCode{
		Sender:       creator,
		WASMByteCode: testContract,
		Source:       "",
		Builder:      "somerandombuildtag-0.6.2",
	}

	err = msg.ValidateBasic()
	require.Error(t, err)

	_, err = h(data.ctx, &msg)
	require.Error(t, err)

	t.Log("no error with valid source and build tag")
	msg = MsgStoreCode{
		Sender:       creator,
		WASMByteCode: testContract,
		Source:       "https://github.com/enigmampc/SecretNetwork/blob/cosnwasm/x/compute/testdata/escrow.wasm",
		Builder:      "confio/cosmwasm-opt:0.7.0",
	}
	err = msg.ValidateBasic()
	require.NoError(t, err)

	res, err := h(data.ctx, &msg)
	require.NoError(t, err)
	require.Equal(t, res.Data, []byte("1"))

	_, _, bob := keyPubAddr()
	initMsg := initMsg{
		Verifier:    fred,
		Beneficiary: bob,
	}
	initMsgBz, err := json.Marshal(initMsg)
	require.NoError(t, err)

	initCmd := MsgInstantiateContract{
		Sender:    creator,
		CodeID:    1,
		InitMsg:   initMsgBz,
		InitFunds: deposit,
	}

	//compute.PrepareInitSignedTx()
	data.ctx = compute.PrepareInitSignedTx(t, data.keeper, data.ctx, creator, privCreator, initMsgBz, 1, deposit)

	res, err = h(data.ctx, &initCmd)
	require.NoError(t, err)
	contractAddr := sdk.AccAddress(res.Data)

	execCmd := MsgExecuteContract{
		Sender:    fred,
		Contract:  contractAddr,
		Msg:       []byte(`{"release":{}}`),
		SentFunds: topUp,
	}
	res, err = h(data.ctx, &execCmd)
	require.NoError(t, err)

	// ensure all contract state is as after init
	assertCodeList(t, q, data.ctx, 1)
	assertCodeBytes(t, q, data.ctx, 1, testContract)

	assertContractList(t, q, data.ctx, 1, []string{contractAddr.String()})
	assertContractInfo(t, q, data.ctx, contractAddr, 1, creator)
	// assertContractState(t, q, data.ctx, contractAddr, state{
	// 	Verifier:    []byte(fred),
	// 	Beneficiary: []byte(bob),
	// 	Funder:      []byte(creator),
	// })

	// export into genstate
	genState := ExportGenesis(data.ctx, data.keeper)

	// create new app to import genstate into
	newData := setupTest(t)
	q2 := NewLegacyQuerier(data.keeper)

	// initialize new app with genstate
	InitGenesis(newData.ctx, newData.keeper, *genState)

	// run same checks again on newdata, to make sure it was reinitialized correctly
	assertCodeList(t, q2, newData.ctx, 1)
	assertCodeBytes(t, q2, newData.ctx, 1, testContract)

	assertContractList(t, q2, newData.ctx, 1, []string{contractAddr.String()})
	assertContractInfo(t, q2, newData.ctx, contractAddr, 1, creator)
	// assertContractState(t, q2, newData.ctx, contractAddr, state{
	// 	Verifier:    []byte(fred),
	// 	Beneficiary: []byte(bob),
	// 	Funder:      []byte(creator),
	// })
}
