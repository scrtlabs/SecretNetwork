package registration

//
//type testData struct {
//	module     module.AppModule
//	ctx        sdk.Context
//	acctKeeper auth.AccountKeeper
//	keeper     Keeper
//}
//
//// returns a cleanup function, which must be deferred on
//func setupTest(t *testing.T) (testData, func()) {
//	tempDir, err := os.MkdirTemp("", "wasm")
//	require.NoError(t, err)
//
//	ctx, acctKeeper, keeper := CreateTestInput(t, false, tempDir)
//	data := testData{
//		module:     NewAppModule(keeper),
//		ctx:        ctx,
//		acctKeeper: acctKeeper,
//		keeper:     keeper,
//	}
//	cleanup := func() { os.RemoveAll(tempDir) }
//	return data, cleanup
//}
//
//func keyPubAddr() (crypto.PrivKey, crypto.PubKey, sdk.AccAddress) {
//	key := ed25519.GenPrivKey()
//	pub := key.PubKey()
//	addr := sdk.AccAddress(pub.Address())
//	return key, pub, addr
//}
//
//func mustLoad(path string) []byte {
//	bz, err := os.ReadFile(path)
//	if err != nil {
//		panic(err)
//	}
//	return bz
//}
//
//var (
//	key1, pub1, addr1 = keyPubAddr()
//	testContract      = mustLoad("./internal/keeper/testdata/contract.wasm")
//	escrowContract    = mustLoad("./testdata/escrow.wasm")
//)
//
//func TestHandleCreate(t *testing.T) {
//	cases := map[string]struct {
//		msg     sdk.Msg
//		isValid bool
//	}{
//		"empty": {
//			msg:     MsgStoreCode{},
//			isValid: false,
//		},
//		"invalid wasm": {
//			msg: MsgStoreCode{
//				Sender:       addr1,
//				WASMByteCode: []byte("foobar"),
//			},
//			isValid: false,
//		},
//		"valid wasm": {
//			msg: MsgStoreCode{
//				Sender:       addr1,
//				WASMByteCode: testContract,
//			},
//			isValid: true,
//		},
//		"other valid wasm": {
//			msg: MsgStoreCode{
//				Sender:       addr1,
//				WASMByteCode: escrowContract,
//			},
//			isValid: true,
//		},
//	}
//
//	for name, tc := range cases {
//		tc := tc
//		t.Run(name, func(t *testing.T) {
//			data, cleanup := setupTest(t)
//			defer cleanup()
//
//			h := data.module.NewHandler()
//			q := data.module.NewQuerierHandler()
//
//			res, err := h(data.ctx, tc.msg)
//			if !tc.isValid {
//				require.Error(t, err, "%#v", res)
//				assertCodeList(t, q, data.ctx, 0)
//				assertCodeBytes(t, q, data.ctx, 1, nil)
//				return
//			}
//			require.NoError(t, err)
//			assertCodeList(t, q, data.ctx, 1)
//		})
//	}
//}
//
//type initMsg struct {
//	Verifier    sdk.AccAddress `json:"verifier"`
//	Beneficiary sdk.AccAddress `json:"beneficiary"`
//}
//
//type state struct {
//	Verifier    wasmTypes.CanonicalAddress `json:"verifier"`
//	Beneficiary wasmTypes.CanonicalAddress `json:"beneficiary"`
//	Funder      wasmTypes.CanonicalAddress `json:"funder"`
//}
//
//func TestHandleInstantiate(t *testing.T) {
//	data, cleanup := setupTest(t)
//	defer cleanup()
//
//	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
//	creator , _ := createFakeFundedAccount(data.ctx, data.acctKeeper, deposit)
//
//	h := data.module.NewHandler()
//	q := data.module.NewQuerierHandler()
//
//	msg := MsgStoreCode{
//		Sender:       creator,
//		WASMByteCode: testContract,
//	}
//	res, err := h(data.ctx, msg)
//	require.NoError(t, err)
//	require.Equal(t, res.Data, []byte("1"))
//
//	_, _, bob := keyPubAddr()
//	_, _, fred := keyPubAddr()
//
//	initMsg := initMsg{
//		Verifier:    fred,
//		Beneficiary: bob,
//	}
//	initMsgBz, err := json.Marshal(initMsg)
//	require.NoError(t, err)
//
//	// create with no balance is also legal
//	initCmd := MsgInstantiateContract{
//		Sender:    creator,
//		Code:      1,
//		InitMsg:   initMsgBz,
//		InitFunds: nil,
//	}
//	res, err = h(data.ctx, initCmd)
//	require.NoError(t, err)
//	contractAddr := sdk.AccAddress(res.Data)
//	require.Equal(t, "secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg", contractAddr.String())
//	// this should be standard x/compute init event, nothing from contract
//	require.Equal(t, 1, len(res.Events), prettyEvents(res.Events))
//	assert.Equal(t, "message", res.Events[0].Type)
//	assertAttribute(t, "module", "wasm", res.Events[0].Attributes[0])
//
//	assertCodeList(t, q, data.ctx, 1)
//	assertCodeBytes(t, q, data.ctx, 1, testContract)
//
//	assertContractList(t, q, data.ctx, 1, []string{contractAddr.String()})
//	assertContractInfo(t, q, data.ctx, contractAddr, 1, creator)
//	assertContractState(t, q, data.ctx, contractAddr, state{
//		Verifier:    []byte(fred),
//		Beneficiary: []byte(bob),
//		Funder:      []byte(creator),
//	})
//}
//
//func TestHandleExecute(t *testing.T) {
//	data, cleanup := setupTest(t)
//	defer cleanup()
//
//	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
//	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
//	creator , _ := createFakeFundedAccount(data.ctx, data.acctKeeper, deposit.Add(deposit...))
//	fred , _ := createFakeFundedAccount(data.ctx, data.acctKeeper, topUp)
//
//	h := data.module.NewHandler()
//	q := data.module.NewQuerierHandler()
//
//	msg := MsgStoreCode{
//		Sender:       creator,
//		WASMByteCode: testContract,
//	}
//	res, err := h(data.ctx, msg)
//	require.NoError(t, err)
//	require.Equal(t, res.Data, []byte("1"))
//
//	_, _, bob := keyPubAddr()
//	initMsg := initMsg{
//		Verifier:    fred,
//		Beneficiary: bob,
//	}
//	initMsgBz, err := json.Marshal(initMsg)
//	require.NoError(t, err)
//
//	initCmd := MsgInstantiateContract{
//		Sender:    creator,
//		Code:      1,
//		InitMsg:   initMsgBz,
//		InitFunds: deposit,
//	}
//	res, err = h(data.ctx, initCmd)
//	require.NoError(t, err)
//	contractAddr := sdk.AccAddress(res.Data)
//	require.Equal(t, "secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg", contractAddr.String())
//	// this should be standard x/compute init event, plus a bank send event (2), with no custom contract events
//	require.Equal(t, 2, len(res.Events), prettyEvents(res.Events))
//	assert.Equal(t, "transfer", res.Events[0].Type)
//	assert.Equal(t, "message", res.Events[1].Type)
//	assertAttribute(t, "module", "wasm", res.Events[1].Attributes[0])
//
//	// ensure bob doesn't exist
//	bobAcct := data.acctKeeper.GetAccount(data.ctx, bob)
//	require.Nil(t, bobAcct)
//
//	// ensure funder has reduced balance
//	creatorAcct := data.acctKeeper.GetAccount(data.ctx, creator)
//	require.NotNil(t, creatorAcct)
//	// we started at 2*deposit, should have spent one above
//	assert.Equal(t, deposit, creatorAcct.GetCoins())
//
//	// ensure contract has updated balance
//	contractAcct := data.acctKeeper.GetAccount(data.ctx, contractAddr)
//	require.NotNil(t, contractAcct)
//	assert.Equal(t, deposit, contractAcct.GetCoins())
//
//	execCmd := MsgExecuteContract{
//		Sender:    fred,
//		Contract:  contractAddr,
//		Msg:       []byte(`{"release":{}}`),
//		SentFunds: topUp,
//	}
//	res, err = h(data.ctx, execCmd)
//	require.NoError(t, err)
//	// this should be standard x/compute init event, plus 2 bank send event, plus a special event from the contract
//	require.Equal(t, 4, len(res.Events), prettyEvents(res.Events))
//	assert.Equal(t, "transfer", res.Events[0].Type)
//	assertAttribute(t, "recipient", contractAddr.String(), res.Events[0].Attributes[0])
//	assertAttribute(t, "sender", fred.String(), res.Events[0].Attributes[1])
//	assertAttribute(t, "amount", "5000denom", res.Events[0].Attributes[2])
//	// custom contract event
//	assert.Equal(t, "wasm", res.Events[1].Type)
//	assertAttribute(t, "contract_address", contractAddr.String(), res.Events[1].Attributes[0])
//	assertAttribute(t, "action", "release", res.Events[1].Attributes[1])
//	// second transfer (this without conflicting message)
//	assert.Equal(t, "transfer", res.Events[2].Type)
//	assertAttribute(t, "recipient", bob.String(), res.Events[2].Attributes[0])
//	assertAttribute(t, "sender", contractAddr.String(), res.Events[2].Attributes[1])
//	assertAttribute(t, "amount", "105000denom", res.Events[2].Attributes[2])
//	// finally, standard x/compute tag
//	assert.Equal(t, "message", res.Events[3].Type)
//	assertAttribute(t, "module", "wasm", res.Events[3].Attributes[0])
//
//	// ensure bob now exists and got both payments released
//	bobAcct = data.acctKeeper.GetAccount(data.ctx, bob)
//	require.NotNil(t, bobAcct)
//	balance := bobAcct.GetCoins()
//	assert.Equal(t, deposit.Add(topUp...), balance)
//
//	// ensure contract has updated balance
//	contractAcct = data.acctKeeper.GetAccount(data.ctx, contractAddr)
//	require.NotNil(t, contractAcct)
//	assert.Equal(t, sdk.Coins(nil), contractAcct.GetCoins())
//
//	// ensure all contract state is as after init
//	assertCodeList(t, q, data.ctx, 1)
//	assertCodeBytes(t, q, data.ctx, 1, testContract)
//
//	assertContractList(t, q, data.ctx, 1, []string{contractAddr.String()})
//	assertContractInfo(t, q, data.ctx, contractAddr, 1, creator)
//	assertContractState(t, q, data.ctx, contractAddr, state{
//		Verifier:    []byte(fred),
//		Beneficiary: []byte(bob),
//		Funder:      []byte(creator),
//	})
//}
//
//func TestHandleExecuteEscrow(t *testing.T) {
//	data, cleanup := setupTest(t)
//	defer cleanup()
//
//	deposit := sdk.NewCoins(sdk.NewInt64Coin("denom", 100000))
//	topUp := sdk.NewCoins(sdk.NewInt64Coin("denom", 5000))
//	creator , _ := createFakeFundedAccount(data.ctx, data.acctKeeper, deposit.Add(deposit...))
//	fred , _ := createFakeFundedAccount(data.ctx, data.acctKeeper, topUp)
//
//	h := data.module.NewHandler()
//
//	msg := MsgStoreCode{
//		Sender:       creator,
//		WASMByteCode: escrowContract,
//	}
//	res, err := h(data.ctx, &msg)
//	require.NoError(t, err)
//	require.Equal(t, res.Data, []byte("1"))
//
//	_, _, bob := keyPubAddr()
//	initMsg := map[string]interface{}{
//		"arbiter":    fred.String(),
//		"recipient":  bob.String(),
//		"end_time":   0,
//		"end_height": 0,
//	}
//	initMsgBz, err := json.Marshal(initMsg)
//	require.NoError(t, err)
//
//	initCmd := MsgInstantiateContract{
//		Sender:    creator,
//		Code:      1,
//		InitMsg:   initMsgBz,
//		InitFunds: deposit,
//	}
//	res, err = h(data.ctx, initCmd)
//	require.NoError(t, err)
//	contractAddr := sdk.AccAddress(res.Data)
//	require.Equal(t, "secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg", contractAddr.String())
//
//	handleMsg := map[string]interface{}{
//		"approve": map[string]interface{}{},
//	}
//	handleMsgBz, err := json.Marshal(handleMsg)
//	require.NoError(t, err)
//
//	execCmd := MsgExecuteContract{
//		Sender:    fred,
//		Contract:  contractAddr,
//		Msg:       handleMsgBz,
//		SentFunds: topUp,
//	}
//	res, err = h(data.ctx, execCmd)
//	require.NoError(t, err)
//
//	// ensure bob now exists and got both payments released
//	bobAcct := data.acctKeeper.GetAccount(data.ctx, bob)
//	require.NotNil(t, bobAcct)
//	balance := bobAcct.GetCoins()
//	assert.Equal(t, deposit.Add(topUp...), balance)
//
//	// ensure contract has updated balance
//	contractAcct := data.acctKeeper.GetAccount(data.ctx, contractAddr)
//	require.NotNil(t, contractAcct)
//	assert.Equal(t, sdk.Coins(nil), contractAcct.GetCoins())
//
//	// q := data.module.NewQuerierHandler()
//	// // ensure all contract state is as after init
//	// assertCodeList(t, q, data.ctx, 1)
//	// assertCodeBytes(t, q, data.ctx, 1, testContract)
//
//	// assertContractList(t, q, data.ctx, []string{contractAddr.String()})
//	// assertContractInfo(t, q, data.ctx, contractAddr, 1, creator)
//	// assertContractState(t, q, data.ctx, contractAddr, state{
//	// 	Verifier:    fred.String(),
//	// 	Beneficiary: bob.String(),
//	// 	Funder:      creator.String(),
//	// })
//}
//
//type prettyEvent struct {
//	Type string
//	Attr []sdk.Attribute
//}
//
//func prettyEvents(evts sdk.Events) string {
//	res := make([]prettyEvent, len(evts))
//	for i, e := range evts {
//		res[i] = prettyEvent{
//			Type: e.Type,
//			Attr: prettyAttrs(e.Attributes),
//		}
//	}
//	bz, err := json.MarshalIndent(res, "", "  ")
//	if err != nil {
//		panic(err)
//	}
//	return string(bz)
//}
//
//func prettyAttrs(attrs []kv.Pair) []sdk.Attribute {
//	pretty := make([]sdk.Attribute, len(attrs))
//	for i, a := range attrs {
//		pretty[i] = prettyAttr(a)
//	}
//	return pretty
//}
//
//func prettyAttr(attr kv.Pair) sdk.Attribute {
//	return sdk.NewAttribute(string(attr.Key), string(attr.Value))
//}
//
//func assertAttribute(t *testing.T, key string, value string, attr kv.Pair) {
//	assert.Equal(t, key, string(attr.Key), prettyAttr(attr))
//	assert.Equal(t, value, string(attr.Value), prettyAttr(attr))
//}
//
//func assertCodeList(t *testing.T, q sdk.Querier, ctx sdk.Context, expectedNum int) {
//	bz, sdkerr := q(ctx, []string{QueryListCode}, abci.RequestQuery{})
//	require.NoError(t, sdkerr)
//
//	if len(bz) == 0 {
//		require.Equal(t, expectedNum, 0)
//		return
//	}
//
//	var res []CodeInfo
//	err := json.Unmarshal(bz, &res)
//	require.NoError(t, err)
//
//	assert.Equal(t, expectedNum, len(res))
//}
//
//func assertCodeBytes(t *testing.T, q sdk.Querier, ctx sdk.Context, codeID uint64, expectedBytes []byte) {
//	path := []string{QueryGetCode, fmt.Sprintf("%d", codeID)}
//	bz, sdkerr := q(ctx, path, abci.RequestQuery{})
//	require.NoError(t, sdkerr)
//
//	if len(bz) == 0 {
//		require.Equal(t, len(expectedBytes), 0)
//		return
//	}
//
//	var res GetCodeResponse
//	err := json.Unmarshal(bz, &res)
//	require.NoError(t, err)
//
//	assert.Equal(t, expectedBytes, res.Data)
//	assert.Equal(t, codeID, res.ID)
//}
//
//func assertContractList(t *testing.T, q sdk.Querier, ctx sdk.Context, codeID uint64, addrs []string) {
//	bz, sdkerr := q(ctx, []string{QueryListContractByCode, fmt.Sprintf("%d", codeID)}, abci.RequestQuery{})
//	require.NoError(t, sdkerr)
//
//	if len(bz) == 0 {
//		require.Equal(t, len(addrs), 0)
//		return
//	}
//
//	var res []ContractInfoWithAddress
//	err := json.Unmarshal(bz, &res)
//	require.NoError(t, err)
//
//	var hasAddrs = make([]string, len(res))
//	for i, r := range res {
//		hasAddrs[i] = r.Address.String()
//	}
//
//	assert.Equal(t, hasAddrs, addrs)
//}
//
//func assertContractState(t *testing.T, q sdk.Querier, ctx sdk.Context, addr sdk.AccAddress, expected state) {
//	path := []string{QueryGetContractState, addr.String(), keeper.QueryMethodContractStateAll}
//	bz, sdkerr := q(ctx, path, abci.RequestQuery{})
//	require.NoError(t, sdkerr)
//
//	var res []Model
//	err := json.Unmarshal(bz, &res)
//	require.NoError(t, err)
//	require.Equal(t, 1, len(res), "#v", res)
//	require.Equal(t, []byte("config"), []byte(res[0].Key))
//
//	expectedBz, err := json.Marshal(expected)
//	require.NoError(t, err)
//	assert.Equal(t, expectedBz, res[0].Value)
//}
//
//func assertContractInfo(t *testing.T, q sdk.Querier, ctx sdk.Context, addr sdk.AccAddress, codeID uint64, creator sdk.AccAddress) {
//	path := []string{QueryGetContract, addr.String()}
//	bz, sdkerr := q(ctx, path, abci.RequestQuery{})
//	require.NoError(t, sdkerr)
//
//	var res ContractInfo
//	err := json.Unmarshal(bz, &res)
//	require.NoError(t, err)
//
//	assert.Equal(t, codeID, res.CodeID)
//	assert.Equal(t, creator, res.Creator)
//}
//
//func createFakeFundedAccount(ctx sdk.Context, am auth.AccountKeeper, coins sdk.Coins) sdk.AccAddress {
//	_, _, addr := keyPubAddr()
//	baseAcct := auth.NewBaseAccountWithAddress(addr)
//	_ = baseAcct.SetCoins(coins)
//	am.SetAccount(ctx, &baseAcct)
//
//	return addr
//}
