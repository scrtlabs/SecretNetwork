package keeper

import (
	"os"
	"testing"

	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitGenesisNoMaster(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keeper := CreateTestInput(t, false, tempDir, true)

	data := types.GenesisState{
		Registration:      nil,
		IoMasterKey:       nil,
		NodeExchMasterKey: nil,
	}

	assert.Panics(t, func() { InitGenesis(ctx, keeper, data) }, "Init genesis didn't panic without master certificate")
}

func TestInitGenesis(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keeper := CreateTestInput(t, false, tempDir, true)

	cert, err := os.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	key, err := FetchRawPubKeyFromLegacyCert(cert)
	require.NoError(t, err)

	data := types.GenesisState{
		Registration:      nil,
		IoMasterKey:       &types.MasterKey{Bytes: key},
		NodeExchMasterKey: &types.MasterKey{Bytes: key},
	}

	InitGenesis(ctx, keeper, data)
}

func TestExportGenesis(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wasm")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx, keeper := CreateTestInput(t, false, tempDir, true)

	cert, err := os.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	key, err := FetchRawPubKeyFromLegacyCert(cert)
	require.NoError(t, err)

	data := types.GenesisState{
		Registration:      nil,
		IoMasterKey:       &types.MasterKey{Bytes: key},
		NodeExchMasterKey: &types.MasterKey{Bytes: key},
	}

	InitGenesis(ctx, keeper, data)

	data2 := ExportGenesis(ctx, keeper)

	require.Equal(t, string(data.IoMasterKey.Bytes), string(data2.IoMasterKey.Bytes))
	require.Equal(t, string(data.NodeExchMasterKey.Bytes), string(data2.NodeExchMasterKey.Bytes))
	require.Equal(t, data2.Registration, data2.Registration)
}
