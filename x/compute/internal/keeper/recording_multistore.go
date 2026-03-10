package keeper

import (
	"io"

	storetypes "cosmossdk.io/store/types"
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/api"
)

// RecordingMultiStore wraps a MultiStore and intercepts GetKVStore,
// returning RecordingKVStore wrappers that record writes as CrossModuleOps
// on the EcallRecorder.
//
// All stores are recorded EXCEPT those listed in excludedKeys (typically
// the compute module's own store, which is recorded inside the enclave).
type RecordingMultiStore struct {
	storetypes.MultiStore
	recorder     *api.EcallRecorder
	excludedKeys map[string]bool // store key names to exclude from recording
}

// NewRecordingMultiStore creates a recording wrapper around the given MultiStore.
// excludedKeys are store keys whose writes should NOT be recorded (e.g. the compute store).
func NewRecordingMultiStore(
	inner storetypes.MultiStore,
	recorder *api.EcallRecorder,
	excludedKeys []storetypes.StoreKey,
) *RecordingMultiStore {
	ek := make(map[string]bool, len(excludedKeys))
	for _, sk := range excludedKeys {
		ek[sk.Name()] = true
	}
	return &RecordingMultiStore{
		MultiStore:   inner,
		recorder:     recorder,
		excludedKeys: ek,
	}
}

func (rms *RecordingMultiStore) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	inner := rms.MultiStore.GetKVStore(key)
	if rms.excludedKeys[key.Name()] {
		return inner
	}
	return &RecordingKVStore{
		KVStore:  inner,
		storeKey: key.Name(),
		recorder: rms.recorder,
	}
}

func (rms *RecordingMultiStore) GetStore(key storetypes.StoreKey) storetypes.Store {
	return rms.MultiStore.GetStore(key)
}

func (rms *RecordingMultiStore) CacheMultiStore() storetypes.CacheMultiStore {
	return rms.MultiStore.CacheMultiStore()
}

func (rms *RecordingMultiStore) CacheMultiStoreWithVersion(version int64) (storetypes.CacheMultiStore, error) {
	return rms.MultiStore.CacheMultiStoreWithVersion(version)
}

func (rms *RecordingMultiStore) CacheWrap() storetypes.CacheWrap {
	return rms.MultiStore.CacheWrap()
}

func (rms *RecordingMultiStore) CacheWrapWithTrace(w io.Writer, tc storetypes.TraceContext) storetypes.CacheWrap {
	return rms.MultiStore.CacheWrapWithTrace(w, tc)
}

func (rms *RecordingMultiStore) GetStoreType() storetypes.StoreType {
	return rms.MultiStore.GetStoreType()
}

func (rms *RecordingMultiStore) TracingEnabled() bool {
	return rms.MultiStore.TracingEnabled()
}

func (rms *RecordingMultiStore) SetTracer(w io.Writer) storetypes.MultiStore {
	rms.MultiStore.SetTracer(w)
	return rms
}

func (rms *RecordingMultiStore) SetTracingContext(tc storetypes.TraceContext) storetypes.MultiStore {
	rms.MultiStore.SetTracingContext(tc)
	return rms
}

func (rms *RecordingMultiStore) LatestVersion() int64 {
	return rms.MultiStore.LatestVersion()
}

// RecordingKVStore wraps a KVStore and records all Set/Delete operations as
// CrossModuleOps on the EcallRecorder.
type RecordingKVStore struct {
	storetypes.KVStore
	storeKey string
	recorder *api.EcallRecorder
}

func (rks *RecordingKVStore) Set(key, value []byte) {
	rks.KVStore.Set(key, value)
	rks.recorder.AppendCrossModuleOp(api.CrossModuleOp{
		StoreKey: rks.storeKey,
		Key:      cloneBytes(key),
		Value:    cloneBytes(value),
		IsDelete: false,
	})
}

func (rks *RecordingKVStore) Delete(key []byte) {
	rks.KVStore.Delete(key)
	rks.recorder.AppendCrossModuleOp(api.CrossModuleOp{
		StoreKey: rks.storeKey,
		Key:      cloneBytes(key),
		IsDelete: true,
	})
}

// Read-through methods (no recording needed)
func (rks *RecordingKVStore) Get(key []byte) []byte {
	return rks.KVStore.Get(key)
}

func (rks *RecordingKVStore) Has(key []byte) bool {
	return rks.KVStore.Has(key)
}

func (rks *RecordingKVStore) Iterator(start, end []byte) storetypes.Iterator {
	return rks.KVStore.Iterator(start, end)
}

func (rks *RecordingKVStore) ReverseIterator(start, end []byte) storetypes.Iterator {
	return rks.KVStore.ReverseIterator(start, end)
}

func (rks *RecordingKVStore) GetStoreType() storetypes.StoreType {
	return rks.KVStore.GetStoreType()
}

func (rks *RecordingKVStore) CacheWrap() storetypes.CacheWrap {
	return rks.KVStore.CacheWrap()
}

func (rks *RecordingKVStore) CacheWrapWithTrace(w io.Writer, tc storetypes.TraceContext) storetypes.CacheWrap {
	return rks.KVStore.CacheWrapWithTrace(w, tc)
}

// cloneBytes returns a copy of a byte slice, or nil if the input is nil.
func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// ApplyCrossModuleOps replays recorded cross-module mutations on the real multistore.
// Called by the keeper after wasmer.Execute() in replay mode to apply mutations that
// query handlers made to other modules' stores during the SGX execution (e.g.,
// distribution reward withdrawals triggered by staking queries).
func ApplyCrossModuleOps(ms storetypes.MultiStore, ops []api.CrossModuleOp) {
	for _, op := range ops {
		store := ms.GetKVStore(storetypes.NewKVStoreKey(op.StoreKey))
		if op.IsDelete {
			store.Delete(op.Key)
		} else {
			store.Set(op.Key, op.Value)
		}
	}
}
