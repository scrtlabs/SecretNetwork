package keeper

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/enigmampc/cosmos-sdk/baseapp"
	"github.com/enigmampc/cosmos-sdk/codec"
	"github.com/enigmampc/cosmos-sdk/store"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	"github.com/enigmampc/SecretNetwork/x/registration/internal/keeper/mock"
	"github.com/enigmampc/SecretNetwork/x/registration/internal/types"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"
)

func CreateTestSeedConfig(t *testing.T) []byte {

	seed := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	cfg := types.SeedConfig{
		EncryptedKey: seed,
		MasterCert:   base64.StdEncoding.EncodeToString(cert),
	}

	cfgBytes, err := json.Marshal(&cfg)
	require.NoError(t, err)

	return cfgBytes
}

func MakeTestCodec() *codec.Codec {
	var cdc = codec.New()

	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)

	return cdc
}

func CreateTestInput(t *testing.T, isCheckTx bool, tempDir string, bootstrap bool) (sdk.Context, Keeper) {

	err := os.Setenv("SGX_MODE", "SW")
	require.Nil(t, err)

	keyContract := sdk.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyContract, sdk.StoreTypeIAVL, db)
	err = ms.LoadLatestVersion()
	require.Nil(t, err)

	ctx := sdk.NewContext(ms, abci.Header{}, isCheckTx, log.NewNopLogger())
	cdc := MakeTestCodec()

	// TODO: register more than bank.send
	router := baseapp.NewRouter()

	// Load default wasm config
	keeper := NewKeeper(cdc, keyContract, router, mock.MockEnclaveApi{}, tempDir, bootstrap)

	return ctx, keeper
}
