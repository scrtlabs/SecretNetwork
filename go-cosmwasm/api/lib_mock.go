//go:build secretcli
// +build secretcli

package api

//
//// #include <stdlib.h>
//// #include "bindings.h"
//import "C"

// import "C"
import (
	//"fmt"
	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
	v1types "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types/v1"
)

// nice aliases to the rust names
//type i32 = C.int32_t
//type i64 = C.int64_t
//type u64 = C.uint64_t
//type u32 = C.uint32_t
//type u8 = C.uint8_t
//type u8_ptr = *C.uint8_t
//type usize = C.uintptr_t
//type cint = C.int
//
//type Cache struct {
//	ptr *C.cache_t
//}

type Cache struct{}

func HealthCheck() ([]byte, error) {
	return nil, nil
}

func InitBootstrap(spid []byte, apiKey []byte) ([]byte, error) {
	return nil, nil
}

func SubmitBlockSignatures(header []byte, commit []byte, txs []byte, random []byte, cronmsgs []byte /* valSet []byte, nextValSet []byte*/) ([]byte, []byte, error) {
	return nil, nil, nil
}

func SubmitValidatorSetEvidence(evidence []byte) error {
	return nil
}

func LoadSeedToEnclave(masterKey []byte, seed []byte, apiKey []byte) (bool, error) {
	return true, nil
}

type Querier = types.Querier

func MigrationOp(op uint32) (bool, error) {
	return false, nil
}

func EmergencyApproveUpgrade(nodeDir string, msg string) (bool, error) {
	return false, nil
}

func InitCache(dataDir string, supportedFeatures string, cacheSize uint64) (Cache, error) {
	//dir := sendSlice([]byte(dataDir))
	//defer freeAfterSend(dir)
	//features := sendSlice([]byte(supportedFeatures))
	//defer freeAfterSend(features)
	//errmsg := C.Buffer{}
	//
	//ptr, err := C.init_cache(dir, features, usize(cacheSize), &errmsg)
	//if err != nil {
	//	return Cache{}, errorWithMessage(err, errmsg)
	//}
	return Cache{}, nil
}

func ReleaseCache(cache Cache) {
	// C.release_cache(cache.ptr)
}

func InitEnclaveRuntime(ModuleCacheSize uint16) error {
	return nil
}

func Create(cache Cache, wasm []byte) ([]byte, error) {
	//code := sendSlice(wasm)
	//defer freeAfterSend(code)
	//errmsg := C.Buffer{}
	//id, err := C.create(cache.ptr, code, &errmsg)
	//if err != nil {
	//	return nil, errorWithMessage(err, errmsg)
	//}
	//return receiveVector(id), nil
	return nil, nil
}

func GetCode(cache Cache, code_id []byte) ([]byte, error) {
	//id := sendSlice(code_id)
	//defer freeAfterSend(id)
	//errmsg := C.Buffer{}
	//code, err := C.get_code(cache.ptr, id, &errmsg)
	//if err != nil {
	//	return nil, errorWithMessage(err, errmsg)
	//}
	//return receiveVector(code), nil
	return nil, nil
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
	return nil, 0, nil
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
	return nil, nil
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
	//id := sendSlice(code_id)
	//defer freeAfterSend(id)
	//p := sendSlice(params)
	//defer freeAfterSend(p)
	//m := sendSlice(msg)
	//defer freeAfterSend(m)
	//db := buildDB(store, gasMeter)
	//a := buildAPI(api)
	//q := buildQuerier(querier)
	//var gasUsed u64
	//errmsg := C.Buffer{}
	//res, err := C.instantiate(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg)
	//if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
	//	// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
	//	return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	//}
	//return receiveVector(res), uint64(gasUsed), nil
	return nil, 0, nil
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
	//id := sendSlice(code_id)
	//defer freeAfterSend(id)
	//p := sendSlice(params)
	//defer freeAfterSend(p)
	//m := sendSlice(msg)
	//defer freeAfterSend(m)
	//db := buildDB(store, gasMeter)
	//a := buildAPI(api)
	//q := buildQuerier(querier)
	//var gasUsed u64
	//errmsg := C.Buffer{}
	//res, err := C.handle(cache.ptr, id, p, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg)
	//if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
	//	// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
	//	return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	//}
	//return receiveVector(res), uint64(gasUsed), nil
	return nil, 0, nil
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
	//id := sendSlice(code_id)
	//defer freeAfterSend(id)
	//m := sendSlice(msg)
	//defer freeAfterSend(m)
	//db := buildDB(store, gasMeter)
	//a := buildAPI(api)
	//q := buildQuerier(querier)
	//var gasUsed u64
	//errmsg := C.Buffer{}
	//res, err := C.query(cache.ptr, id, m, db, a, q, u64(gasLimit), &gasUsed, &errmsg)
	//if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
	//	// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
	//	return nil, uint64(gasUsed), errorWithMessage(err, errmsg)
	//}
	//return receiveVector(res), uint64(gasUsed), nil
	return nil, 0, nil
}

func AnalyzeCode(
	cache Cache,
	codeHash []byte,
) (*v1types.AnalysisReport, error) {
	//cs := sendSlice(codeHash)
	//defer runtime.KeepAlive(codeHash)
	//errMsg := C.Buffer{}
	//report, err := C.analyze_code(cache.ptr, cs, &errMsg)
	//
	//if err != nil {
	//	return nil, errorWithMessage(err, errMsg)
	//}
	//res := v1types.AnalysisReport{
	//	HasIBCEntryPoints: bool(report.has_ibc_entry_points),
	//	RequiredFeatures:  string(receiveVector(report.required_features)),
	//}
	//return &res, nil
	return nil, nil
}

// KeyGen Send KeyGen request to enclave
func KeyGen() ([]byte, error) {
	//errmsg := C.Buffer{}
	//res, err := C.key_gen(&errmsg)
	//if err != nil {
	//	return nil, errorWithMessage(err, errmsg)
	//}
	//return receiveVector(res), nil
	return nil, nil
}

// KeyGen Seng KeyGen request to enclave
func CreateAttestationReport(apiKey []byte, no_epid bool, no_dcap bool, is_migration_report bool) (bool, error) {
	//errmsg := C.Buffer{}
	//_, err := C.create_attestation_report(&errmsg)
	//if err != nil {
	//	return false, errorWithMessage(err, errmsg)
	//}
	return true, nil
}

func GetEncryptedSeed(cert []byte) ([]byte, error) {
	//errmsg := C.Buffer{}
	//certSlice := sendSlice(cert)
	//defer freeAfterSend(certSlice)
	//res, err := C.get_encrypted_seed(certSlice, &errmsg)
	//if err != nil {
	//	return nil, errorWithMessage(err, errmsg)
	//}
	//return receiveVector(res), nil
	return nil, nil
}

func GetEncryptedGenesisSeed(cert []byte) ([]byte, error) {
	return nil, nil
}

/**** To error module ***/

//func errorWithMessage(err error, b C.Buffer) error {
//	//msg := receiveVector(b)
//	//if msg == nil {
//	//	return err
//	//}
//	//return fmt.Errorf("%s", string(msg))
//	return fmt.Errorf("heelo")
//}

func OnUpgradeProposalPassed(mrEnclaveHash []byte) error {
	return nil
}
