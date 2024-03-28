package keeper

// import (
// 	"encoding/hex"
// 	"encoding/json"
// 	"os"
// 	"testing"

// 	"cosmossdk.io/api/tendermint/abci"
// 	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
// 	errorsmod "cosmossdk.io/errors"
// 	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
// 	"github.com/scrtlabs/SecretNetwork/x/registration"
// 	ra "github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation"
// 	"github.com/stretchr/testify/require"
// )

// //
// ////

// func TestNewQuerier(t *testing.T) {
// 	tempDir, err := os.MkdirTemp("", "wasm")
// 	require.NoError(t, err)
// 	defer os.RemoveAll(tempDir)
// 	ctx, keeper := CreateTestInput(t, false, tempDir, true)

// 	nodeIdInvalid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// 	nodeIdValid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// 	querier := NewQuerier(keeper) // TODO: Should test NewQuerier() as well

// 	cert, err := os.ReadFile("../../testdata/attestation_cert_sw")
// 	require.NoError(t, err)

// 	regInfo := types.RegistrationNodeInfo{
// 		Certificate:   cert,
// 		EncryptedSeed: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
// 	}

// 	keeper.SetRegistrationInfo(ctx, regInfo)

// 	publicKey, err := ra.VerifyRaCert(regInfo.Certificate)
// 	if err != nil {
// 		return
// 	}

// 	expectedSecretParams, _ := json.Marshal(types.GenesisState{
// 		Registration:      nil,
// 		NodeExchMasterKey: &types.MasterKey{Bytes: publicKey},
// 		IoMasterKey:       &types.MasterKey{Bytes: publicKey},
// 	})

// 	specs := map[string]struct {
// 		srcPath []string
// 		srcReq  abci.RequestQuery
// 		// smart queries return raw bytes from contract not []types.Model
// 		// if this is set, then we just compare - (should be json encoded string)
// 		// if success and expSmartRes is not set, we parse into []types.Model and compare
// 		expErr *errorsmod.Error
// 		result string
// 	}{
// 		"query malformed node id": {
// 			[]string{QueryEncryptedSeed, nodeIdInvalid},
// 			abci.RequestQuery{Data: []byte("")},
// 			sdkErrors.ErrInvalidAddress,
// 			"",
// 		},
// 		"query invalid node id": {
// 			[]string{QueryEncryptedSeed, nodeIdValid},
// 			abci.RequestQuery{Data: []byte("")},
// 			sdkErrors.ErrUnknownAddress,
// 			"",
// 		},
// 		"query valid node id": {
// 			[]string{QueryEncryptedSeed, hex.EncodeToString(publicKey)},
// 			abci.RequestQuery{Data: []byte("")},
// 			nil,
// 			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
// 		},
// 		"query master key fail": {
// 			[]string{QueryMasterKey},
// 			abci.RequestQuery{Data: []byte("")},
// 			sdkErrors.ErrUnknownAddress,
// 			"",
// 		},
// 	}

// 	for msg, spec := range specs {
// 		t.Run(msg, func(t *testing.T) {
// 			binResult, err := querier.EncryptedSeed()(ctx, spec.srcPath, spec.srcReq)
// 			require.True(t, spec.expErr.Is(err), err)

// 			if spec.result != "" {
// 				require.Equal(t, spec.result, string(binResult))
// 			}
// 		})
// 	}

// 	keeper.SetMasterKey(ctx, types.MasterKey{Bytes: publicKey}, types.MasterNodeKeyId)
// 	keeper.SetMasterKey(ctx, types.MasterKey{Bytes: publicKey}, types.MasterIoKeyId)

// 	binResult, err := querier(ctx, []string{QueryMasterKey}, abci.RequestQuery{Data: []byte("")})
// 	require.NoError(t, err)
// 	require.Equal(t, string(binResult), string(expectedSecretParams))
// }
