package keeper

//
////
import (
	"encoding/hex"
	"encoding/json"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/SecretNetwork/x/registration/internal/types"
	ra "github.com/enigmampc/SecretNetwork/x/registration/remote_attestation"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
)

func TestNewQuerier(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keeper := CreateTestInput(t, false, tempDir, true)

	nodeIdInvalid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	nodeIdValid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	querier := NewLegacyQuerier(keeper) // TODO: Should test NewQuerier() as well

	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	regInfo := types.RegistrationNodeInfo{
		Certificate:   cert,
		EncryptedSeed: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}

	keeper.SetRegistrationInfo(ctx, regInfo)

	publicKey, err := ra.VerifyRaCert(regInfo.Certificate)
	if err != nil {
		return
	}

	expectedSecretParams, _ := json.Marshal(types.GenesisState{
		Registration:              nil,
		NodeExchMasterCertificate: &types.MasterCertificate{Bytes: regInfo.Certificate},
		IoMasterCertificate:       &types.MasterCertificate{Bytes: regInfo.Certificate},
	})

	specs := map[string]struct {
		srcPath []string
		srcReq  abci.RequestQuery
		// smart queries return raw bytes from contract not []types.Model
		// if this is set, then we just compare - (should be json encoded string)
		// if success and expSmartRes is not set, we parse into []types.Model and compare
		expErr *sdkErrors.Error
		result string
	}{
		"query malformed node id": {
			[]string{QueryEncryptedSeed, nodeIdInvalid},
			abci.RequestQuery{Data: []byte("")},
			sdkErrors.ErrInvalidAddress,
			"",
		},
		"query invalid node id": {
			[]string{QueryEncryptedSeed, nodeIdValid},
			abci.RequestQuery{Data: []byte("")},
			sdkErrors.ErrUnknownAddress,
			"",
		},
		"query valid node id": {
			[]string{QueryEncryptedSeed, hex.EncodeToString(publicKey)},
			abci.RequestQuery{Data: []byte("")},
			nil,
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		"query master cert fail": {
			[]string{QueryMasterCertificate},
			abci.RequestQuery{Data: []byte("")},
			sdkErrors.ErrUnknownAddress,
			"",
		},
	}

	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			binResult, err := querier(ctx, spec.srcPath, spec.srcReq)
			require.True(t, spec.expErr.Is(err), err)

			if spec.result != "" {
				require.Equal(t, spec.result, string(binResult))
			}
		})
	}

	keeper.setMasterCertificate(ctx, types.MasterCertificate{Bytes: regInfo.Certificate}, types.MasterNodeKeyId)
	keeper.setMasterCertificate(ctx, types.MasterCertificate{Bytes: regInfo.Certificate}, types.MasterIoKeyId)

	binResult, err := querier(ctx, []string{QueryMasterCertificate}, abci.RequestQuery{Data: []byte("")})
	require.Equal(t, string(binResult), string(expectedSecretParams))
}
