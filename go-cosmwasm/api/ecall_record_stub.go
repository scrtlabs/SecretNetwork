//go:build secretcli
// +build secretcli

package api

// Stub implementations for secretcli builds (no SGX support)

type NodeMode string

const (
	NodeModeSGX    NodeMode = "sgx"
	NodeModeReplay NodeMode = "replay"
)

// EcallRecorder stub for secretcli
type EcallRecorder struct {
	mode NodeMode
}

var globalRecorder *EcallRecorder

// GetRecorder returns a stub recorder for secretcli builds
func GetRecorder() *EcallRecorder {
	if globalRecorder == nil {
		globalRecorder = &EcallRecorder{mode: NodeModeSGX}
	}
	return globalRecorder
}

func (r *EcallRecorder) Mode() NodeMode        { return r.mode }
func (r *EcallRecorder) IsSGXMode() bool       { return r.mode == NodeModeSGX }
func (r *EcallRecorder) IsReplayMode() bool    { return r.mode == NodeModeReplay }
func (r *EcallRecorder) Close() error          { return nil }
func (r *EcallRecorder) PruneOldRecords(int64) {}

func (r *EcallRecorder) RecordSubmitBlockSignatures(height int64, random []byte, evidence []byte) error {
	return nil
}

func (r *EcallRecorder) ReplaySubmitBlockSignatures(height int64) (random []byte, evidence []byte, found bool) {
	return nil, nil, false
}

func (r *EcallRecorder) HasRecordForHeight(height int64) bool {
	return false
}

func (r *EcallRecorder) GetLatestRecordedHeight() int64 {
	return 0
}

func (r *EcallRecorder) DeleteRecordsBeforeHeight(height int64) error {
	return nil
}

func (r *EcallRecorder) RecordGetEncryptedSeed(certHash []byte, output []byte) error {
	return nil
}

func (r *EcallRecorder) ReplayGetEncryptedSeed(certHash []byte) (output []byte, found bool) {
	return nil, false
}
