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

	// In-memory cache for current block's streams (replay mode)
	blockStreamsMu sync.RWMutex
	blockStreams   map[int64][]byte // key: execution index, value: raw stream bytes
}

var (
	globalRecorder *EcallRecorder
	recorderMu     sync.Mutex
)

// Key prefixes for different ecall types
var (
	prefixSubmitBlockSignatures = []byte{0x01}
	prefixGetEncryptedSeed      = []byte{0x02}
	prefixEcallStream           = []byte{0x04} // For ecall streams: prefix | height | index
)

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

	// If already initialized, check if we need to upgrade the recording state
	if globalRecorder != nil {
		hasDB := globalRecorder.db != nil
		if mode == NodeModeReplay || storeSGXData == hasDB {
			return globalRecorder
		}
		// Transitioning states (tempApp didn't have flag, but real app does)
		if globalRecorder.db != nil {
			globalRecorder.db.Close()
		}
	}

	if mode == NodeModeReplay || (mode == NodeModeSGX && !storeSGXData) {
		globalRecorder = &EcallRecorder{
			mode:         mode,
			db:           nil,
			blockStreams: make(map[int64][]byte),
		}
		if mode == NodeModeReplay {
			logInfo("EcallRecorder", "Initialized in replay mode (no local DB, fetches from remote)")
		} else {
			logInfo("EcallRecorder", "Initialized in %s mode (storing disabled)", mode)
		}
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
			mode:         mode,
			db:           nil,
			blockStreams: make(map[int64][]byte),
		}
		return globalRecorder
	}

	globalRecorder = &EcallRecorder{
		mode:         mode,
		db:           db,
		blockStreams: make(map[int64][]byte),
	}

	if storeSGXData {
		logInfo("EcallRecorder", "Initialized in %s mode with storing enabled, db dir: %s", mode, dbDir)
	} else {
		logInfo("EcallRecorder", "Initialized in %s mode, db dir: %s", mode, dbDir)
	}

	return globalRecorder
}

// --- Block-scoped execution tracking ---

// StartBlock initializes tracking for a new block, resetting the execution counter
func (r *EcallRecorder) StartBlock(height int64) {
	atomic.StoreInt64(&r.currentBlockHeight, height)
	atomic.StoreInt64(&r.executionIndex, 0)

	// Clear previous block's streams from memory
	r.blockStreamsMu.Lock()
	r.blockStreams = make(map[int64][]byte)
	r.blockStreamsMu.Unlock()
}

// NextExecutionIndex returns the next execution index and increments the counter
func (r *EcallRecorder) NextExecutionIndex() int64 {
	return atomic.AddInt64(&r.executionIndex, 1)
}

// GetCurrentBlockHeight returns the current block height being processed
func (r *EcallRecorder) GetCurrentBlockHeight() int64 {
	return atomic.LoadInt64(&r.currentBlockHeight)
}

// SetBlockStreams sets all streams for the current block (used in replay mode after batch fetch)
func (r *EcallRecorder) SetBlockStreams(streams map[int64][]byte) {
	r.blockStreamsMu.Lock()
	defer r.blockStreamsMu.Unlock()

	r.blockStreams = streams
	for idx := range streams {
		logDebug("SetBlockStreams", "Stored stream at index=%d len=%d", idx, len(streams[idx]))
	}
}

// GetStreamFromMemory retrieves a stream from the in-memory cache
func (r *EcallRecorder) GetStreamFromMemory(index int64) ([]byte, bool) {
	r.blockStreamsMu.RLock()
	defer r.blockStreamsMu.RUnlock()

	streamBytes, found := r.blockStreams[index]
	return streamBytes, found
}

// --- Ecall stream recording / retrieval ---

// makeStreamKey creates a key for height+index indexed ecall streams
// Key format: prefix (1 byte) | height (8 bytes) | index (8 bytes)
func makeStreamKey(height int64, index int64) []byte {
	key := make([]byte, 1+8+8)
	key[0] = prefixEcallStream[0]
	binary.BigEndian.PutUint64(key[1:9], uint64(height))
	binary.BigEndian.PutUint64(key[9:17], uint64(index))
	return key
}

// RecordEcallStream records raw ecall stream bytes by height and execution index.
// In SGX mode, this persists the stream to LevelDB for serving to non-SGX nodes.
func (r *EcallRecorder) RecordEcallStream(height int64, index int64, streamBytes []byte) error {
	if r.db == nil {
		// Storing is disabled (opt-in feature) - silently skip
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeStreamKey(height, index)
	if err := r.db.Set(key, streamBytes); err != nil {
		return fmt.Errorf("failed to write ecall stream to db: %w", err)
	}

	logDebug("RecordEcallStream", "Stored stream height=%d index=%d len=%d", height, index, len(streamBytes))
	return nil
}

// GetAllStreamsForBlock retrieves all ecall streams for a given block height.
// Used by the gRPC server to serve streams to non-SGX nodes.
func (r *EcallRecorder) GetAllStreamsForBlock(height int64) (map[int64][]byte, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	startKey := makeStreamKey(height, 0)
	endKey := makeStreamKey(height+1, 0)

	iter, err := r.db.Iterator(startKey, endKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	streams := make(map[int64][]byte)
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) == 17 {
			idx := int64(binary.BigEndian.Uint64(key[9:17]))
			value := iter.Value()
			streamCopy := make([]byte, len(value))
			copy(streamCopy, value)
			streams[idx] = streamCopy
		}
	}

	return streams, nil
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

// --- GetEncryptedSeed recording (by cert hash) ---

// RecordGetEncryptedSeed records the GetEncryptedSeed ecall output
func (r *EcallRecorder) RecordGetEncryptedSeed(certHash []byte, output []byte) error {
	if r.db == nil {
		// Storing is disabled (opt-in feature) - silently skip
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := append(prefixGetEncryptedSeed, certHash...)
	if err := r.db.Set(key, output); err != nil {
		return fmt.Errorf("failed to write to db: %w", err)
	}

	logInfo("EcallRecorder", "Recorded GetEncryptedSeed (%d bytes)", len(output))
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

	logInfo("EcallRecorder", "Replayed GetEncryptedSeed (%d bytes)", len(value))
	return value, true
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
		logInfo("EcallRecorder", "Pruned %d records before height %d", count, height)
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
			logError("EcallRecorder", "Pruning error: %v", err)
		}
	}()
}
