package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// NodeMode determines how the node handles SGX enclave calls
type NodeMode string

const (
	// NodeModeSGX - Run with real SGX enclave and record outputs
	NodeModeSGX NodeMode = "sgx"
	// NodeModeReplay - Replay recorded outputs without SGX
	NodeModeReplay NodeMode = "replay"
)

// EcallRecorder handles recording and replaying ecall data
type EcallRecorder struct {
	mu        sync.RWMutex
	mode      NodeMode
	recordDir string
}

var (
	globalRecorder *EcallRecorder
	recorderOnce   sync.Once
)

// GetRecorder returns the global ecall recorder instance
func GetRecorder() *EcallRecorder {
	recorderOnce.Do(func() {
		mode := NodeMode(os.Getenv("SECRET_NODE_MODE"))
		if mode == "" {
			mode = NodeModeSGX // Default to SGX mode
		}

		recordDir := os.Getenv("SECRET_ECALL_RECORD_DIR")
		if recordDir == "" {
			recordDir = "/tmp/secret_ecall_records"
		}

		globalRecorder = &EcallRecorder{
			mode:      mode,
			recordDir: recordDir,
		}

		// Create record directory if it doesn't exist
		if err := os.MkdirAll(recordDir, 0o755); err != nil {
			fmt.Printf("Warning: could not create ecall record directory: %v\n", err)
		}

		fmt.Printf("[EcallRecorder] Initialized in %s mode, record dir: %s\n", mode, recordDir)
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

// computeHash computes SHA256 hash of input for filename
func computeHash(operation string, input []byte) string {
	h := sha256.New()
	h.Write([]byte(operation))
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))
}

// getFilePath returns the file path for a given operation and hash
func (r *EcallRecorder) getFilePath(operation string, hash string) string {
	return filepath.Join(r.recordDir, fmt.Sprintf("%s_%s.bin", operation, hash[:16]))
}

// Record stores an ecall output to file (used in SGX mode)
func (r *EcallRecorder) Record(operation string, input []byte, output []byte, err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	hash := computeHash(operation, input)
	filePath := r.getFilePath(operation, hash)

	// If there was an error, write empty file with .err extension
	if err != nil {
		errPath := filePath + ".err"
		if writeErr := os.WriteFile(errPath, []byte(err.Error()), 0o644); writeErr != nil {
			return fmt.Errorf("failed to write error file: %w", writeErr)
		}
		fmt.Printf("[EcallRecorder] Recorded error to %s\n", errPath)
		return nil
	}

	// Write output bytes directly to file
	if writeErr := os.WriteFile(filePath, output, 0o644); writeErr != nil {
		return fmt.Errorf("failed to write record file: %w", writeErr)
	}

	fmt.Printf("[EcallRecorder] Recorded %d bytes to %s\n", len(output), filePath)
	return nil
}

// Replay retrieves a recorded ecall output from file (used in replay mode)
func (r *EcallRecorder) Replay(operation string, input []byte) ([]byte, error, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hash := computeHash(operation, input)
	filePath := r.getFilePath(operation, hash)

	// Check for error file first
	errPath := filePath + ".err"
	if errData, err := os.ReadFile(errPath); err == nil {
		fmt.Printf("[EcallRecorder] Replayed error from %s\n", errPath)
		return nil, fmt.Errorf("%s", string(errData)), true
	}

	// Read output file
	output, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, false // Not found
		}
		return nil, fmt.Errorf("failed to read record file: %w", err), true
	}

	fmt.Printf("[EcallRecorder] Replayed %d bytes from %s\n", len(output), filePath)
	return output, nil, true
}

// --- Wrapper functions for specific ecalls ---

// RecordGetEncryptedSeed records the GetEncryptedSeed ecall
func RecordGetEncryptedSeed(cert []byte, output []byte, err error) error {
	return GetRecorder().Record("GetEncryptedSeed", cert, output, err)
}

// ReplayGetEncryptedSeed attempts to replay a recorded GetEncryptedSeed call
func ReplayGetEncryptedSeed(cert []byte) ([]byte, error, bool) {
	return GetRecorder().Replay("GetEncryptedSeed", cert)
}
