package keeper

import (
	"encoding/base64"
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	"github.com/cosmos/cosmos-sdk/x/gov"
	//"github.com/cosmos/cosmos-sdk/x/ibc/applications/transfer"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	"github.com/cosmos/ibc-go/modules/apps/transfer"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	//ibc "github.com/cosmos/cosmos-sdk/x/ibc/core"
	"io/ioutil"
	"os"
	"testing"

	"github.com/cosmos/cosmos-sdk/x/mint"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/registration/internal/keeper/mock"
	regtypes "github.com/enigmampc/SecretNetwork/x/registration/internal/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"
)

func CreateTestSeedConfig(t *testing.T) []byte {

	seed := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	cfg := regtypes.SeedConfig{
		EncryptedKey: seed,
		MasterCert:   base64.StdEncoding.EncodeToString(cert),
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
		paramsclient.ProposalHandler, distrclient.ProposalHandler, upgradeclient.ProposalHandler,
	),
	crisis.AppModuleBasic{},
	slashing.AppModuleBasic{},
	//ibc.AppModuleBasic{},
	upgrade.AppModuleBasic{},
	evidence.AppModuleBasic{},
	transfer.AppModuleBasic{},
)

func MakeTestCodec() codec.Codec {
	return MakeEncodingConfig().Marshaler
}
func MakeEncodingConfig() params.EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txCfg := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	std.RegisterInterfaces(interfaceRegistry)
	std.RegisterLegacyAminoCodec(amino)

	ModuleBasics.RegisterLegacyAminoCodec(amino)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	return params.EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          txCfg,
		Amino:             amino,
	}
}

func CreateTestInput(t *testing.T, isCheckTx bool, tempDir string, bootstrap bool) (sdk.Context, Keeper) {

	err := os.Setenv("SGX_MODE", "SW")
	require.Nil(t, err)

	keyContract := sdk.NewKVStoreKey(regtypes.StoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyContract, sdk.StoreTypeIAVL, db)
	err = ms.LoadLatestVersion()
	require.Nil(t, err)

	ctx := sdk.NewContext(ms, tmproto.Header{}, isCheckTx, log.NewNopLogger())
	cdc := MakeTestCodec()

	// TODO: register more than bank.send
	router := baseapp.NewRouter()

	// Load default wasm config
	keeper := NewKeeper(cdc, keyContract, router, mock.MockEnclaveApi{}, tempDir, bootstrap)

	return ctx, keeper
}
