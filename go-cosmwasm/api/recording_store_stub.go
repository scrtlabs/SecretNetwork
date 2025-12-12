//go:build secretcli
// +build secretcli

package api

// Stub implementations for secretcli builds (no SGX support)

// RecordingKVStore stub for secretcli
type RecordingKVStore struct{}

// NewRecordingKVStore stub
func NewRecordingKVStore(inner interface{}) *RecordingKVStore {
	return nil
}

func (r *RecordingKVStore) GetOps() []StorageOp {
	return nil
}

// ReplayingKVStore stub for secretcli
type ReplayingKVStore struct{}

// NewReplayingKVStore stub
func NewReplayingKVStore(inner interface{}) *ReplayingKVStore {
	return nil
}

func (r *ReplayingKVStore) ApplyOps(ops []StorageOp) {}
