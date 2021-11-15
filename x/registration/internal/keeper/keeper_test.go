package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	eng "github.com/enigmampc/SecretNetwork/types"
	"github.com/enigmampc/SecretNetwork/x/registration/internal/types"
	ra "github.com/enigmampc/SecretNetwork/x/registration/remote_attestation"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(eng.Bech32PrefixAccAddr, eng.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(eng.Bech32PrefixValAddr, eng.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(eng.Bech32PrefixConsAddr, eng.Bech32PrefixConsPub)
	config.Seal()
}

func TestNewKeeper(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "reg")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	_, regKeeper := CreateTestInput(t, false, tempDir, true)
	require.NotNil(t, regKeeper)
}

func TestNewKeeper_Node(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "reg")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	seedPath := filepath.Join(tempDir, types.SecretNodeCfgFolder, types.SecretNodeSeedConfig)

	err = os.MkdirAll(filepath.Join(tempDir, types.SecretNodeCfgFolder), 0700)
	require.NoError(t, err)

	err = ioutil.WriteFile(seedPath, CreateTestSeedConfig(t), 0700)
	require.NoError(t, err)

	_, regKeeper := CreateTestInput(t, false, tempDir, false)
	require.NotNil(t, regKeeper)
}

func TestKeeper_RegisterationStore(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, regKeeper := CreateTestInput(t, false, tempDir, true)

	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	regInfo := types.RegistrationNodeInfo{
		Certificate:   cert,
		EncryptedSeed: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}

	regKeeper.SetRegistrationInfo(ctx, regInfo)

	publicKey, err := ra.VerifyRaCert(regInfo.Certificate)
	if err != nil {
		return
	}

	regInfo2 := regKeeper.getRegistrationInfo(ctx, publicKey)

	require.Equal(t, regInfo, *regInfo2)
}

func TestKeeper_RegisterNode(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, regKeeper := CreateTestInput(t, false, tempDir, true)

	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	regInfo := types.RegistrationNodeInfo{
		Certificate:   cert,
		EncryptedSeed: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}

	regKeeper.SetRegistrationInfo(ctx, regInfo)

	_, err = regKeeper.RegisterNode(ctx, cert)
	require.NoError(t, err)

}
