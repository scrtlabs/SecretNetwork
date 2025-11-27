//go:build !secretcli
// +build !secretcli

package api

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"

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
}

var (
	globalRecorder *EcallRecorder
	recorderOnce   sync.Once
)

// Key prefixes for different ecall types
var (
	prefixSubmitBlockSignatures = []byte{0x01}
	prefixGetEncryptedSeed      = []byte{0x02}
)

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
			mode: mode,
			db:   db,
		}

		fmt.Printf("[EcallRecorder] Initialized in %s mode, db dir: %s\n", mode, dbDir)
	})
	return globalRecorder
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
