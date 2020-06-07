package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lgo_cosmwasm
// #include <stdlib.h>
// #include "bindings.h"
import "C"

import "fmt"

// nice aliases to the rust names
type i32 = C.int32_t
type i64 = C.int64_t
type u64 = C.uint64_t
type u32 = C.uint32_t
type u8 = C.uint8_t
type u8_ptr = *C.uint8_t
type usize = C.uintptr_t
type cint = C.int

type Cache struct {
	ptr *C.cache_t
}

func InitBootstrap() ([]byte, error) {
	errmsg := C.Buffer{}

	res, err := C.init_bootstrap(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), nil
}

func LoadSeedToEnclave(masterCert []byte, seed []byte) (bool, error) {
	pkSlice := sendSlice(masterCert)
	seedSlice := sendSlice(seed)
	errmsg := C.Buffer{}

	_, err := C.init_node(pkSlice, seedSlice, &errmsg)
	if err != nil {
		return false, errorWithMessage(err, errmsg)
	}
	return true, nil
}

func InitCache(dataDir string, cacheSize uint64) (Cache, error) {
	dir := sendSlice([]byte(dataDir))
	errmsg := C.Buffer{}

	ptr, err := C.init_cache(dir, usize(cacheSize), &errmsg)
	if err != nil {
		return Cache{}, errorWithMessage(err, errmsg)
	}
	return Cache{ptr: ptr}, nil
}

func ReleaseCache(cache Cache) {
	C.release_cache(cache.ptr)
}

func Create(cache Cache, wasm []byte) ([]byte, error) {
	code := sendSlice(wasm)
	errmsg := C.Buffer{}
	id, err := C.create(cache.ptr, code, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(id), nil
}

func GetCode(cache Cache, code_id []byte) ([]byte, error) {
	id := sendSlice(code_id)
	errmsg := C.Buffer{}
	code, err := C.get_code(cache.ptr, id, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(code), nil
}

func Instantiate(cache Cache, code_id []byte, params []byte, msg []byte, store KVStore, api *GoAPI, gasLimit uint64) ([]byte, uint64, error) {
	id := sendSlice(code_id)
	p := sendSlice(params)
	m := sendSlice(msg)
	db := buildDB(store)
	a := buildAPI(api)
	var gasUsed u64
	errmsg := C.Buffer{}
	res, err := C.instantiate(cache.ptr, id, p, m, db, a, u64(gasLimit), &gasUsed, &errmsg)
	if err != nil {
		return nil, 0, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), uint64(gasUsed), nil
}

func Handle(cache Cache, code_id []byte, params []byte, msg []byte, store KVStore, api *GoAPI, gasLimit uint64) ([]byte, uint64, error) {
	id := sendSlice(code_id)
	p := sendSlice(params)
	m := sendSlice(msg)
	db := buildDB(store)
	a := buildAPI(api)
	var gasUsed u64
	errmsg := C.Buffer{}
	res, err := C.handle(cache.ptr, id, p, m, db, a, u64(gasLimit), &gasUsed, &errmsg)
	if err != nil {
		return nil, 0, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), uint64(gasUsed), nil
}

func Query(cache Cache, code_id []byte, msg []byte, store KVStore, api *GoAPI, gasLimit uint64) ([]byte, uint64, error) {
	id := sendSlice(code_id)
	m := sendSlice(msg)
	db := buildDB(store)
	a := buildAPI(api)
	var gasUsed u64
	errmsg := C.Buffer{}
	res, err := C.query(cache.ptr, id, m, db, a, u64(gasLimit), &gasUsed, &errmsg)
	if err != nil {
		return nil, 0, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), uint64(gasUsed), nil
}

// KeyGen Send KeyGen request to enclave
func KeyGen() ([]byte, error) {
	errmsg := C.Buffer{}
	res, err := C.key_gen(&errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), nil
}

// KeyGen Seng KeyGen request to enclave
func CreateAttestationReport() (bool, error) {
	errmsg := C.Buffer{}
	_, err := C.create_attestation_report(&errmsg)
	if err != nil {
		return false, errorWithMessage(err, errmsg)
	}
	return true, nil
}

func GetEncryptedSeed(cert []byte) ([]byte, error) {
	errmsg := C.Buffer{}
	certSlice := sendSlice(cert)
	res, err := C.get_encrypted_seed(certSlice, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return receiveSlice(res), nil
}

/**** To error module ***/

func errorWithMessage(err error, b C.Buffer) error {
	msg := receiveSlice(b)
	if msg == nil {
		return err
	}
	return fmt.Errorf("%s", string(msg))
}
