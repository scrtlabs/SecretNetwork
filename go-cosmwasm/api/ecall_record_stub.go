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

// Encrypted seed stubs
func (r *EcallRecorder) RecordGetEncryptedSeed(certHash []byte, output []byte) error {
	return nil
}

func (r *EcallRecorder) ReplayGetEncryptedSeed(certHash []byte) (output []byte, found bool) {
	return nil, false
}

// Utility stubs
func (r *EcallRecorder) HasRecordForHeight(height int64) bool  { return false }
func (r *EcallRecorder) GetLatestRecordedHeight() int64        { return 0 }
func (r *EcallRecorder) DeleteRecordsBeforeHeight(int64) error { return nil }

// Block-scoped execution tracking stubs
func (r *EcallRecorder) StartBlock(height int64)      {}
func (r *EcallRecorder) NextExecutionIndex() int64    { return 0 }
func (r *EcallRecorder) GetCurrentBlockHeight() int64 { return 0 }

// Stream-based methods (new)
func (r *EcallRecorder) SetBlockStreams(streams map[int64][]byte)       {}
func (r *EcallRecorder) GetStreamFromMemory(index int64) ([]byte, bool) { return nil, false }
func (r *EcallRecorder) RecordEcallStream(height int64, index int64, streamBytes []byte) error {
	return nil
}

func (r *EcallRecorder) GetAllStreamsForBlock(height int64) (map[int64][]byte, error) {
	return nil, nil
}
