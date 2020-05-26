package keeper

//
////
import (
	"encoding/hex"
	"encoding/json"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/enigmampc/EnigmaBlockchain/x/registration/internal/types"
	ra "github.com/enigmampc/EnigmaBlockchain/x/registration/remote_attestation"
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

	querier := NewQuerier(keeper)

	cert, err := ioutil.ReadFile("../../testdata/attestation_cert")
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
		NodeExchMasterCertificate: regInfo.Certificate,
		IoMasterCertificate:       regInfo.Certificate,
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
			types.ErrInvalidType,
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
		"query master cert": {
			[]string{QueryMasterCertificate},
			abci.RequestQuery{Data: []byte("")},
			nil,
			string(expectedSecretParams),
		},
	}

	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			if msg == "query master cert" {
				keeper.setMasterCertificate(ctx, types.MasterCertificate(regInfo.Certificate), types.MasterNodeKeyId)
				keeper.setMasterCertificate(ctx, types.MasterCertificate(regInfo.Certificate), types.MasterIoKeyId)
			}
			binResult, err := querier(ctx, spec.srcPath, spec.srcReq)
			require.True(t, spec.expErr.Is(err), err)

			if spec.result != "" {
				require.Equal(t, string(binResult), spec.result)
			}
		})
	}
}
