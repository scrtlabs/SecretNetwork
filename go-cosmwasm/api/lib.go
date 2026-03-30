//go:build !secretcli && !nosgx
// +build !secretcli,!nosgx

package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	v1types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"

	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
)

// nice aliases to the rust names
type i32 = C.int32_t

type (
	i64    = C.int64_t
	u64    = C.uint64_t
	u32    = C.uint32_t
	u8     = C.uint8_t
	u8_ptr = *C.uint8_t
	usize  = C.uintptr_t
	cint   = C.int
	cbool  = C.bool
)

type Cache struct {
	ptr *C.cache_t
}

func HealthCheck() ([]byte, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		return []byte("replay"), nil
	}
	errmsg := C.Buffer{}

	res, err := C.get_health_check(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

func SubmitBlockSignatures(header []byte, commit []byte, txs []byte, encRandom []byte, cronMsgs []byte /* valSet []byte, nextValSet []byte */) ([]byte, []byte, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		return nil, nil, errors.New("submit block signatures not supported on non-SGX node")
	}
	errmsg := C.Buffer{}
	spidSlice := sendSlice(header)
	defer freeAfterSend(spidSlice)
	apiKeySlice := sendSlice(commit)
	defer freeAfterSend(apiKeySlice)
	encRandomSlice := sendSlice(encRandom)
	defer freeAfterSend(encRandomSlice)
	txsSlice := sendSlice(txs)
	defer freeAfterSend(txsSlice)

	res, err := C.submit_block_signatures(spidSlice, apiKeySlice, txsSlice, encRandomSlice /* valSetSlice, nextValSetSlice,*/, &errmsg)
	if err != nil {
		return nil, nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res.buf1), receiveVector(res.buf2), nil
}

func SubmitValidatorSetEvidence(evidence []byte) error {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		logInfo("SubmitValidatorSetEvidence", "Skipped in replay mode")
		return nil
	}
	errmsg := C.Buffer{}
	evidenceSlice := sendSlice(evidence)
	defer freeAfterSend(evidenceSlice)
	C.submit_validator_set_evidence(evidenceSlice, &errmsg)
	return nil
}

func InitBootstrap() ([]byte, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, return a dummy 32-byte public key
		// This function is only called during bootstrap which doesn't happen in replay
		logInfo("InitBootstrap", "Skipped in replay mode")
		return make([]byte, 32), nil
	}
	errmsg := C.Buffer{}
	res, err := C.init_bootstrap(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

func LoadSeedToEnclave(masterKey []byte, seed []byte) (bool, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, skip loading seed to enclave (no enclave)
		logInfo("LoadSeedToEnclave", "Skipped in replay mode")
		return true, nil
	}
	pkSlice := sendSlice(masterKey)
	defer freeAfterSend(pkSlice)
	seedSlice := sendSlice(seed)
	defer freeAfterSend(seedSlice)
	errmsg := C.Buffer{}

	_, err := C.init_node(pkSlice, seedSlice, &errmsg)
	if err != nil {
		return false, errorWithMessage(err, errmsg)
	}
	return true, nil
}

func RotateStore(kvs []byte) (bool, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		logInfo("RotateStore", "Skipped in replay mode")
		return false, errors.New("rotate store not supported on non-SGX node")
	}
	// avoid buffer copy. We need modification in-place
	kvSlice := C.Buffer{
		ptr: (*C.uint8_t)(unsafe.Pointer(&kvs[0])),
		len: C.ulong(len(kvs)),
		cap: C.ulong(cap(kvs)),
	}

	ret, err := C.rotate_store(kvSlice.ptr, u32(kvSlice.len))
	if err != nil {
		return false, err
	}
	if !ret {
		return false, errors.New("sealing migration failed")
	}
	return true, nil
}

func MigrationOp(op uint32) (bool, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		logInfo("MigrationOp", "Skipped in replay mode")
		return true, nil // no-op success so upgrade handlers don't fail
	}
	ret, err := C.migration_op(u32(op))
	if err != nil {
		return false, err
	}
	if !ret {
		return false, errors.New("sealing migration failed")
	}
	return true, nil
}

func GetNetworkPubkey(i_seed uint32) ([]byte, []byte) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()

	if recorder.IsReplayMode() {
		nodePk, ioPk, err := GetEcallClient().FetchNetworkPubkey(height, i_seed)
		if err != nil {
			logError("GetNetworkPubkey", "Failed to fetch on replay: %v", err)
			return nil, nil
		}
		return nodePk, ioPk
	}

	res := C.get_network_pubkey(u32(i_seed))
	nodePk := receiveVector(res.buf1)
	ioPk := receiveVector(res.buf2)

	_ = recorder.RecordGetNetworkPubkey(height, i_seed, nodePk, ioPk)
	return nodePk, ioPk
}

func EmergencyApproveUpgrade(nodeDir string, msg string) (bool, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		return false, errors.New("emergency approve upgrade not supported on non-SGX node")
	}
	nodeDirBuf := sendSlice([]byte(nodeDir))
	defer freeAfterSend(nodeDirBuf)

	msgBuf := sendSlice([]byte(msg))
	defer freeAfterSend(msgBuf)

	ret, err := C.emergency_approve_upgrade(nodeDirBuf, msgBuf)
	if err != nil {
		return false, err
	}
	if !ret {
		return false, errors.New("emergency approve upgrade failed")
	}
	return true, nil
}

type Querier = types.Querier

func InitCache(dataDir string, supportedFeatures string, cacheSize uint64) (Cache, error) {
	dir := sendSlice([]byte(dataDir))
	defer freeAfterSend(dir)
	features := sendSlice([]byte(supportedFeatures))
	defer freeAfterSend(features)
	errmsg := C.Buffer{}

	ptr, err := C.init_cache(dir, features, usize(cacheSize), &errmsg)
	if err != nil {
		return Cache{}, errorWithMessage(err, errmsg)
	}
	return Cache{ptr: ptr}, nil
}

func ReleaseCache(cache Cache) {
	C.release_cache(cache.ptr)
}

func InitEnclaveRuntime(moduleCacheSize uint16) error {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, skip enclave runtime initialization (no enclave)
		logInfo("InitEnclaveRuntime", "Skipped in replay mode")
		return nil
	}

	errmsg := C.Buffer{}

	config := C.EnclaveRuntimeConfig{
		module_cache_size: u32(moduleCacheSize),
	}
	_, err := C.configure_enclave_runtime(config, &errmsg)
	if err != nil {
		err = errorWithMessage(err, errmsg)
		return err
	}
	return nil
}

func OnUpgradeProposalPassed(mrEnclaveHash []byte) error {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		logInfo("OnUpgradeProposalPassed", "Skipped in replay mode")
		return nil
	}
	msgBuf := sendSlice(mrEnclaveHash)
	defer freeAfterSend(msgBuf)

	ret, err := C.onchain_approve_upgrade(msgBuf)
	if err != nil {
		return err
	}
	if !ret {
		return errors.New("onchain_approve_upgrade failed")
	}

	return nil
}

func OnApproveMachineID(machineID []byte) error {
	msgBuf := sendSlice(machineID)
	defer freeAfterSend(msgBuf)

	ret, err := C.onchain_approve_machine_id(msgBuf)
	if err != nil {
		return err
	}
	if !ret {
		return errors.New("onchain_approve_machine_id failed")
	}

	return nil
}

func SubmitMachineSwap(index uint32, machineInfo []byte, proof []byte) error {
	machineInfoBuf := sendSlice(machineInfo)
	defer freeAfterSend(machineInfoBuf)
	proofBuf := sendSlice(proof)
	defer freeAfterSend(proofBuf)

	ret, err := C.submit_machine_swap(u32(index), machineInfoBuf, proofBuf)
	if err != nil {
		return err
	}
	if !ret {
		return errors.New("submit_machine_swap failed")
	}

	return nil
}

func Create(cache Cache, wasm []byte) ([]byte, error) {
	code := sendSlice(wasm)
	defer freeAfterSend(code)
	errmsg := C.Buffer{}
	id, err := C.create(cache.ptr, code, &errmsg)

	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()

	if err != nil {
		createErr := errorWithMessage(err, errmsg)
		// Record the failure so non-SGX nodes can replay it
		wasmHash := sha256.Sum256(wasm)
		if recErr := recorder.RecordCreateResult(height, wasmHash[:], nil, createErr.Error()); recErr != nil {
			logError("Create", "Failed to record Create error: %v", recErr)
		}
		return nil, createErr
	}

	codeHash := receiveVector(id)
	// Record the success
	wasmHash := sha256.Sum256(wasm)
	if recErr := recorder.RecordCreateResult(height, wasmHash[:], codeHash, ""); recErr != nil {
		logError("Create", "Failed to record Create result: %v", recErr)
	}
	return codeHash, nil
}

func GetCode(cache Cache, code_id []byte) ([]byte, error) {
	id := sendSlice(code_id)
	defer freeAfterSend(id)
	errmsg := C.Buffer{}
	code, err := C.get_code(cache.ptr, id, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(code), nil
}

func Migrate(
	cache Cache,
	code_id []byte,
	params []byte,
	msg []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
	sigInfo []byte,
	admin []byte,
	adminProof []byte,
) ([]byte, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	if recorder.IsReplayMode() {
		if result, gas, err, found := replayExecution(store, gasMeter, execIndex); found {
			return result, gas, err
		}
		return nil, 0, fmt.Errorf("Migrate replay failed: trace not found for height %d index %d", height, execIndex)
	}

	recording := recorder.IsSGXMode()
	var recordingStore *RecordingKVStore
	var storeForDB KVStore = store
	if recording {
		recordingStore = NewRecordingKVStore(store)
		storeForDB = recordingStore
	}

	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)
	m := sendSlice(msg)
	defer freeAfterSend(m)

	// set up a new stack frame to handle iterators
	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(storeForDB, counter)
	db := buildDB(&dbState, gasMeter)

	s := sendSlice(sigInfo)
	defer freeAfterSend(s)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed u64
	errmsg := C.Buffer{}

	adminBuffer := sendSlice(admin)
	defer freeAfterSend(adminBuffer)

	adminProofBuffer := sendSlice(adminProof)
	defer freeAfterSend(adminProofBuffer)

	// Capture gas before execution to measure callback gas
	var gasBefore uint64
	if recording && gasMeter != nil {
		gasBefore = (*gasMeter).GasConsumed()
	}

	res, err := C.migrate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, adminBuffer, adminProofBuffer)

	if !recording {
		if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
			return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
		}
		return receiveVector(res), uint64(gasUsed), nil
	}

	// Calculate callback gas consumed during execution
	var callbackGas uint64
	if gasMeter != nil {
		gasAfter := (*gasMeter).GasConsumed()
		callbackGas = gasAfter - gasBefore
	}

	// Record the execution trace
	trace := &ExecutionTrace{
		Index:       execIndex,
		Ops:         recordingStore.GetOps(),
		CrossOps:    recorder.GetAndClearPendingCrossModuleOps(),
		GasUsed:     uint64(gasUsed),
		CallbackGas: callbackGas,
	}

	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		trace.HasError = true
		errorMsgBytes := receiveVector(errmsg)
		trace.ErrorMsg = string(errorMsgBytes)
		if errno, ok := err.(syscall.Errno); ok && int(errno) == 2 {
			trace.IsOutOfGas = true
		}
		if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
			logError("Migrate", "Failed to record trace: %v", recordErr)
		}
		if trace.IsOutOfGas {
			return nil, uint64(gasUsed), types.OutOfGasError{}
		}
		if errorMsgBytes == nil {
			return nil, uint64(gasUsed), err
		}
		return nil, uint64(gasUsed), fmt.Errorf("%s", string(errorMsgBytes))
	}

	trace.Result = receiveVector(res)
	if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
		logError("Migrate", "Failed to record trace: %v", recordErr)
	}

	return trace.Result, uint64(gasUsed), nil
}

func UpdateAdmin(
	cache Cache,
	code_id []byte,
	params []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
	sigInfo []byte,
	currentAdmin []byte,
	currentAdminProof []byte,
	newAdmin []byte,
) ([]byte, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	if recorder.IsReplayMode() {
		if result, _, err, found := replayExecution(store, gasMeter, execIndex); found {
			return result, err
		}
		return nil, fmt.Errorf("UpdateAdmin replay failed: trace not found for height %d index %d", height, execIndex)
	}

	recording := recorder.IsSGXMode()
	var recordingStore *RecordingKVStore
	var storeForDB KVStore = store
	if recording {
		recordingStore = NewRecordingKVStore(store)
		storeForDB = recordingStore
	}

	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)

	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(storeForDB, counter)
	db := buildDB(&dbState, gasMeter)

	s := sendSlice(sigInfo)
	defer freeAfterSend(s)
	a := buildAPI(api)
	q := buildQuerier(querier)
	errmsg := C.Buffer{}

	currentAdminBuffer := sendSlice(currentAdmin)
	defer freeAfterSend(currentAdminBuffer)

	currentAdminProofBuffer := sendSlice(currentAdminProof)
	defer freeAfterSend(currentAdminProofBuffer)

	newAdminBuffer := sendSlice(newAdmin)
	defer freeAfterSend(newAdminBuffer)

	var gasBefore uint64
	if recording && gasMeter != nil {
		gasBefore = (*gasMeter).GasConsumed()
	}

	res, err := C.update_admin(cache.ptr, id, p, db, a, q, u64(gasLimit), &errmsg, s, currentAdminBuffer, currentAdminProofBuffer, newAdminBuffer)

	if !recording {
		if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
			return nil, errorWithMessage(err, errmsg)
		}
		return receiveVector(res), nil
	}

	// Calculate callback gas consumed during execution
	var callbackGas uint64
	if gasMeter != nil {
		gasAfter := (*gasMeter).GasConsumed()
		callbackGas = gasAfter - gasBefore
	}

	// Record the execution trace
	trace := &ExecutionTrace{
		Index:       execIndex,
		Ops:         recordingStore.GetOps(),
		CrossOps:    recorder.GetAndClearPendingCrossModuleOps(),
		GasUsed:     0, // UpdateAdmin doesn't return gas used
		CallbackGas: callbackGas,
	}

	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		trace.HasError = true
		errorMsgBytes := receiveVector(errmsg)
		trace.ErrorMsg = string(errorMsgBytes)
		if errno, ok := err.(syscall.Errno); ok && int(errno) == 2 {
			trace.IsOutOfGas = true
		}
		if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
			logError("UpdateAdmin", "Failed to record trace: %v", recordErr)
		}
		if trace.IsOutOfGas {
			return nil, types.OutOfGasError{}
		}
		if errorMsgBytes == nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", string(errorMsgBytes))
	}

	trace.Result = receiveVector(res)
	if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
		logError("UpdateAdmin", "Failed to record trace: %v", recordErr)
	}

	return trace.Result, nil
}

func Instantiate(
	cache Cache,
	code_id []byte,
	params []byte,
	msg []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
	sigInfo []byte,
	admin []byte,
) ([]byte, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	if recorder.IsReplayMode() {
		logDebug("Instantiate", "REPLAY mode: height=%d execIndex=%d", height, execIndex)
		if result, gas, err, found := replayExecution(store, gasMeter, execIndex); found {
			logDebug("Instantiate", "REPLAY success: resultLen=%d gas=%d err=%v", len(result), gas, err)
			return result, gas, err
		}
		logWarn("Instantiate", "REPLAY FAILED: trace not found!")
		return nil, 0, fmt.Errorf("Instantiate replay failed: trace not found for height %d index %d", height, execIndex)
	}

	recording := recorder.IsSGXMode()
	var recordingStore *RecordingKVStore
	var storeForDB KVStore = store
	if recording {
		recordingStore = NewRecordingKVStore(store)
		storeForDB = recordingStore
	}

	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)
	m := sendSlice(msg)
	defer freeAfterSend(m)

	// set up a new stack frame to handle iterators
	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(storeForDB, counter)
	db := buildDB(&dbState, gasMeter)

	s := sendSlice(sigInfo)
	defer freeAfterSend(s)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed u64
	errmsg := C.Buffer{}

	adminBuffer := sendSlice(admin)
	defer freeAfterSend(adminBuffer)

	// Capture gas before execution to measure callback gas
	var gasBefore uint64
	if recording && gasMeter != nil {
		gasBefore = (*gasMeter).GasConsumed()
		logDebug("Instantiate", "SGX gasBefore=%d", gasBefore)
	}

	res, err := C.instantiate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, adminBuffer)

	if !recording {
		if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
			return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
		}
		return receiveVector(res), uint64(gasUsed), nil
	}

	// Calculate callback gas consumed during execution
	var callbackGas uint64
	if gasMeter != nil {
		gasAfter := (*gasMeter).GasConsumed()
		callbackGas = gasAfter - gasBefore
		logDebug("Instantiate", "SGX gasAfter=%d callbackGas=%d (gasAfter - gasBefore)", gasAfter, callbackGas)
	} else {
		logWarn("Instantiate", "SGX WARNING: gasMeter is nil, cannot measure callbackGas")
	}

	logDebug("Instantiate", "SGX C.instantiate returned: gasUsed=%d callbackGas=%d", gasUsed, callbackGas)

	// Record the execution trace
	trace := &ExecutionTrace{
		Index:       execIndex,
		Ops:         recordingStore.GetOps(),
		CrossOps:    recorder.GetAndClearPendingCrossModuleOps(),
		GasUsed:     uint64(gasUsed),
		CallbackGas: callbackGas,
	}

	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		trace.HasError = true
		errorMsgBytes := receiveVector(errmsg)
		trace.ErrorMsg = string(errorMsgBytes)
		if errno, ok := err.(syscall.Errno); ok && int(errno) == 2 {
			trace.IsOutOfGas = true
		}
		if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
			logError("Instantiate", "Failed to record trace: %v", recordErr)
		}
		if trace.IsOutOfGas {
			return nil, uint64(gasUsed), types.OutOfGasError{}
		}
		if errorMsgBytes == nil {
			return nil, uint64(gasUsed), err
		}
		return nil, uint64(gasUsed), fmt.Errorf("%s", string(errorMsgBytes))
	}

	trace.Result = receiveVector(res)
	if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
		logError("Instantiate", "Failed to record trace: %v", recordErr)
	} else {
		logDebug("Instantiate", "SGX recorded trace: height=%d index=%d ops=%d resultLen=%d gasUsed=%d callbackGas=%d",
			height, execIndex, len(trace.Ops), len(trace.Result), trace.GasUsed, trace.CallbackGas)
	}

	return trace.Result, uint64(gasUsed), nil
}

func Handle(
	cache Cache,
	code_id []byte,
	params []byte,
	msg []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
	sigInfo []byte,
	handleType types.HandleType,
) ([]byte, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	if recorder.IsReplayMode() {
		logDebug("Handle", "REPLAY mode: height=%d execIndex=%d", height, execIndex)
		if result, gas, err, found := replayExecution(store, gasMeter, execIndex); found {
			logDebug("Handle", "REPLAY success: resultLen=%d gas=%d err=%v", len(result), gas, err)
			return result, gas, err
		}
		logWarn("Handle", "REPLAY FAILED: trace not found!")
		return nil, 0, fmt.Errorf("Handle replay failed: trace not found for height %d index %d", height, execIndex)
	}

	recording := recorder.IsSGXMode()
	var recordingStore *RecordingKVStore
	var storeForDB KVStore = store
	if recording {
		recordingStore = NewRecordingKVStore(store)
		storeForDB = recordingStore
	}

	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)
	m := sendSlice(msg)
	defer freeAfterSend(m)

	// set up a new stack frame to handle iterators
	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(storeForDB, counter)
	db := buildDB(&dbState, gasMeter)
	s := sendSlice(sigInfo)
	defer freeAfterSend(s)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed u64
	errmsg := C.Buffer{}

	// Capture gas before execution to measure callback gas
	var gasBefore uint64
	if recording && gasMeter != nil {
		gasBefore = (*gasMeter).GasConsumed()
		logDebug("Handle", "SGX gasBefore=%d", gasBefore)
	}

	res, err := C.handle(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, u8(handleType))

	if !recording {
		if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
			return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
		}
		return receiveVector(res), uint64(gasUsed), nil
	}

	// Calculate callback gas consumed during execution
	var callbackGas uint64
	if gasMeter != nil {
		gasAfter := (*gasMeter).GasConsumed()
		callbackGas = gasAfter - gasBefore
		logDebug("Handle", "SGX gasAfter=%d callbackGas=%d (gasAfter - gasBefore)", gasAfter, callbackGas)
	} else {
		logWarn("Handle", "SGX WARNING: gasMeter is nil, cannot measure callbackGas")
	}

	logDebug("Handle", "SGX C.handle returned: gasUsed=%d callbackGas=%d", gasUsed, callbackGas)

	// Record the execution trace
	trace := &ExecutionTrace{
		Index:       execIndex,
		Ops:         recordingStore.GetOps(),
		CrossOps:    recorder.GetAndClearPendingCrossModuleOps(),
		GasUsed:     uint64(gasUsed),
		CallbackGas: callbackGas,
	}

	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		trace.HasError = true
		errorMsgBytes := receiveVector(errmsg)
		trace.ErrorMsg = string(errorMsgBytes)
		if errno, ok := err.(syscall.Errno); ok && int(errno) == 2 {
			trace.IsOutOfGas = true
		}
		if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
			logError("Handle", "Failed to record trace: %v", recordErr)
		}
		if trace.IsOutOfGas {
			return nil, uint64(gasUsed), types.OutOfGasError{}
		}
		if errorMsgBytes == nil {
			return nil, uint64(gasUsed), err
		}
		return nil, uint64(gasUsed), fmt.Errorf("%s", string(errorMsgBytes))
	}

	trace.Result = receiveVector(res)
	if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
		logError("Handle", "Failed to record trace: %v", recordErr)
	} else {
		logDebug("Handle", "SGX recorded trace: height=%d index=%d ops=%d resultLen=%d gasUsed=%d callbackGas=%d",
			height, execIndex, len(trace.Ops), len(trace.Result), trace.GasUsed, trace.CallbackGas)
	}

	return trace.Result, uint64(gasUsed), nil
}

func Query(
	cache Cache,
	code_id []byte,
	params []byte,
	msg []byte,
	gasMeter *GasMeter,
	store KVStore,
	api *GoAPI,
	querier *Querier,
	gasLimit uint64,
) ([]byte, uint64, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		return nil, 0, errors.New("secret contract query not supported on non-SGX node")
	}
	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)
	m := sendSlice(msg)
	defer freeAfterSend(m)

	// set up a new stack frame to handle iterators
	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(store, counter)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed u64
	errmsg := C.Buffer{}

	//// This is done in order to ensure that goroutines don't
	//// swap threads between recursive calls to the enclave.
	//runtime.LockOSThread()
	//defer runtime.UnlockOSThread()

	res, err := C.query(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	}
	return receiveVector(res), uint64(gasUsed), nil
}

func AnalyzeCode(
	cache Cache,
	codeHash []byte,
) (*v1types.AnalysisReport, error) {
	cs := sendSlice(codeHash)
	defer runtime.KeepAlive(codeHash)
	errMsg := C.Buffer{}
	report, err := C.analyze_code(cache.ptr, cs, &errMsg)
	if err != nil {
		return nil, errorWithMessage(err, errMsg)
	}
	res := v1types.AnalysisReport{
		HasIBCEntryPoints: bool(report.has_ibc_entry_points),
		RequiredFeatures:  string(receiveVector(report.required_features)),
	}
	return &res, nil
}

// KeyGen Send KeyGen request to enclave
func KeyGen() ([]byte, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, return a dummy 32-byte public key
		// Key generation is only needed for node registration which doesn't happen in replay
		logInfo("KeyGen", "Skipped in replay mode, returning dummy key")
		return make([]byte, 32), nil
	}

	errmsg := C.Buffer{}
	res, err := C.key_gen(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

// CreateAttestationReport Send request to enclave
func CreateAttestationReport(ext_sk []byte, is_migration_report bool) (bool, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, skip attestation report creation (no SGX)
		logInfo("CreateAttestationReport", "Skipped in replay mode")
		return true, nil
	}
	errmsg := C.Buffer{}

	flags := u32(0)
	if is_migration_report {
		flags |= u32(0x10)
	}

	skSlice := sendSlice(ext_sk)
	defer freeAfterSend(skSlice)

	_, err := C.create_attestation_report(skSlice, flags, &errmsg)
	if err != nil {
		return false, errorWithMessage(err, errmsg)
	}
	return true, nil
}

func GetEncryptedSeed(cert []byte, replace_machine_id []byte) ([]byte, []byte, error) {
	recorder := GetRecorder()
	certHash := sha256.Sum256(cert)
	certHashHex := hex.EncodeToString(certHash[:])

	logInfo("GetEncryptedSeed", "SGX called: certHashHex=%s certLen=%d replayMode=%v",
		certHashHex, len(cert), recorder.IsReplayMode())

	if recorder.IsReplayMode() {
		// Try local DB first
		height := recorder.GetCurrentBlockHeight()
		if output, errMsg, found := recorder.ReplayGetEncryptedSeed(height, certHash[:]); found {
			if errMsg != "" {
				// Replay the exact same error the SGX enclave produced
				return nil, fmt.Errorf("%s", errMsg)
			}
			return output, nil
		}

		// Fetch from remote SGX node
		client := GetEcallClient()
		output, err := client.FetchEncryptedSeed(height, certHashHex)
		if err != nil {
			return nil, fmt.Errorf("GetEncryptedSeed replay failed: %w", err)
		}

		// Cache locally
		if cacheErr := recorder.RecordGetEncryptedSeed(height, certHash[:], output); cacheErr != nil {
			logError("GetEncryptedSeed", "Failed to cache: %v", cacheErr)
		}
		return output, nil
	}

	// SGX mode: call enclave and record result
	errmsg := C.Buffer{}
	certSlice := sendSlice(cert)
	defer freeAfterSend(certSlice)
	replace_machine_slice := sendSlice(replace_machine_id)
	defer freeAfterSend(replace_machine_slice)
	res, err := C.get_encrypted_seed(certSlice, replace_machine_slice, &errmsg)
	if err != nil {
		enclaveErr := errorWithMessage(err, errmsg)
		logInfo("GetEncryptedSeed", "SGX enclave FAILED for %s: %v", certHashHex, enclaveErr)
		// Record the error so non-SGX nodes can replay the exact same message
		height := recorder.GetCurrentBlockHeight()
		if recErr := recorder.RecordGetEncryptedSeedError(height, certHash[:], enclaveErr.Error()); recErr != nil {
			logError("GetEncryptedSeed", "Failed to record error: %v", recErr)
		}
		return nil, enclaveErr
	}

	output := receiveVector(res)
	height := recorder.GetCurrentBlockHeight()
	logInfo("GetEncryptedSeed", "SGX enclave SUCCESS for %s (%d bytes), recording at height %d...", certHashHex, len(output), height)
	if err := recorder.RecordGetEncryptedSeed(height, certHash[:], output); err != nil {
		logError("GetEncryptedSeed", "Failed to record: %v", err)
	} else {
		logInfo("GetEncryptedSeed", "Recorded GetEncryptedSeed for %s OK", certHashHex)
	}
	return output, nil
}

func GetEncryptedGenesisSeed(pk []byte) ([]byte, error) {
	// Genesis seed is only used during bootstrap of a new network.
	// Non-SGX replay nodes don't need this - they sync from existing networks.
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		return nil, errors.New("get encrypted genesis seed not supported on non-SGX node")
	}
	errmsg := C.Buffer{}
	pkSlice := sendSlice(pk)
	defer freeAfterSend(pkSlice)
	res, err := C.get_encrypted_genesis_seed(pkSlice, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

/**** To error module ***/

func errorWithMessage(err error, b C.Buffer) error {
	// this checks for out of gas as a special case
	if errno, ok := err.(syscall.Errno); ok && int(errno) == 2 {
		return types.OutOfGasError{}
	}
	msg := receiveVector(b)
	if msg == nil {
		return err
	}
	return fmt.Errorf("%s", string(msg))
}
