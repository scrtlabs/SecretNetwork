//go:build secretcli

package api

// Stub types for secretcli build — these are never used at runtime

type (
	OcallStreamWriter struct{}
	OcallStreamReader struct{}
)

type EcallResult struct {
	Result   []byte
	GasUsed  uint64
	HasError bool
	ErrorMsg string
}
