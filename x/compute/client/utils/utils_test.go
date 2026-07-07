package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func getTestData(t *testing.T) ([]byte, []byte, []byte) {
	t.Helper()

	wasmCode := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
	someRandomStr := []byte("hello world")
	gzipData, err := GzipIt(wasmCode)
	require.NoError(t, err)

	return wasmCode, someRandomStr, gzipData
}

func TestIsWasm(t *testing.T) {
	wasmCode, someRandomStr, gzipData := getTestData(t)

	t.Log("should return false for empty data")
	require.False(t, IsWasm(nil))
	t.Log("should return false for short data")
	require.False(t, IsWasm([]byte{0x00, 0x61, 0x73}))
	t.Log("should return false for some random string data")
	require.False(t, IsWasm(someRandomStr))
	t.Log("should return false for gzip data")
	require.False(t, IsWasm(gzipData))
	t.Log("should return true for exact wasm")
	require.True(t, IsWasm(wasmCode))
}

func TestIsGzip(t *testing.T) {
	wasmCode, someRandomStr, gzipData := getTestData(t)

	require.False(t, IsGzip(nil))
	require.False(t, IsGzip([]byte{0x1F, 0x8B}))
	require.False(t, IsGzip(wasmCode))
	require.False(t, IsGzip(someRandomStr))
	require.True(t, IsGzip(gzipData))
}

func TestGzipIt(t *testing.T) {
	wasmCode, someRandomStr, _ := getTestData(t)
	originalGzipData := []byte{
		31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 202, 72, 205, 201, 201, 87, 40, 207, 47, 202, 73, 1,
		4, 0, 0, 255, 255, 133, 17, 74, 13, 11, 0, 0, 0,
	}

	t.Log("gzip wasm with no error")
	_, err := GzipIt(wasmCode)
	require.NoError(t, err)

	t.Log("gzip of a string should return exact gzip data")
	strToGzip, err := GzipIt(someRandomStr)

	require.True(t, IsGzip(strToGzip))
	require.NoError(t, err)
	require.Equal(t, originalGzipData, strToGzip)
}
