package keeper

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	"github.com/enigmampc/EnigmaBlockchain/x/registration/internal/types"
)

func CreateTestSeedConfig(t *testing.T) []byte {

	seed := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cert, err := ioutil.ReadFile("../../testdata/attestation_cert")
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
	keeper := NewKeeper(cdc, keyContract, router, MockEnclaveApi{}, tempDir, bootstrap)

	return ctx, keeper
}
