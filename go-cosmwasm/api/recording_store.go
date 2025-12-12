//go:build !secretcli
// +build !secretcli

package api

import (
	dbm "github.com/cosmos/cosmos-db"
)

// StorageOp represents a single storage operation (Set or Delete)
type StorageOp struct {
	Key      []byte
	Value    []byte // nil for delete
	IsDelete bool
}

// RecordingKVStore wraps a KVStore and records all Set/Delete operations
type RecordingKVStore struct {
	inner KVStore
	ops   []StorageOp
}

// NewRecordingKVStore creates a new recording wrapper around a KVStore
func NewRecordingKVStore(inner KVStore) *RecordingKVStore {
	return &RecordingKVStore{
		inner: inner,
		ops:   nil,
	}
}

// Get delegates to the inner store (no recording needed for state)
func (r *RecordingKVStore) Get(key []byte) []byte {
	return r.inner.Get(key)
}

// Set records the operation and delegates to the inner store
func (r *RecordingKVStore) Set(key, value []byte) {
	// Copy key and value to avoid issues with reused buffers
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	r.ops = append(r.ops, StorageOp{
		IsDelete: false,
		Key:      keyCopy,
		Value:    valueCopy,
	})
	r.inner.Set(key, value)
}

// Delete records the operation and delegates to the inner store
func (r *RecordingKVStore) Delete(key []byte) {
	// Copy key to avoid issues with reused buffers
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)

	r.ops = append(r.ops, StorageOp{
		IsDelete: true,
		Key:      keyCopy,
		Value:    nil,
	})
	r.inner.Delete(key)
}

// Iterator delegates to the inner store
func (r *RecordingKVStore) Iterator(start, end []byte) dbm.Iterator {
	return r.inner.Iterator(start, end)
}

// ReverseIterator delegates to the inner store
func (r *RecordingKVStore) ReverseIterator(start, end []byte) dbm.Iterator {
	return r.inner.ReverseIterator(start, end)
}

// GetOps returns all recorded operations
func (r *RecordingKVStore) GetOps() []StorageOp {
	return r.ops
}

// ClearOps clears recorded operations
func (r *RecordingKVStore) ClearOps() {
	r.ops = nil
}

// ReplayingKVStore applies recorded operations to a store without calling the enclave
type ReplayingKVStore struct {
	inner KVStore
}

// NewReplayingKVStore creates a store that can replay operations
func NewReplayingKVStore(inner KVStore) *ReplayingKVStore {
	return &ReplayingKVStore{inner: inner}
}

// ApplyOps applies a list of storage operations to the store
func (r *ReplayingKVStore) ApplyOps(ops []StorageOp) {
	for _, op := range ops {
		if op.IsDelete {
			r.inner.Delete(op.Key)
		} else {
			r.inner.Set(op.Key, op.Value)
		}
	}
}

// Get delegates to the inner store
func (r *ReplayingKVStore) Get(key []byte) []byte {
	return r.inner.Get(key)
}

// Set delegates to the inner store
func (r *ReplayingKVStore) Set(key, value []byte) {
	r.inner.Set(key, value)
}

// Delete delegates to the inner store
func (r *ReplayingKVStore) Delete(key []byte) {
	r.inner.Delete(key)
}

// Iterator delegates to the inner store
func (r *ReplayingKVStore) Iterator(start, end []byte) dbm.Iterator {
	return r.inner.Iterator(start, end)
}

// ReverseIterator delegates to the inner store
func (r *ReplayingKVStore) ReverseIterator(start, end []byte) dbm.Iterator {
	return r.inner.ReverseIterator(start, end)
}
