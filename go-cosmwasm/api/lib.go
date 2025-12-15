//go:build !secretcli
// +build !secretcli

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
	"time"
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

// replayExecution handles replay of a recorded execution trace.
// It applies storage ops to the store and consumes the correct amount of gas.
// Returns (result, gasUsed, error, found). If found is false, trace was not available.
func replayExecution(store KVStore, gasMeter *GasMeter, execIndex int64) ([]byte, uint64, error, bool) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()

	// Get trace from memory (fetched on-demand when needed)
	// Traces are created during execution on SGX node, so we fetch them when actually needed
	trace, found := recorder.GetTraceFromMemory(execIndex)
	if !found {
		// Trace not in memory - wait for it to become available from SGX node
		// Traces are created during execution on SGX node, so we need to wait for them
		fmt.Printf("[replayExecution] TRACE NOT FOUND in memory: height=%d index=%d, waiting for SGX node to create it\n", height, execIndex)

		client := GetEcallClient()
		maxRetries := 20                    // More retries since traces are created during execution
		retryDelay := 50 * time.Millisecond // Start with 50ms
		maxDelay := 2 * time.Second         // Cap at 2 seconds

		for attempt := 0; attempt < maxRetries; attempt++ {
			// Fetch all traces for the block (new ones may have been added)
			allTraces, err := client.FetchBlockTraces(height)
			if err == nil {
				// Update memory cache with all traces
				recorder.SetBlockTraces(allTraces)

				// Check if our trace is now in memory
				trace, found = recorder.GetTraceFromMemory(execIndex)
				if found {
					fmt.Printf("[replayExecution] Successfully fetched trace: height=%d index=%d (attempt %d)\n", height, execIndex, attempt+1)
					break
				}
			}

			// Trace still not found - wait and retry
			if attempt < maxRetries-1 {
				delay := retryDelay * time.Duration(1<<uint(attempt))
				if delay > maxDelay {
					delay = maxDelay
				}
				fmt.Printf("[replayExecution] Trace not available yet, waiting: height=%d index=%d attempt=%d delay=%v\n", height, execIndex, attempt+1, delay)
				time.Sleep(delay)
			}
		}

		if !found {
			fmt.Printf("[replayExecution] TRACE NOT FOUND after retries: height=%d index=%d\n", height, execIndex)
			return nil, 0, nil, false
		}
	}

	fmt.Printf("[replayExecution] Found trace: height=%d index=%d ops=%d resultLen=%d gasUsed=%d callbackGas=%d hasError=%v\n",
		height, execIndex, len(trace.Ops), len(trace.Result), trace.GasUsed, trace.CallbackGas, trace.HasError)
	fmt.Printf("[replayExecution] DEBUG: Will consume callbackGas=%d (in multiplied units) if > localConsumed\n", trace.CallbackGas)

	// Snapshot gas before applying ops
	var gasBefore uint64
	if gasMeter != nil {
		gasBefore = (*gasMeter).GasConsumed()
		fmt.Printf("[replayExecution] Gas snapshot before ops: %d\n", gasBefore)
	}

	// Apply recorded storage ops to the store
	replayer := NewReplayingKVStore(store)
	fmt.Printf("[replayExecution] Applying %d ops to store\n", len(trace.Ops))
	replayer.ApplyOps(trace.Ops)
	fmt.Printf("[replayExecution] Finished applying ops\n")

	// Calculate and consume remaining gas to match SGX callback gas
	if gasMeter != nil {
		gasAfter := (*gasMeter).GasConsumed()
		localGasConsumed := gasAfter - gasBefore

		fmt.Printf("[replayExecution] Gas consumption: before=%d after=%d localConsumed=%d callbackGas=%d\n",
			gasBefore, gasAfter, localGasConsumed, trace.CallbackGas)

		// SAFETY CHECK: If Infinite Meter is working, localGasConsumed MUST be 0 when callbackGas=0
		if localGasConsumed > 0 && trace.CallbackGas == 0 {
			fmt.Printf("[replayExecution] CRITICAL WARNING: Local DB writes consumed %d gas but Validator consumed 0.\n", localGasConsumed)
			fmt.Printf("[replayExecution] This means the Infinite Gas Meter trick in keeper.go is NOT working.\n")
			fmt.Printf("[replayExecution] The store passed to replayExecution is still connected to the real gas meter.\n")
			// We cannot "refund" gas, so this execution is doomed to fail consensus.
		}

		if trace.CallbackGas > 0 {
			if trace.CallbackGas > localGasConsumed {
				remaining := trace.CallbackGas - localGasConsumed
				gasBeforeConsume := (*gasMeter).GasConsumed()
				(*gasMeter).ConsumeGas(remaining, "enclave callback gas replay")
				gasAfterConsume := (*gasMeter).GasConsumed()
				fmt.Printf("[replayExecution] Consumed remaining callback gas: callbackGas=%d localConsumed=%d remaining=%d\n",
					trace.CallbackGas, localGasConsumed, remaining)
				fmt.Printf("[replayExecution] Gas before consume=%d after consume=%d (consumed %d multiplied units = %d SDK gas)\n",
					gasBeforeConsume, gasAfterConsume, remaining, remaining/1000)
			} else if trace.CallbackGas < localGasConsumed {
				fmt.Printf("[replayExecution] WARNING: local gas (%d) > recorded callbackGas (%d), cannot reconcile\n",
					localGasConsumed, trace.CallbackGas)
			} else {
				fmt.Printf("[replayExecution] Callback gas matches: %d\n", trace.CallbackGas)
			}
		} else {
			// callbackGas=0 means validator consumed 0 callback gas
			// Since localConsumed=0 (infinite meter working), we match perfectly
			// No need to consume additional gas
			fmt.Printf("[replayExecution] callbackGas=0 and localConsumed=0 - gas consumption matches validator\n")
		}

		// Note: We do NOT consume compute gas here. The keeper will call consumeGas(ctx, gasUsed)
		// after we return, which will consume (gasUsed / GasMultiplier) + 1.
		// We only reconcile callback gas here to match the DB operation gas.
	}

	if trace.HasError {
		fmt.Printf("[replayExecution] Returning error: %s\n", trace.ErrorMsg)
		return nil, trace.GasUsed, fmt.Errorf("%s", trace.ErrorMsg), true
	}

	// Return gasUsed as-is. If gasUsed=0, it means the validator consumed 0 compute gas.
	// We've already consumed callbackGas above to match the validator's callback gas consumption.
	// The keeper will call consumeGas(ctx, gasUsed), which will consume (gasUsed / 1000) + 1.
	// If gasUsed=0, this will consume 1 SDK gas, which matches the validator's behavior.
	gasToReturn := trace.GasUsed
	if gasToReturn == 0 {
		fmt.Printf("[replayExecution] gasUsed=0: validator consumed 0 compute gas, returning 0 (keeper will consume 1 SDK gas minimum)\n")
	}

	// Return gasToReturn so the keeper can call consumeGas(ctx, gasUsed)
	// The keeper's consumeGas will consume (gasUsed / GasMultiplier) + 1, matching SGX behavior.
	// We've already reconciled callback gas above.
	return trace.Result, gasToReturn, nil, true
}

func HealthCheck() ([]byte, error) {
	errmsg := C.Buffer{}

	res, err := C.get_health_check(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

func SubmitBlockSignatures(header []byte, commit []byte, txs []byte, encRandom []byte /* valSet []byte, nextValSet []byte */) ([]byte, []byte, error) {
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
	errmsg := C.Buffer{}
	evidenceSlice := sendSlice(evidence)
	defer freeAfterSend(evidenceSlice)
	C.submit_validator_set_evidence(evidenceSlice, &errmsg)
	return nil
}

func InitBootstrap(spid []byte, apiKey []byte) ([]byte, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, return a dummy 32-byte public key
		// This function is only called during bootstrap which doesn't happen in replay
		fmt.Println("[InitBootstrap] Skipped in replay mode")
		return make([]byte, 32), nil
	}

	errmsg := C.Buffer{}
	spidSlice := sendSlice(spid)
	defer freeAfterSend(spidSlice)
	apiKeySlice := sendSlice(apiKey)
	defer freeAfterSend(apiKeySlice)

	res, err := C.init_bootstrap(spidSlice, apiKeySlice, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

func LoadSeedToEnclave(masterKey []byte, seed []byte, apiKey []byte) (bool, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, skip loading seed to enclave (no enclave)
		fmt.Println("[LoadSeedToEnclave] Skipped in replay mode")
		return true, nil
	}

	pkSlice := sendSlice(masterKey)
	defer freeAfterSend(pkSlice)
	seedSlice := sendSlice(seed)
	defer freeAfterSend(seedSlice)
	apiKeySlice := sendSlice(apiKey)
	defer freeAfterSend(apiKeySlice)
	errmsg := C.Buffer{}

	_, err := C.init_node(pkSlice, seedSlice, apiKeySlice, &errmsg)
	if err != nil {
		return false, errorWithMessage(err, errmsg)
	}
	return true, nil
}

func RotateStore(kvs []byte) (bool, error) {
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
	res := C.get_network_pubkey(u32(i_seed))
	return receiveVector(res.buf1), receiveVector(res.buf2)
}

func EmergencyApproveUpgrade(nodeDir string, msg string) (bool, error) {
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
		fmt.Println("[InitEnclaveRuntime] Skipped in replay mode")
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

func OnApproveMachineID(machineID []byte, proof *[32]byte, is_on_chain bool) error {
	msgBuf := sendSlice(machineID)
	defer freeAfterSend(msgBuf)

	ret, err := C.onchain_approve_machine_id(msgBuf, (*C.uint8_t)(unsafe.Pointer(proof)), C.bool(is_on_chain))
	if err != nil {
		return err
	}
	if !ret {
		return errors.New("onchain_approve_machine_id failed")
	}

	return nil
}

func Create(cache Cache, wasm []byte) ([]byte, error) {
	code := sendSlice(wasm)
	defer freeAfterSend(code)
	errmsg := C.Buffer{}
	id, err := C.create(cache.ptr, code, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(id), nil
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

	// SGX mode: wrap store to record operations
	recordingStore := NewRecordingKVStore(store)

	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)
	m := sendSlice(msg)
	defer freeAfterSend(m)

	// set up a new stack frame to handle iterators
	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(recordingStore, counter)
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
	if gasMeter != nil {
		gasBefore = (*gasMeter).GasConsumed()
	}

	res, err := C.migrate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, adminBuffer, adminProofBuffer)

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
		GasUsed:     uint64(gasUsed),
		CallbackGas: callbackGas,
	}

	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		trace.HasError = true
		trace.ErrorMsg = string(receiveVector(errmsg))
		if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
			fmt.Printf("[Migrate] Failed to record trace: %v\n", recordErr)
		}
		return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	}

	trace.Result = receiveVector(res)
	if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
		fmt.Printf("[Migrate] Failed to record trace: %v\n", recordErr)
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

	// SGX mode: wrap store to record operations
	recordingStore := NewRecordingKVStore(store)

	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)

	// set up a new stack frame to handle iterators
	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(recordingStore, counter)
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

	// Capture gas before execution to measure callback gas
	var gasBefore uint64
	if gasMeter != nil {
		gasBefore = (*gasMeter).GasConsumed()
	}

	res, err := C.update_admin(cache.ptr, id, p, db, a, q, u64(gasLimit), &errmsg, s, currentAdminBuffer, currentAdminProofBuffer, newAdminBuffer)

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
		GasUsed:     0, // UpdateAdmin doesn't return gas used
		CallbackGas: callbackGas,
	}

	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		trace.HasError = true
		trace.ErrorMsg = string(receiveVector(errmsg))
		if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
			fmt.Printf("[UpdateAdmin] Failed to record trace: %v\n", recordErr)
		}
		return nil, errorWithMessage(err, errmsg)
	}

	trace.Result = receiveVector(res)
	if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
		fmt.Printf("[UpdateAdmin] Failed to record trace: %v\n", recordErr)
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
		fmt.Printf("[Instantiate] REPLAY mode: height=%d execIndex=%d\n", height, execIndex)
		if result, gas, err, found := replayExecution(store, gasMeter, execIndex); found {
			fmt.Printf("[Instantiate] REPLAY success: resultLen=%d gas=%d err=%v\n", len(result), gas, err)
			return result, gas, err
		}
		fmt.Printf("[Instantiate] REPLAY FAILED: trace not found!\n")
		return nil, 0, fmt.Errorf("Instantiate replay failed: trace not found for height %d index %d", height, execIndex)
	}

	// SGX mode: wrap store to record operations
	recordingStore := NewRecordingKVStore(store)

	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)
	m := sendSlice(msg)
	defer freeAfterSend(m)

	// set up a new stack frame to handle iterators
	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(recordingStore, counter)
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
	if gasMeter != nil {
		gasBefore = (*gasMeter).GasConsumed()
		fmt.Printf("[Instantiate] SGX gasBefore=%d\n", gasBefore)
	}

	res, err := C.instantiate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, adminBuffer)

	// Calculate callback gas consumed during execution
	var callbackGas uint64
	if gasMeter != nil {
		gasAfter := (*gasMeter).GasConsumed()
		callbackGas = gasAfter - gasBefore
		fmt.Printf("[Instantiate] SGX gasAfter=%d callbackGas=%d (gasAfter - gasBefore)\n", gasAfter, callbackGas)
	} else {
		fmt.Printf("[Instantiate] SGX WARNING: gasMeter is nil, cannot measure callbackGas\n")
	}

	fmt.Printf("[Instantiate] SGX C.instantiate returned: gasUsed=%d callbackGas=%d\n", gasUsed, callbackGas)

	// Record the execution trace
	trace := &ExecutionTrace{
		Index:       execIndex,
		Ops:         recordingStore.GetOps(),
		GasUsed:     uint64(gasUsed),
		CallbackGas: callbackGas,
	}

	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		trace.HasError = true
		trace.ErrorMsg = string(receiveVector(errmsg))
		if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
			fmt.Printf("[Instantiate] Failed to record trace: %v\n", recordErr)
		}
		return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	}

	trace.Result = receiveVector(res)
	if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
		fmt.Printf("[Instantiate] Failed to record trace: %v\n", recordErr)
	} else {
		fmt.Printf("[Instantiate] SGX recorded trace: height=%d index=%d ops=%d resultLen=%d gasUsed=%d callbackGas=%d\n",
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
		fmt.Printf("[Handle] REPLAY mode: height=%d execIndex=%d\n", height, execIndex)
		if result, gas, err, found := replayExecution(store, gasMeter, execIndex); found {
			fmt.Printf("[Handle] REPLAY success: resultLen=%d gas=%d err=%v\n", len(result), gas, err)
			return result, gas, err
		}
		fmt.Printf("[Handle] REPLAY FAILED: trace not found!\n")
		return nil, 0, fmt.Errorf("Handle replay failed: trace not found for height %d index %d", height, execIndex)
	}

	// SGX mode: wrap store to record operations
	recordingStore := NewRecordingKVStore(store)

	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)
	m := sendSlice(msg)
	defer freeAfterSend(m)

	// set up a new stack frame to handle iterators
	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(recordingStore, counter)
	db := buildDB(&dbState, gasMeter)
	s := sendSlice(sigInfo)
	defer freeAfterSend(s)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed u64
	errmsg := C.Buffer{}

	// Capture gas before execution to measure callback gas
	var gasBefore uint64
	if gasMeter != nil {
		gasBefore = (*gasMeter).GasConsumed()
		fmt.Printf("[Handle] SGX gasBefore=%d\n", gasBefore)
	}

	res, err := C.handle(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, u8(handleType))

	// Calculate callback gas consumed during execution
	var callbackGas uint64
	if gasMeter != nil {
		gasAfter := (*gasMeter).GasConsumed()
		callbackGas = gasAfter - gasBefore
		fmt.Printf("[Handle] SGX gasAfter=%d callbackGas=%d (gasAfter - gasBefore)\n", gasAfter, callbackGas)
	} else {
		fmt.Printf("[Handle] SGX WARNING: gasMeter is nil, cannot measure callbackGas\n")
	}

	fmt.Printf("[Handle] SGX C.handle returned: gasUsed=%d callbackGas=%d\n", gasUsed, callbackGas)

	// Record the execution trace
	trace := &ExecutionTrace{
		Index:       execIndex,
		Ops:         recordingStore.GetOps(),
		GasUsed:     uint64(gasUsed),
		CallbackGas: callbackGas,
	}

	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		trace.HasError = true
		trace.ErrorMsg = string(receiveVector(errmsg))
		if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
			fmt.Printf("[Handle] Failed to record trace: %v\n", recordErr)
		}
		return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	}

	trace.Result = receiveVector(res)
	if recordErr := recorder.RecordExecutionTrace(height, execIndex, trace); recordErr != nil {
		fmt.Printf("[Handle] Failed to record trace: %v\n", recordErr)
	} else {
		fmt.Printf("[Handle] SGX recorded trace: height=%d index=%d ops=%d resultLen=%d gasUsed=%d callbackGas=%d\n",
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
		fmt.Println("[KeyGen] Skipped in replay mode, returning dummy key")
		return make([]byte, 32), nil
	}

	errmsg := C.Buffer{}
	res, err := C.key_gen(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

// CreateAttestationReport Send CreateAttestationReport request to enclave
func CreateAttestationReport(no_epid bool, no_dcap bool, is_migration_report bool) (bool, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, skip attestation report creation (no SGX)
		fmt.Println("[CreateAttestationReport] Skipped in replay mode")
		return true, nil
	}

	errmsg := C.Buffer{}

	flags := u32(0)
	if no_epid {
		flags |= u32(1)
	}
	if no_dcap {
		flags |= u32(2)
	}
	if is_migration_report {
		flags |= u32(0x10)
	}

	_, err := C.create_attestation_report(flags, &errmsg)
	if err != nil {
		return false, errorWithMessage(err, errmsg)
	}
	return true, nil
}

func GetEncryptedSeed(cert []byte) ([]byte, error) {
	recorder := GetRecorder()
	certHash := sha256.Sum256(cert)
	certHashHex := hex.EncodeToString(certHash[:])

	if recorder.IsReplayMode() {
		// Try local DB first
		if output, found := recorder.ReplayGetEncryptedSeed(certHash[:]); found {
			return output, nil
		}

		// Fetch from remote SGX node
		client := GetEcallClient()
		output, err := client.FetchEncryptedSeed(certHashHex)
		if err != nil {
			return nil, fmt.Errorf("GetEncryptedSeed replay failed: %w", err)
		}

		// Cache locally
		if cacheErr := recorder.RecordGetEncryptedSeed(certHash[:], output); cacheErr != nil {
			fmt.Printf("[GetEncryptedSeed] Failed to cache: %v\n", cacheErr)
		}
		return output, nil
	}

	// SGX mode: call enclave and record result
	errmsg := C.Buffer{}
	certSlice := sendSlice(cert)
	defer freeAfterSend(certSlice)
	res, err := C.get_encrypted_seed(certSlice, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}

	output := receiveVector(res)
	if err := recorder.RecordGetEncryptedSeed(certHash[:], output); err != nil {
		fmt.Printf("[GetEncryptedSeed] Failed to record: %v\n", err)
	}
	return output, nil
}

func GetEncryptedGenesisSeed(pk []byte) ([]byte, error) {
	// Genesis seed is only used during bootstrap of a new network.
	// Non-SGX replay nodes don't need this - they sync from existing networks.
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
