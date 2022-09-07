package keeper

import (
	"io/ioutil"
	"testing"

	"github.com/enigmampc/SecretNetwork/x/registration/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitGenesisNoMaster(t *testing.T) {
	tempDir := t.TempDir()
	ctx, keeper := CreateTestInput(t, false, tempDir, true)
	//
	//cert, err := ioutil.ReadFile("../../testdata/attestation_cert")
	//require.NoError(t, err)

	data := types.GenesisState{
		Registration:              nil,
		IoMasterCertificate:       nil,
		NodeExchMasterCertificate: nil,
	}

	assert.Panics(t, func() { InitGenesis(ctx, keeper, data) }, "Init genesis didn't panic without master certificate")
}

func TestInitGenesis(t *testing.T) {
	tempDir := t.TempDir()
	ctx, keeper := CreateTestInput(t, false, tempDir, true)

	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	data := types.GenesisState{
		Registration:              nil,
		IoMasterCertificate:       &types.MasterCertificate{Bytes: cert},
		NodeExchMasterCertificate: &types.MasterCertificate{Bytes: cert},
	}

	InitGenesis(ctx, keeper, data)
}

func TestExportGenesis(t *testing.T) {
	tempDir := t.TempDir()
	ctx, keeper := CreateTestInput(t, false, tempDir, true)

	cert, err := ioutil.ReadFile("../../testdata/attestation_cert_sw")
	require.NoError(t, err)

	data := types.GenesisState{
		Registration:              nil,
		IoMasterCertificate:       &types.MasterCertificate{Bytes: cert},
		NodeExchMasterCertificate: &types.MasterCertificate{Bytes: cert},
	}

	InitGenesis(ctx, keeper, data)

	data2 := ExportGenesis(ctx, keeper)

	require.Equal(t, string(data.IoMasterCertificate.Bytes), string(data2.IoMasterCertificate.Bytes))
	require.Equal(t, string(data.NodeExchMasterCertificate.Bytes), string(data2.NodeExchMasterCertificate.Bytes))
	require.Equal(t, data2.Registration, data2.Registration)
}
