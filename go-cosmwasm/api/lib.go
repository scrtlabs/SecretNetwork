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

// GasMultiplier matches the WASM VM gas multiplier used in callbacks.
// 1 SDK gas = GasMultiplier WASM gas.
const GasMultiplier uint64 = 1000

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
	cronMsgsSlice := sendSlice(cronMsgs)
	defer freeAfterSend(cronMsgsSlice)

	res, err := C.submit_block_signatures(spidSlice, apiKeySlice, txsSlice, encRandomSlice, cronMsgsSlice /* valSetSlice, nextValSetSlice,*/, &errmsg)
	if err != nil {
		return nil, nil, errorWithMessage(err, errmsg)
	}

	random := receiveVector(res.buf1)
	evidence := receiveVector(res.buf2)

	// Record as ecall stream at index 0 (reserved for SubmitBlockSignatures)
	height := recorder.GetCurrentBlockHeight()
	streamWriter := NewOcallStreamWriter()
	// No ocalls for SubmitBlockSignatures — just the result
	packedResult := PackBlockSignaturesResult(random, evidence)
	streamBytes := streamWriter.Finalize(EcallResult{
		Result:   packedResult,
		GasUsed:  0,
		HasError: false,
	})
	if recordErr := recorder.RecordEcallStream(height, 0, streamBytes); recordErr != nil {
		logError("SubmitBlockSignatures", "Failed to record stream: %v", recordErr)
	}

	return random, evidence, nil
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

func InitBootstrap(spid []byte, apiKey []byte) ([]byte, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, return a dummy 32-byte public key
		// This function is only called during bootstrap which doesn't happen in replay
		logInfo("InitBootstrap", "Skipped in replay mode")
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
		logInfo("LoadSeedToEnclave", "Skipped in replay mode")
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
	if recorder.IsReplayMode() {
		// No enclave; return empty so UpdateNetworkKeys loop exits immediately
		return nil, nil
	}
	res := C.get_network_pubkey(u32(i_seed))
	return receiveVector(res.buf1), receiveVector(res.buf2)
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

func OnApproveMachineID(machineID []byte, proof *[32]byte, is_on_chain bool) error {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, skip machine ID approval (no SGX)
		logInfo("OnApproveMachineID", "Skipped in replay mode")
		return nil
	}

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

// finalizeAndRecordStream builds the ecall result, finalizes the stream, records it, and returns the result/error
func finalizeAndRecordStream(
	funcName string,
	streamWriter *OcallStreamWriter,
	recorder *EcallRecorder,
	height int64,
	execIndex int64,
	gasUsed uint64,
	sdkGasUsed uint64,
	res C.Buffer,
	err error,
	errmsg C.Buffer,
) ([]byte, error) {
	// Build ecall result
	ecallResult := EcallResult{GasUsed: gasUsed, SDKGasUsed: sdkGasUsed}

	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		errorMsgBytes := receiveVector(errmsg)
		ecallResult.HasError = true
		ecallResult.ErrorMsg = string(errorMsgBytes)
		streamBytes := streamWriter.Finalize(ecallResult)

		if recordErr := recorder.RecordEcallStream(height, execIndex, streamBytes); recordErr != nil {
			logError(funcName, "Failed to record stream: %v", recordErr)
		}

		if errno, ok := err.(syscall.Errno); ok && int(errno) == 2 {
			return nil, types.OutOfGasError{}
		}
		if errorMsgBytes == nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", string(errorMsgBytes))
	}

	ecallResult.Result = receiveVector(res)
	streamBytes := streamWriter.Finalize(ecallResult)

	if recordErr := recorder.RecordEcallStream(height, execIndex, streamBytes); recordErr != nil {
		logError(funcName, "Failed to record stream: %v", recordErr)
	} else {
		logDebug(funcName, "SGX recorded stream: height=%d index=%d streamLen=%d resultLen=%d gasUsed=%d",
			height, execIndex, len(streamBytes), len(ecallResult.Result), ecallResult.GasUsed)
	}

	return ecallResult.Result, nil
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
) ([]byte, uint64, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	if recorder.IsReplayMode() {
		streamBytes, found := recorder.GetStreamFromMemory(execIndex)
		if !found {
			return nil, 0, 0, fmt.Errorf("Migrate replay failed: stream not found for height %d index %d", height, execIndex)
		}
		result, wasmGasUsed, sdkGasUsed, err := ReplayStream(store, querier, streamBytes)
		return result, wasmGasUsed, sdkGasUsed, err
	}

	// SGX mode: set up stream writer to record ocalls
	streamWriter := NewOcallStreamWriter()
	SetActiveStreamWriter(streamWriter)

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

	// Capture exact SDK gas by extracting the underlying raw meter (bypasses 1000x multiplier)
	var rawGasMeter GasMeter
	if ogm, ok := (*gasMeter).(OriginalGasMeter); ok {
		rawGasMeter = ogm.OriginalMeterAPI()
	} else {
		rawGasMeter = *gasMeter
	}
	gasBefore := rawGasMeter.GasConsumed()

	res, err := C.migrate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, adminBuffer, adminProofBuffer)

	// Clear stream writer immediately after ecall completes
	ClearActiveStreamWriter()

	// Capture SDK gas consumed during the ecall (store ops)
	gasAfter := rawGasMeter.GasConsumed()
	storeOpsSDKGas := gasAfter - gasBefore

	// Total SDK gas = store ops + (enclave WASM gas / 1000) + 1
	totalSDKGas := storeOpsSDKGas + (uint64(gasUsed) / GasMultiplier) + 1

	result, finalErr := finalizeAndRecordStream("Migrate", streamWriter, recorder, height, execIndex, uint64(gasUsed), totalSDKGas, res, err, errmsg)
	return result, uint64(gasUsed), totalSDKGas, finalErr
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
		streamBytes, found := recorder.GetStreamFromMemory(execIndex)
		if !found {
			return nil, fmt.Errorf("UpdateAdmin replay failed: stream not found for height %d index %d", height, execIndex)
		}
		result, _, _, err := ReplayStream(store, querier, streamBytes)
		return result, err
	}

	// SGX mode: set up stream writer to record ocalls
	streamWriter := NewOcallStreamWriter()
	SetActiveStreamWriter(streamWriter)

	id := sendSlice(code_id)
	defer freeAfterSend(id)
	p := sendSlice(params)
	defer freeAfterSend(p)

	// set up a new stack frame to handle iterators
	counter := startContract()
	defer endContract(counter)

	dbState := buildDBState(store, counter)
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

	res, err := C.update_admin(cache.ptr, id, p, db, a, q, u64(gasLimit), &errmsg, s, currentAdminBuffer, currentAdminProofBuffer, newAdminBuffer)

	// Clear stream writer immediately after ecall completes
	ClearActiveStreamWriter()

	result, finalErr := finalizeAndRecordStream("UpdateAdmin", streamWriter, recorder, height, execIndex, 0, 0, res, err, errmsg)
	return result, finalErr
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
) ([]byte, uint64, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	if recorder.IsReplayMode() {
		logDebug("Instantiate", "REPLAY mode: height=%d execIndex=%d", height, execIndex)
		streamBytes, found := recorder.GetStreamFromMemory(execIndex)
		if !found {
			logWarn("Instantiate", "REPLAY FAILED: stream not found!")
			return nil, 0, 0, fmt.Errorf("Instantiate replay failed: stream not found for height %d index %d", height, execIndex)
		}
		result, wasmGasUsed, sdkGasUsed, err := ReplayStream(store, querier, streamBytes)
		return result, wasmGasUsed, sdkGasUsed, err
	}

	// SGX mode: set up stream writer to record ocalls
	streamWriter := NewOcallStreamWriter()
	SetActiveStreamWriter(streamWriter)

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

	s := sendSlice(sigInfo)
	defer freeAfterSend(s)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed u64
	errmsg := C.Buffer{}

	adminBuffer := sendSlice(admin)
	defer freeAfterSend(adminBuffer)

	// Capture exact SDK gas by extracting the underlying raw meter
	var rawGasMeter GasMeter
	if ogm, ok := (*gasMeter).(OriginalGasMeter); ok {
		rawGasMeter = ogm.OriginalMeterAPI()
	} else {
		rawGasMeter = *gasMeter
	}
	gasBefore := rawGasMeter.GasConsumed()

	res, err := C.instantiate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, adminBuffer)

	// Clear stream writer immediately after ecall completes
	ClearActiveStreamWriter()

	// Capture SDK gas consumed during the ecall (store ops)
	gasAfter := rawGasMeter.GasConsumed()
	storeOpsSDKGas := gasAfter - gasBefore

	// Total SDK gas = store ops + (enclave WASM gas / 1000) + 1
	totalSDKGas := storeOpsSDKGas + (uint64(gasUsed) / GasMultiplier) + 1

	result, finalErr := finalizeAndRecordStream("Instantiate", streamWriter, recorder, height, execIndex, uint64(gasUsed), totalSDKGas, res, err, errmsg)
	return result, uint64(gasUsed), totalSDKGas, finalErr
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
) ([]byte, uint64, uint64, error) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()
	execIndex := recorder.NextExecutionIndex()

	if recorder.IsReplayMode() {
		logDebug("Handle", "REPLAY mode: height=%d execIndex=%d", height, execIndex)
		streamBytes, found := recorder.GetStreamFromMemory(execIndex)
		if !found {
			logWarn("Handle", "REPLAY FAILED: stream not found!")
			return nil, 0, 0, fmt.Errorf("Handle replay failed: stream not found for height %d index %d", height, execIndex)
		}
		result, wasmGasUsed, sdkGasUsed, err := ReplayStream(store, querier, streamBytes)
		return result, wasmGasUsed, sdkGasUsed, err
	}

	// SGX mode: set up stream writer to record ocalls
	streamWriter := NewOcallStreamWriter()
	SetActiveStreamWriter(streamWriter)

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
	s := sendSlice(sigInfo)
	defer freeAfterSend(s)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasUsed u64
	errmsg := C.Buffer{}

	// Capture exact SDK gas by extracting the underlying raw meter
	var rawGasMeter GasMeter
	if ogm, ok := (*gasMeter).(OriginalGasMeter); ok {
		rawGasMeter = ogm.OriginalMeterAPI()
	} else {
		rawGasMeter = *gasMeter
	}
	gasBefore := rawGasMeter.GasConsumed()

	res, err := C.handle(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, u8(handleType))

	// Clear stream writer immediately after ecall completes
	ClearActiveStreamWriter()

	// Capture SDK gas consumed during the ecall (store ops)
	gasAfter := rawGasMeter.GasConsumed()
	storeOpsSDKGas := gasAfter - gasBefore

	// Total SDK gas = store ops + (enclave WASM gas / 1000) + 1
	totalSDKGas := storeOpsSDKGas + (uint64(gasUsed) / GasMultiplier) + 1

	result, finalErr := finalizeAndRecordStream("Handle", streamWriter, recorder, height, execIndex, uint64(gasUsed), totalSDKGas, res, err, errmsg)
	return result, uint64(gasUsed), totalSDKGas, finalErr
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

// CreateAttestationReport Send CreateAttestationReport request to enclave
func CreateAttestationReport(no_epid bool, no_dcap bool, is_migration_report bool) (bool, error) {
	recorder := GetRecorder()
	if recorder.IsReplayMode() {
		// In replay mode, skip attestation report creation (no SGX)
		logInfo("CreateAttestationReport", "Skipped in replay mode")
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
			logError("GetEncryptedSeed", "Failed to cache: %v", cacheErr)
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
		logError("GetEncryptedSeed", "Failed to record: %v", err)
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
