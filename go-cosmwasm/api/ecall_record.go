//go:build !secretcli
// +build !secretcli

package api

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

	// Config
	retentionBlocks int64
	pruneInterval   int64

	// Block-scoped execution tracking
	currentBlockHeight int64
	executionIndex     int64

	// In-memory cache for current block's traces (replay mode)
	blockTracesMu sync.RWMutex
	blockTraces   map[int64]*ExecutionTrace // key: execution index

	// Pending cross-module ops collected during the current execution.
	// Set by keeper (via SetPendingCrossModuleOps) before wasmer.Execute,
	// consumed by lib.go (via GetAndClearPendingCrossModuleOps) when building the trace.
	pendingCrossOpsMu sync.Mutex
	pendingCrossOps   []CrossModuleOp
}

var (
	globalRecorder *EcallRecorder
	recorderMu     sync.Mutex
)

// Key prefixes for different ecall types
var (
	prefixSubmitBlockSignatures = []byte{0x01}
	prefixGetEncryptedSeed      = []byte{0x02}
	prefixExecutionTrace        = []byte{0x03} // For contract execution: prefix | height | index
	prefixMachineIDProof        = []byte{0x04} // For MachineID approval: prefix | height | machineID
	prefixCreateResult          = []byte{0x05} // For Create (store code): prefix | height | sha256(wasm)
	prefixGetEncryptedSeedErr   = []byte{0x06} // For GetEncryptedSeed errors: prefix | certHash
	prefixGetNetworkPubkey      = []byte{0x07} // For GetNetworkPubkey: prefix | height | i_seed
)

// CrossModuleOp represents a write to a module store other than the contract's
// own prefixed store. These happen as side-effects of Go querier callbacks
// (e.g. distribution's initializeDelegation during DelegationTotalRewards).
type CrossModuleOp struct {
	StoreKey string // Module store key name (e.g. "distribution")
	Key      []byte
	Value    []byte // nil means delete
	IsDelete bool
}

// ExecutionTrace stores all storage operations from a contract execution
type ExecutionTrace struct {
	Index       int64 // Execution index within the block
	Ops         []StorageOp
	CrossOps    []CrossModuleOp // Cross-module mutations from query side-effects
	Result      []byte          // The return value from the ecall
	GasUsed     uint64          // Gas reported by the enclave
	CallbackGas uint64          // Total gas consumed by callbacks (store ops) during execution
	HasError    bool
	IsOutOfGas  bool // True when the enclave returned errno==2 (OutOfGas)
	ErrorMsg    string
}

// DefaultRetentionBlocks is the default number of blocks to retain (~90 days at 6s blocks)
const DefaultRetentionBlocks int64 = 1296000

// PruneIntervalBlocks defines how often to run pruning (every 100 blocks)
const PruneIntervalBlocks int64 = 100

// GetRecorder returns the global ecall recorder instance
func GetRecorder() *EcallRecorder {
	recorderMu.Lock()
	defer recorderMu.Unlock()

	mode := NodeMode(os.Getenv("SECRET_NODE_MODE"))
	if mode == "" {
		mode = NodeModeSGX // Default to SGX mode
	}

	// Check if storing SGX data is enabled (from config or env var)
	storeSGXData := os.Getenv("SECRET_STORE_SGX_DATA") == "true"

	// Get retention settings
	retentionBlocks := DefaultRetentionBlocks
	if v := os.Getenv("SECRET_SGX_DATA_RETENTION_BLOCKS"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			retentionBlocks = parsed
		}
	}

	pruneInterval := PruneIntervalBlocks
	if v := os.Getenv("SECRET_SGX_DATA_PRUNE_INTERVAL"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			pruneInterval = parsed
		}
	}

	// If already initialized, check if we need to upgrade the recording state
	if globalRecorder != nil {
		hasDB := globalRecorder.db != nil
		if mode == NodeModeReplay || storeSGXData == hasDB {
			// Update config dynamically if re-initialized
			globalRecorder.mu.Lock()
			globalRecorder.retentionBlocks = retentionBlocks
			globalRecorder.pruneInterval = pruneInterval
			globalRecorder.mu.Unlock()
			return globalRecorder
		}
		// Transitioning states (tempApp didn't have flag, but real app does)
		if globalRecorder.db != nil {
			globalRecorder.db.Close()
		}
	}

	if mode == NodeModeReplay || (mode == NodeModeSGX && !storeSGXData) {
		globalRecorder = &EcallRecorder{
			mode:            mode,
			db:              nil,
			retentionBlocks: retentionBlocks,
			pruneInterval:   pruneInterval,
			blockTraces:     make(map[int64]*ExecutionTrace),
		}
		// if mode == NodeModeReplay {
		// 	logInfo("EcallRecorder", "Initialized in replay mode (no local DB, fetches from remote)")
		// } else {
		// 	logInfo("EcallRecorder", "Initialized in %s mode (storing disabled)", mode)
		// }
		return globalRecorder
	}

	// Get database directory from env or use default (SGX mode with storing enabled only)
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
		logWarn("EcallRecorder", "Could not create db directory: %v", err)
	}

	// Open LevelDB database
	db, err := dbm.NewDB("ecall_records", dbm.GoLevelDBBackend, dbDir)
	if err != nil {
		logError("EcallRecorder", "Error opening database: %v", err)
		// Create a nil recorder that will skip recording
		globalRecorder = &EcallRecorder{
			mode:            mode,
			db:              nil,
			retentionBlocks: retentionBlocks,
			pruneInterval:   pruneInterval,
			blockTraces:     make(map[int64]*ExecutionTrace),
		}
		return globalRecorder
	}

	globalRecorder = &EcallRecorder{
		mode:            mode,
		db:              db,
		retentionBlocks: retentionBlocks,
		pruneInterval:   pruneInterval,
		blockTraces:     make(map[int64]*ExecutionTrace),
	}

	// if storeSGXData {
	// 	logInfo("EcallRecorder", "Initialized in %s mode with storing enabled, db dir: %s", mode, dbDir)
	// } else {
	// 	logInfo("EcallRecorder", "Initialized in %s mode, db dir: %s", mode, dbDir)
	// }

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
		logDebug("SetBlockTraces", "Stored trace at index=%d", trace.Index)
	}
}

// GetTraceFromMemory retrieves a trace from the in-memory cache
func (r *EcallRecorder) GetTraceFromMemory(index int64) (*ExecutionTrace, bool) {
	r.blockTracesMu.RLock()
	defer r.blockTracesMu.RUnlock()

	trace, found := r.blockTraces[index]
	return trace, found
}

// SetPendingCrossModuleOps replaces the pending cross-module ops list.
// Called by the keeper to initialize the list before a WASM execution.
func (r *EcallRecorder) SetPendingCrossModuleOps(ops []CrossModuleOp) {
	r.pendingCrossOpsMu.Lock()
	defer r.pendingCrossOpsMu.Unlock()
	r.pendingCrossOps = ops
}

// AppendCrossModuleOp adds a single cross-module op to the pending list.
// Called by the RecordingMultiStore when a cross-module write is observed.
func (r *EcallRecorder) AppendCrossModuleOp(op CrossModuleOp) {
	r.pendingCrossOpsMu.Lock()
	defer r.pendingCrossOpsMu.Unlock()
	r.pendingCrossOps = append(r.pendingCrossOps, op)
}

// GetAndClearPendingCrossModuleOps returns the accumulated cross-module ops
// and clears the pending list. Called by lib.go after wasmer.Execute to
// include the ops in the execution trace.
func (r *EcallRecorder) GetAndClearPendingCrossModuleOps() []CrossModuleOp {
	r.pendingCrossOpsMu.Lock()
	defer r.pendingCrossOpsMu.Unlock()
	ops := r.pendingCrossOps
	r.pendingCrossOps = nil
	return ops
}

// Mode returns the current node mode
func (r *EcallRecorder) Mode() NodeMode {
	return r.mode
}

func (r *EcallRecorder) IsSGXMode() bool {
	return r.mode == NodeModeSGX && r.db != nil
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
		// Storing is disabled (opt-in feature) - silently skip
		return nil
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
	// if height%1000 == 0 {
	// 	logInfo("EcallRecorder", "Recorded SubmitBlockSignatures for height %d", height)
	// }
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
	// if height%1000 == 0 {
	// 	logInfo("EcallRecorder", "Replayed SubmitBlockSignatures for height %d", height)
	// }
	return random, evidence, true
}

// --- MachineID proof recording ---

// makeMachineIDProofKey creates a key: prefix | height (8 bytes) | machineID
func makeMachineIDProofKey(height int64, machineID []byte) []byte {
	key := make([]byte, len(prefixMachineIDProof)+8+len(machineID))
	copy(key, prefixMachineIDProof)
	binary.BigEndian.PutUint64(key[len(prefixMachineIDProof):], uint64(height))
	copy(key[len(prefixMachineIDProof)+8:], machineID)
	return key
}

// RecordMachineIDProof records the proof output from OnApproveMachineID
func (r *EcallRecorder) RecordMachineIDProof(height int64, machineID []byte, proof []byte) error {
	if r.db == nil {
		// Storing is disabled - silently skip
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeMachineIDProofKey(height, machineID)
	if err := r.db.Set(key, proof); err != nil {
		return fmt.Errorf("failed to write machine ID proof to db: %w", err)
	}

	// logInfo("EcallRecorder", "Recorded MachineIDProof for height %d, machineID len=%d", height, len(machineID))
	return nil
}

// ReplayMachineIDProof retrieves the recorded proof for a machine ID approval
func (r *EcallRecorder) ReplayMachineIDProof(height int64, machineID []byte) (proof []byte, found bool) {
	if r.db == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeMachineIDProofKey(height, machineID)
	value, err := r.db.Get(key)
	if err != nil || value == nil {
		return nil, false
	}

	// logInfo("EcallRecorder", "Replayed MachineIDProof for height %d (%d bytes)", height, len(value))
	return value, true
}

// --- GetNetworkPubkey recording ---

func makeNetworkPubkeyKey(height int64, iSeed uint32) []byte {
	key := make([]byte, 1+8+4)
	key[0] = prefixGetNetworkPubkey[0]
	binary.BigEndian.PutUint64(key[1:9], uint64(height))
	binary.BigEndian.PutUint32(key[9:13], iSeed)
	return key
}

func (r *EcallRecorder) RecordGetNetworkPubkey(height int64, iSeed uint32, nodePk, ioPk []byte) error {
	if r.db == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Pack lengths + data
	value := make([]byte, 2+len(nodePk)+2+len(ioPk))
	binary.BigEndian.PutUint16(value[0:2], uint16(len(nodePk)))
	copy(value[2:2+len(nodePk)], nodePk)

	offset := 2 + len(nodePk)
	binary.BigEndian.PutUint16(value[offset:offset+2], uint16(len(ioPk)))
	copy(value[offset+2:], ioPk)

	key := makeNetworkPubkeyKey(height, iSeed)
	if err := r.db.Set(key, value); err != nil {
		return fmt.Errorf("failed to write network pubkey to db: %w", err)
	}

	// logInfo("EcallRecorder", "Recorded GetNetworkPubkey at height %d for i_seed %d", height, iSeed)
	return nil
}

func (r *EcallRecorder) ReplayGetNetworkPubkey(height int64, iSeed uint32) (nodePk, ioPk []byte, found bool) {
	if r.db == nil {
		return nil, nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeNetworkPubkeyKey(height, iSeed)
	value, err := r.db.Get(key)
	if err != nil || value == nil {
		return nil, nil, false
	}

	nodePkLen := binary.BigEndian.Uint16(value[0:2])
	nodePk = make([]byte, nodePkLen)
	copy(nodePk, value[2:2+nodePkLen])

	offset := 2 + nodePkLen
	ioPkLen := binary.BigEndian.Uint16(value[offset : offset+2])
	ioPk = make([]byte, ioPkLen)
	copy(ioPk, value[offset+2:])

	// logInfo("EcallRecorder", "Replayed GetNetworkPubkey at height %d for i_seed %d", height, iSeed)
	return nodePk, ioPk, true
}

// --- GetEncryptedSeed recording (by height + cert hash) ---

// makeSeedKey creates a key: prefix(1) | height(8) | certHash
func makeSeedKey(height int64, certHash []byte) []byte {
	key := make([]byte, 1+8+len(certHash))
	key[0] = prefixGetEncryptedSeed[0]
	binary.BigEndian.PutUint64(key[1:9], uint64(height))
	copy(key[9:], certHash)
	return key
}

// makeSeedErrKey creates a key: prefix(1) | height(8) | certHash
func makeSeedErrKey(height int64, certHash []byte) []byte {
	key := make([]byte, 1+8+len(certHash))
	key[0] = prefixGetEncryptedSeedErr[0]
	binary.BigEndian.PutUint64(key[1:9], uint64(height))
	copy(key[9:], certHash)
	return key
}

// RecordGetEncryptedSeed records the GetEncryptedSeed ecall output (success case)
// Key format: prefix(1) | height(8) | certHash
func (r *EcallRecorder) RecordGetEncryptedSeed(height int64, certHash []byte, outp1 []byte, outp2 []byte) error {
	if r.db == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeSeedKey(height, certHash)
	if err := r.db.Set(key, outp1); err != nil {
		return fmt.Errorf("failed to write to db: %w", err)
	}

	key2 := append(key, 0x01)
	if err := r.db.Set(key2, outp2); err != nil {
		return fmt.Errorf("failed to write to db: %w", err)
	}

	// logInfo("EcallRecorder", "Recorded GetEncryptedSeed success at height %d (%d bytes)", height, len(outp1))
	return nil
}

// RecordGetEncryptedSeedError records a failed GetEncryptedSeed ecall with its error message
// Key format: prefix(1) | height(8) | certHash
func (r *EcallRecorder) RecordGetEncryptedSeedError(height int64, certHash []byte, errMsg string) error {
	if r.db == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeSeedErrKey(height, certHash)
	if err := r.db.Set(key, []byte(errMsg)); err != nil {
		return fmt.Errorf("failed to write error to db: %w", err)
	}

	// logInfo("EcallRecorder", "Recorded GetEncryptedSeed error at height %d: %s", height, errMsg)
	return nil
}

// ReplayGetEncryptedSeed retrieves recorded GetEncryptedSeed data for a given height
// Returns (output, "", true) on recorded success, (nil, errMsg, true) on recorded error, (nil, "", false) if not found
func (r *EcallRecorder) ReplayGetEncryptedSeed(height int64, certHash []byte) (outp1 []byte, outp2 []byte, errMsg string, found bool) {
	if r.db == nil {
		return nil, nil, "", false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check for error entry first (separate key prefix)
	errKey := makeSeedErrKey(height, certHash)
	errVal, err := r.db.Get(errKey)
	if err == nil && errVal != nil && len(errVal) > 0 {
		// logInfo("EcallRecorder", "Replayed GetEncryptedSeed error at height %d", height)
		return nil, nil, string(errVal), true
	}

	// Check for success entry
	key := makeSeedKey(height, certHash)
	outp1, err = r.db.Get(key)
	if err != nil || outp1 == nil {
		return nil, nil, "", false
	}

	key2 := append(key, 0x01)
	outp2, err = r.db.Get(key2)
	if err != nil || outp2 == nil {
		return nil, nil, "", false
	}

	// logInfo("EcallRecorder", "Replayed GetEncryptedSeed success at height %d (%d bytes)", height, len(outp1))
	return outp1, outp2, "", true
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
		// Storing is disabled (opt-in feature) - silently skip
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	trace.Index = index

	logDebug("RecordExecutionTrace", "Storing trace height=%d index=%d callbackGas=%d", height, index, trace.CallbackGas)

	// Convert to protobuf and serialize
	protoTrace := executionTraceToProto(trace)
	data, err := proto.Marshal(protoTrace)
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}

	logDebug("RecordExecutionTrace", "Serialized data length=%d", len(data))

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
			logDebug("RecordExecutionTrace", "Verified readback callbackGas=%d", verifyTrace.CallbackGas)
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
		logError("EcallRecorder", "Failed to deserialize trace: %v", err)
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
		logDebug("GetAllTracesForBlock", "Raw trace data length=%d", len(rawData))

		// Deserialize protobuf
		var protoTrace ExecutionTraceProto
		if err := proto.Unmarshal(rawData, &protoTrace); err != nil {
			logError("EcallRecorder", "Failed to deserialize trace: %v", err)
			continue
		}

		trace := protoToExecutionTrace(&protoTrace)

		logDebug("GetAllTracesForBlock", "Deserialized trace index=%d callbackGas=%d gasUsed=%d ops=%d",
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
	crossOps := make([]*CrossModuleOpProto, len(trace.CrossOps))
	for i, cop := range trace.CrossOps {
		crossOps[i] = &CrossModuleOpProto{
			StoreKey: cop.StoreKey,
			Key:      cop.Key,
			Value:    cop.Value,
			IsDelete: cop.IsDelete,
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
		CrossOps:    crossOps,
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
	crossOps := make([]CrossModuleOp, len(proto.CrossOps))
	for i, cop := range proto.CrossOps {
		crossOps[i] = CrossModuleOp{
			StoreKey: cop.StoreKey,
			Key:      cop.Key,
			Value:    cop.Value,
			IsDelete: cop.IsDelete,
		}
	}
	return &ExecutionTrace{
		Index:       proto.Index,
		Ops:         ops,
		CrossOps:    crossOps,
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

	count := 0

	// Prune all height-keyed prefixes
	for _, prefix := range [][]byte{
		prefixSubmitBlockSignatures,
		prefixGetEncryptedSeed,
		prefixGetEncryptedSeedErr,
		prefixExecutionTrace,
		prefixMachineIDProof,
		prefixCreateResult,
		prefixGetNetworkPubkey,
	} {
		startKey := makeBlockKey(prefix, 0)
		endKey := makeBlockKey(prefix, height)

		iter, err := r.db.Iterator(startKey, endKey)
		if err != nil {
			return err
		}

		for ; iter.Valid(); iter.Next() {
			if err := batch.Delete(iter.Key()); err != nil {
				iter.Close()
				return err
			}
			count++
		}
		iter.Close()
	}

	if err := batch.Write(); err != nil {
		return err
	}

	if count > 0 {
		// logInfo("EcallRecorder", "Pruned %d records before height %d", count, height)
	}
	return nil
}

// PruneOldRecords runs pruning if conditions are met (every pruneInterval)
func (r *EcallRecorder) PruneOldRecords(currentHeight int64) {
	// Only prune in SGX mode (non-replay)
	if r.IsReplayMode() {
		return
	}

	r.mu.RLock()
	interval := r.pruneInterval
	retention := r.retentionBlocks
	r.mu.RUnlock()

	// Only prune every pruneInterval
	if interval <= 0 || currentHeight%interval != 0 {
		return
	}

	// Calculate cutoff height
	cutoffHeight := currentHeight - retention
	if cutoffHeight <= 0 {
		return
	}

	// Run pruning in background to not block block processing
	go func() {
		if err := r.DeleteRecordsBeforeHeight(cutoffHeight); err != nil {
			logError("EcallRecorder", "Pruning error: %v", err)
		}
	}()
}

// --- Create result recording ---

// CreateResult stores the outcome of an SGX Create call
type CreateResult struct {
	CodeHash []byte // The hash returned by the enclave (empty on error)
	HasError bool
	ErrorMsg string
}

// makeCreateResultKey creates a key: prefix | height (8 bytes) | wasmHash (32 bytes)
func makeCreateResultKey(height int64, wasmHash []byte) []byte {
	key := make([]byte, len(prefixCreateResult)+8+len(wasmHash))
	copy(key, prefixCreateResult)
	binary.BigEndian.PutUint64(key[len(prefixCreateResult):], uint64(height))
	copy(key[len(prefixCreateResult)+8:], wasmHash)
	return key
}

// RecordCreateResult records the result of an SGX Create call
func (r *EcallRecorder) RecordCreateResult(height int64, wasmHash []byte, codeHash []byte, errMsg string) error {
	if r.db == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	result := &CreateResult{
		CodeHash: codeHash,
		HasError: errMsg != "",
		ErrorMsg: errMsg,
	}

	// Simple encoding: hasError (1 byte) | errMsgLen (4 bytes) | errMsg | codeHash
	var value []byte
	if result.HasError {
		errBytes := []byte(result.ErrorMsg)
		value = make([]byte, 1+4+len(errBytes))
		value[0] = 1
		binary.BigEndian.PutUint32(value[1:5], uint32(len(errBytes)))
		copy(value[5:], errBytes)
	} else {
		value = make([]byte, 1+len(codeHash))
		value[0] = 0
		copy(value[1:], codeHash)
	}

	key := makeCreateResultKey(height, wasmHash)
	if err := r.db.Set(key, value); err != nil {
		return fmt.Errorf("failed to write Create result to db: %w", err)
	}

	// logInfo("EcallRecorder", "Recorded Create result for height %d, hasError=%v", height, result.HasError)
	return nil
}

// ReplayCreateResult retrieves the recorded Create result
func (r *EcallRecorder) ReplayCreateResult(height int64, wasmHash []byte) (codeHash []byte, errMsg string, found bool) {
	if r.db == nil {
		return nil, "", false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeCreateResultKey(height, wasmHash)
	value, err := r.db.Get(key)
	if err != nil || value == nil {
		return nil, "", false
	}

	if len(value) < 1 {
		return nil, "", false
	}

	hasError := value[0] == 1
	if hasError {
		if len(value) < 5 {
			return nil, "", false
		}
		errLen := binary.BigEndian.Uint32(value[1:5])
		if len(value) < 5+int(errLen) {
			return nil, "", false
		}
		return nil, string(value[5 : 5+errLen]), true
	}

	return value[1:], "", true
}

// GetAllCreateResultsForBlock returns all Create results recorded for a given block height
func (r *EcallRecorder) GetAllCreateResultsForBlock(height int64) ([]*CreateResult, [][]byte, error) {
	if r.db == nil {
		return nil, nil, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Build prefix for iteration: prefix | height
	iterPrefix := make([]byte, len(prefixCreateResult)+8)
	copy(iterPrefix, prefixCreateResult)
	binary.BigEndian.PutUint64(iterPrefix[len(prefixCreateResult):], uint64(height))

	var results []*CreateResult
	var wasmHashes [][]byte

	// Build end key: increment the last byte of the height to get next height prefix
	// IMPORTANT: Must copy to avoid mutating iterPrefix via shared backing array
	iterEnd := make([]byte, len(iterPrefix))
	copy(iterEnd, iterPrefix)
	iterEnd[len(iterEnd)-1]++

	iter, err := r.db.Iterator(iterPrefix, iterEnd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Extract wasmHash from key (after prefix + height)
		wasmHash := key[len(prefixCreateResult)+8:]
		wasmHashCopy := make([]byte, len(wasmHash))
		copy(wasmHashCopy, wasmHash)
		wasmHashes = append(wasmHashes, wasmHashCopy)

		result := &CreateResult{}
		if len(value) >= 1 && value[0] == 1 {
			result.HasError = true
			if len(value) >= 5 {
				errLen := binary.BigEndian.Uint32(value[1:5])
				if len(value) >= 5+int(errLen) {
					result.ErrorMsg = string(value[5 : 5+errLen])
				}
			}
		} else if len(value) >= 1 {
			result.CodeHash = make([]byte, len(value)-1)
			copy(result.CodeHash, value[1:])
		}
		results = append(results, result)
	}

	return results, wasmHashes, nil
}
