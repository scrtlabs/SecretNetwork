//go:build !secretcli
// +build !secretcli

package api

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/gogo/protobuf/proto"
)

// NodeMode determines how the node handles SGX enclave calls
type NodeMode string

const (
	// NodeModeSGX - Run with real SGX enclave and record outputs
	NodeModeSGX NodeMode = "sgx"
	// NodeModeReplay - Replay recorded outputs without SGX
	NodeModeReplay NodeMode = "replay"
)

// EcallRecorder handles recording and replaying ecall data using LevelDB
type EcallRecorder struct {
	mu   sync.RWMutex
	mode NodeMode
	db   dbm.DB

	// Block-scoped execution tracking
	currentBlockHeight int64
	executionIndex     int64

	// In-memory cache for current block's traces (replay mode)
	blockTracesMu sync.RWMutex
	blockTraces   map[int64]*ExecutionTrace // key: execution index
}

var (
	globalRecorder *EcallRecorder
	recorderOnce   sync.Once
)

// Key prefixes for different ecall types
var (
	prefixSubmitBlockSignatures = []byte{0x01}
	prefixGetEncryptedSeed      = []byte{0x02}
	prefixExecutionTrace        = []byte{0x03} // For contract execution: prefix | height | index
)

// ExecutionTrace stores all storage operations from a contract execution
type ExecutionTrace struct {
	Index       int64 // Execution index within the block
	Ops         []StorageOp
	Result      []byte // The return value from the ecall
	GasUsed     uint64 // Gas reported by the enclave
	CallbackGas uint64 // Total gas consumed by callbacks (store ops) during execution
	HasError    bool
	ErrorMsg    string
}

// DefaultRetentionBlocks is the default number of blocks to retain (~1 day at 6s blocks)
const DefaultRetentionBlocks int64 = 14400

// PruneIntervalBlocks defines how often to run pruning (every 100 blocks)
const PruneIntervalBlocks int64 = 100

// GetRecorder returns the global ecall recorder instance
func GetRecorder() *EcallRecorder {
	recorderOnce.Do(func() {
		mode := NodeMode(os.Getenv("SECRET_NODE_MODE"))
		if mode == "" {
			mode = NodeModeSGX // Default to SGX mode
		}

		// Get database directory from env or use default
		dbDir := os.Getenv("SECRET_ECALL_RECORD_DIR")
		if dbDir == "" {
			// Default to ~/.secretd/data/
			homeDir := os.Getenv("HOME")
			secretHome := os.Getenv("SECRETD_HOME")
			if secretHome != "" {
				dbDir = filepath.Join(secretHome, "data")
			} else {
				dbDir = filepath.Join(homeDir, ".secretd", "data")
			}
		}

		// Ensure directory exists
		if err := os.MkdirAll(dbDir, 0o755); err != nil {
			fmt.Printf("[EcallRecorder] Warning: could not create db directory: %v\n", err)
		}

		// Open LevelDB database
		db, err := dbm.NewDB("ecall_records", dbm.GoLevelDBBackend, dbDir)
		if err != nil {
			fmt.Printf("[EcallRecorder] Error opening database: %v\n", err)
			// Create a nil recorder that will skip recording
			globalRecorder = &EcallRecorder{
				mode: mode,
				db:   nil,
			}
			return
		}

		globalRecorder = &EcallRecorder{
			mode:        mode,
			db:          db,
			blockTraces: make(map[int64]*ExecutionTrace),
		}

		fmt.Printf("[EcallRecorder] Initialized in %s mode, db dir: %s\n", mode, dbDir)
	})
	return globalRecorder
}

// --- Block-scoped execution tracking ---

// StartBlock initializes tracking for a new block, resetting the execution counter
func (r *EcallRecorder) StartBlock(height int64) {
	atomic.StoreInt64(&r.currentBlockHeight, height)
	atomic.StoreInt64(&r.executionIndex, 0)

	// Clear previous block's traces from memory
	r.blockTracesMu.Lock()
	r.blockTraces = make(map[int64]*ExecutionTrace)
	r.blockTracesMu.Unlock()
}

// NextExecutionIndex returns the next execution index and increments the counter
func (r *EcallRecorder) NextExecutionIndex() int64 {
	return atomic.AddInt64(&r.executionIndex, 1)
}

// GetCurrentBlockHeight returns the current block height being processed
func (r *EcallRecorder) GetCurrentBlockHeight() int64 {
	return atomic.LoadInt64(&r.currentBlockHeight)
}

// SetBlockTraces sets all traces for the current block (used in replay mode after batch fetch)
func (r *EcallRecorder) SetBlockTraces(traces []*ExecutionTrace) {
	r.blockTracesMu.Lock()
	defer r.blockTracesMu.Unlock()

	r.blockTraces = make(map[int64]*ExecutionTrace)
	for _, trace := range traces {
		r.blockTraces[trace.Index] = trace
		fmt.Printf("[SetBlockTraces] Stored trace at index=%d\n", trace.Index)
	}
}

// GetTraceFromMemory retrieves a trace from the in-memory cache
func (r *EcallRecorder) GetTraceFromMemory(index int64) (*ExecutionTrace, bool) {
	r.blockTracesMu.RLock()
	defer r.blockTracesMu.RUnlock()

	trace, found := r.blockTraces[index]
	return trace, found
}

// Mode returns the current node mode
func (r *EcallRecorder) Mode() NodeMode {
	return r.mode
}

// IsSGXMode returns true if running in SGX mode
func (r *EcallRecorder) IsSGXMode() bool {
	return r.mode == NodeModeSGX
}

// IsReplayMode returns true if running in replay mode
func (r *EcallRecorder) IsReplayMode() bool {
	return r.mode == NodeModeReplay
}

// Close closes the database
func (r *EcallRecorder) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// --- Key generation helpers ---

// makeBlockKey creates a key for block-height indexed data
func makeBlockKey(prefix []byte, height int64) []byte {
	key := make([]byte, len(prefix)+8)
	copy(key, prefix)
	binary.BigEndian.PutUint64(key[len(prefix):], uint64(height))
	return key
}

// --- SubmitBlockSignatures recording ---

// RecordSubmitBlockSignatures records the output of SubmitBlockSignatures by block height
func (r *EcallRecorder) RecordSubmitBlockSignatures(height int64, random []byte, evidence []byte) error {
	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Combine random (32 bytes) and evidence (32 bytes) into 64 bytes
	value := make([]byte, 64)
	copy(value[:32], random)
	copy(value[32:], evidence)

	key := makeBlockKey(prefixSubmitBlockSignatures, height)
	if err := r.db.Set(key, value); err != nil {
		return fmt.Errorf("failed to write to db: %w", err)
	}

	// Only log every 1000 blocks to reduce noise
	if height%1000 == 0 {
		fmt.Printf("[EcallRecorder] Recorded SubmitBlockSignatures for height %d\n", height)
	}
	return nil
}

// ReplaySubmitBlockSignatures retrieves recorded SubmitBlockSignatures data by block height
func (r *EcallRecorder) ReplaySubmitBlockSignatures(height int64) (random []byte, evidence []byte, found bool) {
	if r.db == nil {
		return nil, nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeBlockKey(prefixSubmitBlockSignatures, height)
	value, err := r.db.Get(key)
	if err != nil || value == nil || len(value) != 64 {
		return nil, nil, false
	}

	random = make([]byte, 32)
	evidence = make([]byte, 32)
	copy(random, value[:32])
	copy(evidence, value[32:])

	// Only log every 1000 blocks to reduce noise
	if height%1000 == 0 {
		fmt.Printf("[EcallRecorder] Replayed SubmitBlockSignatures for height %d\n", height)
	}
	return random, evidence, true
}

// --- GetEncryptedSeed recording (by cert hash) ---

// RecordGetEncryptedSeed records the GetEncryptedSeed ecall output
func (r *EcallRecorder) RecordGetEncryptedSeed(certHash []byte, output []byte) error {
	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := append(prefixGetEncryptedSeed, certHash...)
	if err := r.db.Set(key, output); err != nil {
		return fmt.Errorf("failed to write to db: %w", err)
	}

	fmt.Printf("[EcallRecorder] Recorded GetEncryptedSeed (%d bytes)\n", len(output))
	return nil
}

// ReplayGetEncryptedSeed retrieves recorded GetEncryptedSeed data
func (r *EcallRecorder) ReplayGetEncryptedSeed(certHash []byte) (output []byte, found bool) {
	if r.db == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := append(prefixGetEncryptedSeed, certHash...)
	value, err := r.db.Get(key)
	if err != nil || value == nil {
		return nil, false
	}

	fmt.Printf("[EcallRecorder] Replayed GetEncryptedSeed (%d bytes)\n", len(value))
	return value, true
}

// --- ExecutionTrace recording (for contract executions) ---

// makeExecutionKey creates a key for height+index indexed execution traces
// Key format: prefix (1 byte) | height (8 bytes) | index (8 bytes)
func makeExecutionKey(height int64, index int64) []byte {
	key := make([]byte, 1+8+8)
	key[0] = prefixExecutionTrace[0]
	binary.BigEndian.PutUint64(key[1:9], uint64(height))
	binary.BigEndian.PutUint64(key[9:17], uint64(index))
	return key
}

// RecordExecutionTrace records contract execution storage ops and result
// Uses current block height and the provided execution index
func (r *EcallRecorder) RecordExecutionTrace(height int64, index int64, trace *ExecutionTrace) error {
	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	trace.Index = index

	fmt.Printf("[RecordExecutionTrace] DEBUG: Storing trace height=%d index=%d callbackGas=%d\n", height, index, trace.CallbackGas)

	// Convert to protobuf and serialize
	protoTrace := executionTraceToProto(trace)
	data, err := proto.Marshal(protoTrace)
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}

	fmt.Printf("[RecordExecutionTrace] DEBUG: Serialized data length=%d\n", len(data))

	key := makeExecutionKey(height, index)
	if err := r.db.Set(key, data); err != nil {
		return fmt.Errorf("failed to write to db: %w", err)
	}

	// Verify we can read it back
	readBack, err := r.db.Get(key)
	if err == nil && readBack != nil {
		var verifyProto ExecutionTraceProto
		if err := proto.Unmarshal(readBack, &verifyProto); err == nil {
			verifyTrace := protoToExecutionTrace(&verifyProto)
			fmt.Printf("[RecordExecutionTrace] DEBUG: Verified readback callbackGas=%d\n", verifyTrace.CallbackGas)
		}
	}

	return nil
}

// ReplayExecutionTrace retrieves recorded execution trace by height and index
func (r *EcallRecorder) ReplayExecutionTrace(height int64, index int64) (*ExecutionTrace, bool) {
	if r.db == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeExecutionKey(height, index)
	value, err := r.db.Get(key)
	if err != nil || value == nil {
		return nil, false
	}

	// Deserialize protobuf
	var protoTrace ExecutionTraceProto
	if err := proto.Unmarshal(value, &protoTrace); err != nil {
		fmt.Printf("[EcallRecorder] Failed to deserialize trace: %v\n", err)
		return nil, false
	}

	trace := protoToExecutionTrace(&protoTrace)
	return trace, true
}

// GetAllTracesForBlock retrieves all execution traces for a given block height
func (r *EcallRecorder) GetAllTracesForBlock(height int64) ([]*ExecutionTrace, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create range for this block: [prefix|height|0, prefix|height|maxUint64]
	startKey := makeExecutionKey(height, 0)
	endKey := makeExecutionKey(height+1, 0) // Next block's start is this block's end

	iter, err := r.db.Iterator(startKey, endKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	var traces []*ExecutionTrace
	for ; iter.Valid(); iter.Next() {
		rawData := iter.Value()
		fmt.Printf("[GetAllTracesForBlock] DEBUG: Raw trace data length=%d\n", len(rawData))

		// Deserialize protobuf
		var protoTrace ExecutionTraceProto
		if err := proto.Unmarshal(rawData, &protoTrace); err != nil {
			fmt.Printf("[EcallRecorder] Failed to deserialize trace: %v\n", err)
			continue
		}

		trace := protoToExecutionTrace(&protoTrace)

		fmt.Printf("[GetAllTracesForBlock] DEBUG: Deserialized trace index=%d callbackGas=%d gasUsed=%d ops=%d\n",
			trace.Index, trace.CallbackGas, trace.GasUsed, len(trace.Ops))
		traces = append(traces, trace)
	}

	return traces, nil
}

// executionTraceToProto converts ExecutionTrace to ExecutionTraceProto
func executionTraceToProto(trace *ExecutionTrace) *ExecutionTraceProto {
	ops := make([]*StorageOpProto, len(trace.Ops))
	for i, op := range trace.Ops {
		ops[i] = &StorageOpProto{
			IsDelete: op.IsDelete,
			Key:      op.Key,
			Value:    op.Value,
		}
	}
	return &ExecutionTraceProto{
		Index:       trace.Index,
		Ops:         ops,
		Result:      trace.Result,
		GasUsed:     trace.GasUsed,
		CallbackGas: trace.CallbackGas,
		HasError:    trace.HasError,
		ErrorMsg:    trace.ErrorMsg,
	}
}

// protoToExecutionTrace converts ExecutionTraceProto to ExecutionTrace
func protoToExecutionTrace(proto *ExecutionTraceProto) *ExecutionTrace {
	ops := make([]StorageOp, len(proto.Ops))
	for i, op := range proto.Ops {
		ops[i] = StorageOp{
			IsDelete: op.IsDelete,
			Key:      op.Key,
			Value:    op.Value,
		}
	}
	return &ExecutionTrace{
		Index:       proto.Index,
		Ops:         ops,
		Result:      proto.Result,
		GasUsed:     proto.GasUsed,
		CallbackGas: proto.CallbackGas,
		HasError:    proto.HasError,
		ErrorMsg:    proto.ErrorMsg,
	}
}

// --- Utility functions ---

// HasRecordForHeight checks if a record exists for a given height
func (r *EcallRecorder) HasRecordForHeight(height int64) bool {
	if r.db == nil {
		return false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeBlockKey(prefixSubmitBlockSignatures, height)
	has, err := r.db.Has(key)
	return err == nil && has
}

// GetLatestRecordedHeight returns the highest recorded block height
func (r *EcallRecorder) GetLatestRecordedHeight() int64 {
	if r.db == nil {
		return 0
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Iterate backwards to find the latest height
	iter, err := r.db.ReverseIterator(
		prefixSubmitBlockSignatures,
		append(prefixSubmitBlockSignatures, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF),
	)
	if err != nil {
		return 0
	}
	defer iter.Close()

	if iter.Valid() {
		key := iter.Key()
		if len(key) == 9 { // prefix (1) + height (8)
			return int64(binary.BigEndian.Uint64(key[1:]))
		}
	}
	return 0
}

// DeleteRecordsBeforeHeight removes records older than the given height (for pruning)
func (r *EcallRecorder) DeleteRecordsBeforeHeight(height int64) error {
	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	batch := r.db.NewBatch()
	defer batch.Close()

	startKey := makeBlockKey(prefixSubmitBlockSignatures, 0)
	endKey := makeBlockKey(prefixSubmitBlockSignatures, height)

	iter, err := r.db.Iterator(startKey, endKey)
	if err != nil {
		return err
	}
	defer iter.Close()

	count := 0
	for ; iter.Valid(); iter.Next() {
		if err := batch.Delete(iter.Key()); err != nil {
			return err
		}
		count++
	}

	if err := batch.Write(); err != nil {
		return err
	}

	if count > 0 {
		fmt.Printf("[EcallRecorder] Pruned %d records before height %d\n", count, height)
	}
	return nil
}

// PruneOldRecords runs pruning if conditions are met (every PruneIntervalBlocks)
func (r *EcallRecorder) PruneOldRecords(currentHeight int64) {
	// Only prune in SGX mode (non-replay)
	if r.IsReplayMode() {
		return
	}

	// Only prune every PruneIntervalBlocks
	if currentHeight%PruneIntervalBlocks != 0 {
		return
	}

	// Calculate cutoff height
	cutoffHeight := currentHeight - DefaultRetentionBlocks
	if cutoffHeight <= 0 {
		return
	}

	// Run pruning in background to not block block processing
	go func() {
		if err := r.DeleteRecordsBeforeHeight(cutoffHeight); err != nil {
			fmt.Printf("[EcallRecorder] Pruning error: %v\n", err)
		}
	}()
}
