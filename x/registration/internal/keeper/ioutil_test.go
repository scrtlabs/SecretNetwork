package keeper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFile(t *testing.T) {
	_, err := getFile("./testdata/contract.wasm")
	require.NoError(t, err)
}

func TestFileExists(t *testing.T) {
	val := fileExists("./testdata/contract.wasm")
	assert.Equal(t, val, true)

	val = fileExists("./testdata/contractXYZ.wasm")
	assert.Equal(t, val, false)
}
