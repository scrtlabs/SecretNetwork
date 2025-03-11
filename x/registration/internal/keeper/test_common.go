package keeper

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	codec_types "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/ibc-go/modules/capability"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	regtypes "github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	"github.com/cosmos/cosmos-sdk/x/mint"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/evidence"

	"cosmossdk.io/x/upgrade"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	registrationmock "github.com/scrtlabs/SecretNetwork/x/registration/internal/keeper/mock"
)

type TestEncodingConfig struct {
	InterfaceRegistry codec_types.InterfaceRegistry
	Codec             codec.Codec
	Marshaler         *codec.ProtoCodec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

func CreateTestSeedConfig(t *testing.T) []byte {
	seed := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cert, err := os.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	key, err := fetchPubKeyFromLegacyCert(cert)
	require.NoError(t, err)

	cfg := regtypes.SeedConfig{
		EncryptedKey: seed,
		MasterKey:    base64.StdEncoding.EncodeToString(key),
		Version:      regtypes.SeedConfigVersion,
	}

	cfgBytes, err := json.Marshal(&cfg)
	require.NoError(t, err)

	return cfgBytes
}

var ModuleBasics = module.NewBasicManager(
	auth.AppModuleBasic{},
	bank.AppModuleBasic{},
	capability.AppModuleBasic{},
	staking.AppModuleBasic{},
	mint.AppModuleBasic{},
	distribution.AppModuleBasic{},
	gov.NewAppModuleBasic(
		[]govclient.ProposalHandler{
			paramsclient.ProposalHandler,
			// distrclient.ProposalHandler,
			// upgradeclient.ProposalHandler,
		},
	),
	slashing.AppModuleBasic{},
	// ibc.AppModuleBasic{},
	upgrade.AppModuleBasic{},
	evidence.AppModuleBasic{},
	transfer.AppModuleBasic{},
)

func MakeTestCodec() codec.Codec {
	return MakeEncodingConfig().Marshaler
}

func MakeEncodingConfig() TestEncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := codec_types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txCfg := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	std.RegisterInterfaces(interfaceRegistry)
	std.RegisterLegacyAminoCodec(amino)

	ModuleBasics.RegisterLegacyAminoCodec(amino)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	return TestEncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          txCfg,
		Amino:             amino,
	}
}

func CreateTestInput(t *testing.T, isCheckTx bool, tempDir string, bootstrap bool) (sdk.Context, Keeper) {
	err := os.Setenv("SGX_MODE", "SW")
	require.Nil(t, err)

	keys := storetypes.NewKVStoreKeys(regtypes.StoreKey)

	// replace the logger by testing values in a real test case (e.g. log.NewTestLogger(t))
	logger := log.NewNopLogger()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db, logger, metrics.NewNoOpMetrics())

	cms.MountStoreWithDB(keys[regtypes.StoreKey], storetypes.StoreTypeIAVL, db)
	err = cms.LoadLatestVersion()
	require.Nil(t, err)

	ctx := sdk.NewContext(cms, tmproto.Header{}, isCheckTx, log.NewNopLogger())
	cdc := MakeTestCodec()

	// TODO: register more than bank.send
	router := baseapp.NewMsgServiceRouter()

	// Load default wasm config
	keeper := NewKeeper(cdc, runtime.NewKVStoreService(keys[regtypes.StoreKey]), router, registrationmock.MockEnclaveApi{}, tempDir, bootstrap)

	return ctx, keeper
}
