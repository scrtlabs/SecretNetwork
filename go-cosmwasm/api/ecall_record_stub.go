//go:build secretcli
// +build secretcli

package api

// Stub implementations for secretcli builds (no SGX support)

type NodeMode string

const (
	NodeModeSGX    NodeMode = "sgx"
	NodeModeReplay NodeMode = "replay"
)

// StorageOp represents a single storage operation (Set or Delete)
type StorageOp struct {
	Key      []byte
	Value    []byte
	IsDelete bool
}

// CrossModuleOp represents a write to a module store other than the contract's own prefixed store.
type CrossModuleOp struct {
	StoreKey string
	Key      []byte
	Value    []byte
	IsDelete bool
}

// ExecutionTrace stores all storage operations from a contract execution
type ExecutionTrace struct {
	Index       int64
	Ops         []StorageOp
	CrossOps    []CrossModuleOp
	Result      []byte
	GasUsed     uint64
	CallbackGas uint64
	HasError    bool
	IsOutOfGas  bool
	ErrorMsg    string
}

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

func (r *EcallRecorder) RecordGetEncryptedSeed(height int64, certHash []byte, output []byte) error {
	return nil
}

func (r *EcallRecorder) RecordGetEncryptedSeedError(height int64, certHash []byte, errMsg string) error {
	return nil
}

func (r *EcallRecorder) ReplayGetEncryptedSeed(height int64, certHash []byte) (output []byte, errMsg string, found bool) {
	return nil, "", false
}

func (r *EcallRecorder) RecordMachineIDProof(height int64, machineID []byte, proof []byte) error {
	return nil
}

func (r *EcallRecorder) ReplayMachineIDProof(height int64, machineID []byte) (proof []byte, found bool) {
	return nil, false
}

func (r *EcallRecorder) RecordExecutionTrace(height int64, index int64, trace *ExecutionTrace) error {
	return nil
}

func (r *EcallRecorder) ReplayExecutionTrace(height int64, index int64) (*ExecutionTrace, bool) {
	return nil, false
}

func (r *EcallRecorder) GetAllTracesForBlock(height int64) ([]*ExecutionTrace, error) {
	return nil, nil
}

// Block-scoped execution tracking stubs
func (r *EcallRecorder) StartBlock(height int64)                                {}
func (r *EcallRecorder) NextExecutionIndex() int64                              { return 0 }
func (r *EcallRecorder) GetCurrentBlockHeight() int64                           { return 0 }
func (r *EcallRecorder) SetBlockTraces(traces []*ExecutionTrace)                {}
func (r *EcallRecorder) GetTraceFromMemory(index int64) (*ExecutionTrace, bool) { return nil, false }

// Cross-module ops stubs
func (r *EcallRecorder) SetPendingCrossModuleOps(ops []CrossModuleOp)      {}
func (r *EcallRecorder) AppendCrossModuleOp(op CrossModuleOp)              {}
func (r *EcallRecorder) GetAndClearPendingCrossModuleOps() []CrossModuleOp { return nil }

// CreateResult stores the outcome of an SGX Create call
type CreateResult struct {
	CodeHash []byte
	HasError bool
	ErrorMsg string
}

// Create result recording stubs
func (r *EcallRecorder) RecordCreateResult(height int64, wasmHash []byte, codeHash []byte, errMsg string) error {
	return nil
}

func (r *EcallRecorder) ReplayCreateResult(height int64, wasmHash []byte) (codeHash []byte, errMsg string, found bool) {
	return nil, "", false
}

func (r *EcallRecorder) GetAllCreateResultsForBlock(height int64) ([]*CreateResult, [][]byte, error) {
	return nil, nil, nil
}

func (r *EcallRecorder) RecordGetNetworkPubkey(height int64, iSeed uint32, nodePk, ioPk []byte) error {
	return nil
}

func (r *EcallRecorder) ReplayGetNetworkPubkey(height int64, iSeed uint32) ([]byte, []byte, bool) {
	return nil, nil, false
}
