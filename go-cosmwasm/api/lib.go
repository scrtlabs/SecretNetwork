//go:build !secretcli
// +build !secretcli

package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
	"fmt"
	"runtime"
	"syscall"

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

func SubmitBlockSignatures(header []byte, commit []byte, txs []byte, encRandom []byte /* valSet []byte, nextValSet []byte */) ([]byte, error) {
	errmsg := C.Buffer{}
	spidSlice := sendSlice(header)
	defer freeAfterSend(spidSlice)
	apiKeySlice := sendSlice(commit)
	defer freeAfterSend(apiKeySlice)
	encRandomSlice := sendSlice(encRandom)
	defer freeAfterSend(encRandomSlice)
	txsSlice := sendSlice(txs)
	defer freeAfterSend(txsSlice)
	//valSetSlice := sendSlice(valSet)
	//defer freeAfterSend(apiKeySlice)
	//nextValSetSlice := sendSlice(nextValSet)
	//defer freeAfterSend(apiKeySlice)

	res, err := C.submit_block_signatures(spidSlice, apiKeySlice, txsSlice, encRandomSlice /* valSetSlice, nextValSetSlice,*/, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveVector(res), nil
}

func InitBootstrap(spid []byte, apiKey []byte) ([]byte, error) {
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

	res, err := C.instantiate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg, s)
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

// CreateAttestationReport Send CreateAttestationReport request to enclave
func CreateAttestationReport(apiKey []byte, dryRun bool) (bool, error) {
	errmsg := C.Buffer{}
	apiKeySlice := sendSlice(apiKey)
	defer freeAfterSend(apiKeySlice)

	_, err := C.create_attestation_report(apiKeySlice, &errmsg, cbool(dryRun))
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
