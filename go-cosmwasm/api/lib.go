//go:build !secretcli
// +build !secretcli

package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
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
	errmsg := C.Buffer{}

	res, err := C.get_health_check(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

func SubmitBlockSignatures(header []byte, commit []byte, txs []byte, encRandom []byte, cronMsgs []byte /* valSet []byte, nextValSet []byte */) ([]byte, []byte, error) {
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
	return receiveVector(res.buf1), receiveVector(res.buf2), nil
}

func SubmitValidatorSetEvidence(evidence []byte) error {
	errmsg := C.Buffer{}
	evidenceSlice := sendSlice(evidence)
	defer freeAfterSend(evidenceSlice)
	C.submit_validator_set_evidence(evidenceSlice, &errmsg)
	return nil
}

func InitBootstrap() ([]byte, error) {
	errmsg := C.Buffer{}
	res, err := C.init_bootstrap(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

func LoadSeedToEnclave(masterKey []byte, seed []byte) (bool, error) {
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

	//// This is done in order to ensure that goroutines don't
	//// swap threads between recursive calls to the enclave.
	//runtime.LockOSThread()
	//defer runtime.UnlockOSThread()

	res, err := C.migrate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, adminBuffer, adminProofBuffer)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	}
	return receiveVector(res), uint64(gasUsed), nil
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

	//// This is done in order to ensure that goroutines don't
	//// swap threads between recursive calls to the enclave.
	//runtime.LockOSThread()
	//defer runtime.UnlockOSThread()

	res, err := C.update_admin(cache.ptr, id, p, db, a, q, u64(gasLimit), &errmsg, s, currentAdminBuffer, currentAdminProofBuffer, newAdminBuffer)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
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

	//// This is done in order to ensure that goroutines don't
	//// swap threads between recursive calls to the enclave.
	//runtime.LockOSThread()
	//defer runtime.UnlockOSThread()

	res, err := C.instantiate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, adminBuffer)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	}
	return receiveVector(res), uint64(gasUsed), nil
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

	//// This is done in order to ensure that goroutines don't
	//// swap threads between recursive calls to the enclave.
	//runtime.LockOSThread()
	//defer runtime.UnlockOSThread()

	res, err := C.handle(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s, u8(handleType))
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	}
	return receiveVector(res), uint64(gasUsed), nil
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
	errmsg := C.Buffer{}
	res, err := C.key_gen(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

// CreateAttestationReport Send request to enclave
func CreateAttestationReport(is_migration_report bool) (bool, error) {
	errmsg := C.Buffer{}

	flags := u32(0)
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
	errmsg := C.Buffer{}
	certSlice := sendSlice(cert)
	defer freeAfterSend(certSlice)
	res, err := C.get_encrypted_seed(certSlice, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

func GetEncryptedGenesisSeed(pk []byte) ([]byte, error) {
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
