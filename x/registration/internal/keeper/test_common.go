package keeper

//"github.com/cosmos/ibc-go/v8/testing/simapp/params"

// "github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer"

// ibc "github.com/cosmos/cosmos-sdk/x/ibc/core"

// func CreateTestSeedConfig(t *testing.T) []byte {
// 	seed := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
// 	cert, err := os.ReadFile("../../testdata/attestation_cert_sw")
// 	require.NoError(t, err)

// 	key, err := fetchPubKeyFromLegacyCert(cert)
// 	require.NoError(t, err)

// 	cfg := regtypes.SeedConfig{
// 		EncryptedKey: seed,
// 		MasterKey:    base64.StdEncoding.EncodeToString(key),
// 		Version:      regtypes.SeedConfigVersion,
// 	}

// 	cfgBytes, err := json.Marshal(&cfg)
// 	require.NoError(t, err)

// 	return cfgBytes
// }

// var ModuleBasics = module.NewBasicManager(
// 	auth.AppModuleBasic{},
// 	bank.AppModuleBasic{},
// 	capability.AppModuleBasic{},
// 	staking.AppModuleBasic{},
// 	mint.AppModuleBasic{},
// 	distribution.AppModuleBasic{},
// 	gov.NewAppModuleBasic(
// 		paramsclient.ProposalHandler, distrclient.ProposalHandler, upgradeclient.ProposalHandler,
// 	),
// 	crisis.AppModuleBasic{},
// 	slashing.AppModuleBasic{},
// 	// ibc.AppModuleBasic{},
// 	upgrade.AppModuleBasic{},
// 	evidence.AppModuleBasic{},
// 	transfer.AppModuleBasic{},
// )

// func MakeTestCodec() codec.Codec {
// 	return MakeEncodingConfig().Marshaler
// }

// func MakeEncodingConfig() params.EncodingConfig {
// 	amino := codec.NewLegacyAmino()
// 	interfaceRegistry := types.NewInterfaceRegistry()
// 	marshaler := codec.NewProtoCodec(interfaceRegistry)
// 	txCfg := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

// 	std.RegisterInterfaces(interfaceRegistry)
// 	std.RegisterLegacyAminoCodec(amino)

// 	ModuleBasics.RegisterLegacyAminoCodec(amino)
// 	ModuleBasics.RegisterInterfaces(interfaceRegistry)
// 	return params.EncodingConfig{
// 		InterfaceRegistry: interfaceRegistry,
// 		Marshaler:         marshaler,
// 		TxConfig:          txCfg,
// 		Amino:             amino,
// 	}
// }

// func CreateTestInput(t *testing.T, isCheckTx bool, tempDir string, bootstrap bool) (sdk.Context, Keeper) {
// 	err := os.Setenv("SGX_MODE", "SW")
// 	require.Nil(t, err)

// 	keyContract := sdk.NewKVStoreKey(regtypes.StoreKey)

// 	db := dbm.NewMemDB()
// 	ms := store.NewCommitMultiStore(db)
// 	ms.MountStoreWithDB(keyContract, sdk.StoreTypeIAVL, db)
// 	err = ms.LoadLatestVersion()
// 	require.Nil(t, err)

// 	ctx := sdk.NewContext(ms, tmproto.Header{}, isCheckTx, log.NewNopLogger())
// 	cdc := MakeTestCodec()

// 	// TODO: register more than bank.send
// 	router := baseapp.NewRouter()

// 	// Load default wasm config
// 	keeper := NewKeeper(cdc, keyContract, router, mock.MockEnclaveApi{}, tempDir, bootstrap)

// 	return ctx, keeper
// }
