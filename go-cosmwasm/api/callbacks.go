// +build !secretcli

package api

/*
#include "bindings.h"

// typedefs for _cgo functions (db)
typedef GoResult (*read_db_fn)(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer *val, Buffer *errOut);
typedef GoResult (*write_db_fn)(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer val, Buffer *errOut);
typedef GoResult (*remove_db_fn)(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer *errOut);
typedef GoResult (*scan_db_fn)(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer start, Buffer end, int32_t order, GoIter *out, Buffer *errOut);
// iterator
typedef GoResult (*next_db_fn)(iterator_t idx, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer *key, Buffer *val, Buffer *errOut);
// and api
typedef GoResult (*humanize_address_fn)(api_t *ptr, Buffer canon, Buffer *human, Buffer *errOut, uint64_t *used_gas);
typedef GoResult (*canonicalize_address_fn)(api_t *ptr, Buffer human, Buffer *canon, Buffer *errOut, uint64_t *used_gas);
typedef GoResult (*query_external_fn)(querier_t *ptr, uint64_t gas_limit, uint64_t *used_gas, Buffer request, Buffer *result, Buffer *errOut);

// forward declarations (db)
GoResult cGet_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer *val, Buffer *errOut);
GoResult cSet_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer val, Buffer *errOut);
GoResult cDelete_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer *errOut);
GoResult cScan_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer start, Buffer end, int32_t order, GoIter *out, Buffer *errOut);
// iterator
GoResult cNext_cgo(iterator_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer *key, Buffer *val, Buffer *errOut);
// api
GoResult cHumanAddress_cgo(api_t *ptr, Buffer canon, Buffer *human, Buffer *errOut, uint64_t *used_gas);
GoResult cCanonicalAddress_cgo(api_t *ptr, Buffer human, Buffer *canon, Buffer *errOut, uint64_t *used_gas);
// and querier
GoResult cQueryExternal_cgo(querier_t *ptr, uint64_t gas_limit, uint64_t *used_gas, Buffer request, Buffer *result, Buffer *errOut);


*/
import "C"

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"unsafe"

	dbm "github.com/tendermint/tm-db"

	"github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
)

// Note: we have to include all exports in the same file (at least since they both import bindings.h),
// or get odd cgo build errors about duplicate definitions

func recoverPanic(ret *C.GoResult) {
	rec := recover()
	// we don't want to import cosmos-sdk
	// we also cannot use interfaces to detect these error types (as they have no methods)
	// so, let's just rely on the descriptive names
	// this is used to detect "out of gas panics"
	if rec != nil {
		name := reflect.TypeOf(rec).Name()
		switch name {
		// These two cases are for types thrown in panics from this module:
		// https://github.com/cosmos/cosmos-sdk/blob/4ffabb65a5c07dbb7010da397535d10927d298c1/store/types/gas.go
		// ErrorOutOfGas needs to be propagated through the rust code and back into go code, where it should
		// probably be thrown in a panic again.
		// TODO figure out how to pass the text in its `Descriptor` field through all the FFI
		// TODO handle these cases on the Rust side in the first place
		case "ErrorOutOfGas":
			*ret = C.GoResult_OutOfGas
		// Looks like this error is not treated specially upstream:
		// https://github.com/cosmos/cosmos-sdk/blob/4ffabb65a5c07dbb7010da397535d10927d298c1/baseapp/baseapp.go#L818-L853
		// but this needs to be periodically verified, in case they do start checking for this type
		// 	case "ErrorGasOverflow":
		default:
			log.Printf("Panic in Go callback: %#v\n", rec)
			*ret = C.GoResult_Panic
		}
	}
}

type Gas = uint64

// GasMeter is a copy of an interface declaration from cosmos-sdk
// https://github.com/cosmos/cosmos-sdk/blob/18890a225b46260a9adc587be6fa1cc2aff101cd/store/types/gas.go#L34
type GasMeter interface {
	GasConsumed() Gas
}

/****** DB ********/

// KVStore copies a subset of types from cosmos-sdk
// We may wish to make this more generic sometime in the future, but not now
// https://github.com/cosmos/cosmos-sdk/blob/bef3689245bab591d7d169abd6bea52db97a70c7/store/types/store.go#L170
type KVStore interface {
	Get(key []byte) []byte
	Set(key, value []byte)
	Delete(key []byte)

	// Iterator over a domain of keys in ascending order. End is exclusive.
	// Start must be less than end, or the Iterator is invalid.
	// Iterator must be closed by caller.
	// To iterate over entire domain, use store.Iterator(nil, nil)
	Iterator(start, end []byte) dbm.Iterator

	// Iterator over a domain of keys in descending order. End is exclusive.
	// Start must be less than end, or the Iterator is invalid.
	// Iterator must be closed by caller.
	ReverseIterator(start, end []byte) dbm.Iterator
}

var db_vtable = C.DB_vtable{
	read_db:   (C.read_db_fn)(C.cGet_cgo),
	write_db:  (C.write_db_fn)(C.cSet_cgo),
	remove_db: (C.remove_db_fn)(C.cDelete_cgo),
	scan_db:   (C.scan_db_fn)(C.cScan_cgo),
}

type DBState struct {
	Store KVStore
	// IteratorStackID is used to lookup the proper stack frame for iterators associated with this DB (iterator.go)
	IteratorStackID uint64
}

// use this to create C.DB in two steps, so the pointer lives as long as the calling stack
//   state := buildDBState(kv, counter)
//   db := buildDB(&state, &gasMeter)
//   // then pass db into some FFI function
func buildDBState(kv KVStore, counter uint64) DBState {
	return DBState{
		Store:           kv,
		IteratorStackID: counter,
	}
}

// contract: original pointer/struct referenced must live longer than C.DB struct
// since this is only used internally, we can verify the code that this is the case
func buildDB(state *DBState, gm *GasMeter) C.DB {
	return C.DB{
		gas_meter: (*C.gas_meter_t)(unsafe.Pointer(gm)),
		state:     (*C.db_t)(unsafe.Pointer(state)),
		vtable:    db_vtable,
	}
}

var iterator_vtable = C.Iterator_vtable{
	next_db: (C.next_db_fn)(C.cNext_cgo),
}

// contract: original pointer/struct referenced must live longer than C.DB struct
// since this is only used internally, we can verify the code that this is the case
func buildIterator(dbCounter uint64, it dbm.Iterator) C.iterator_t {
	idx := storeIterator(dbCounter, it)
	return C.iterator_t{
		db_counter:     u64(dbCounter),
		iterator_index: u64(idx),
	}
}

//export cGet
func cGet(ptr *C.db_t, gasMeter *C.gas_meter_t, usedGas *u64, key C.Buffer, val *C.Buffer, errOut *C.Buffer) (ret C.GoResult) {
	defer recoverPanic(&ret)
	if ptr == nil || gasMeter == nil || usedGas == nil || val == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	gm := *(*GasMeter)(unsafe.Pointer(gasMeter))
	kv := *(*KVStore)(unsafe.Pointer(ptr))
	k := receiveSlice(key)

	gasBefore := gm.GasConsumed()
	v := kv.Get(k)
	gasAfter := gm.GasConsumed()
	*usedGas = (u64)(gasAfter - gasBefore)

	// v will equal nil when the key is missing
	// https://github.com/cosmos/cosmos-sdk/blob/1083fa948e347135861f88e07ec76b0314296832/store/types/store.go#L174
	if v != nil {
		*val = allocateRust(v)
	}
	// else: the Buffer on the rust side is initialised as a "null" buffer,
	// so if we don't write a non-null address to it, it will understand that
	// the key it requested does not exist in the kv store

	return C.GoResult_Ok
}

//export cSet
func cSet(ptr *C.db_t, gasMeter *C.gas_meter_t, usedGas *C.uint64_t, key C.Buffer, val C.Buffer, errOut *C.Buffer) (ret C.GoResult) {
	defer recoverPanic(&ret)
	if ptr == nil || gasMeter == nil || usedGas == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	gm := *(*GasMeter)(unsafe.Pointer(gasMeter))
	kv := *(*KVStore)(unsafe.Pointer(ptr))
	k := receiveSlice(key)
	v := receiveSlice(val)

	gasBefore := gm.GasConsumed()
	kv.Set(k, v)
	gasAfter := gm.GasConsumed()
	*usedGas = (C.uint64_t)(gasAfter - gasBefore)

	return C.GoResult_Ok
}

//export cDelete
func cDelete(ptr *C.db_t, gasMeter *C.gas_meter_t, usedGas *C.uint64_t, key C.Buffer, errOut *C.Buffer) (ret C.GoResult) {
	defer recoverPanic(&ret)
	if ptr == nil || gasMeter == nil || usedGas == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	gm := *(*GasMeter)(unsafe.Pointer(gasMeter))
	kv := *(*KVStore)(unsafe.Pointer(ptr))
	k := receiveSlice(key)

	gasBefore := gm.GasConsumed()
	kv.Delete(k)
	gasAfter := gm.GasConsumed()
	*usedGas = (C.uint64_t)(gasAfter - gasBefore)

	return C.GoResult_Ok
}

//export cScan
func cScan(ptr *C.db_t, gasMeter *C.gas_meter_t, usedGas *C.uint64_t, start C.Buffer, end C.Buffer, order i32, out *C.GoIter, errOut *C.Buffer) (ret C.GoResult) {
	defer recoverPanic(&ret)
	if ptr == nil || gasMeter == nil || usedGas == nil || out == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	gm := *(*GasMeter)(unsafe.Pointer(gasMeter))
	state := (*DBState)(unsafe.Pointer(ptr))
	kv := state.Store
	// handle null as well as data
	var s, e []byte
	if start.ptr != nil {
		s = receiveSlice(start)
	}
	if end.ptr != nil {
		e = receiveSlice(end)
	}

	var iter dbm.Iterator
	gasBefore := gm.GasConsumed()
	switch order {
	case 1: // Ascending
		iter = kv.Iterator(s, e)
	case 2: // Descending
		iter = kv.ReverseIterator(s, e)
	default:
		return C.GoResult_BadArgument
	}
	gasAfter := gm.GasConsumed()
	*usedGas = (C.uint64_t)(gasAfter - gasBefore)

	out.state = buildIterator(state.IteratorStackID, iter)
	out.vtable = iterator_vtable
	return C.GoResult_Ok
}

//export cNext
func cNext(ref C.iterator_t, gasMeter *C.gas_meter_t, usedGas *C.uint64_t, key *C.Buffer, val *C.Buffer, errOut *C.Buffer) (ret C.GoResult) {
	// typical usage of iterator
	// 	for ; itr.Valid(); itr.Next() {
	// 		k, v := itr.Key(); itr.Value()
	// 		...
	// 	}

	defer recoverPanic(&ret)
	if ref.db_counter == 0 || gasMeter == nil || usedGas == nil || key == nil || val == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	gm := *(*GasMeter)(unsafe.Pointer(gasMeter))
	iter := retrieveIterator(uint64(ref.db_counter), uint64(ref.iterator_index))
	if !iter.Valid() {
		// end of iterator, return as no-op, nil key is considered end
		return C.GoResult_Ok
	}

	gasBefore := gm.GasConsumed()
	// call Next at the end, upon creation we have first data loaded
	k := iter.Key()
	v := iter.Value()
	// check iter.Error() ????
	iter.Next()
	gasAfter := gm.GasConsumed()
	*usedGas = (C.uint64_t)(gasAfter - gasBefore)

	if k != nil {
		*key = allocateRust(k)
		*val = allocateRust(v)
	}
	return C.GoResult_Ok
}

/***** GoAPI *******/

type HumanizeAddress func([]byte) (string, uint64, error)
type CanonicalizeAddress func(string) ([]byte, uint64, error)

type GoAPI struct {
	HumanAddress     HumanizeAddress
	CanonicalAddress CanonicalizeAddress
}

var api_vtable = C.GoApi_vtable{
	humanize_address:     (C.humanize_address_fn)(C.cHumanAddress_cgo),
	canonicalize_address: (C.canonicalize_address_fn)(C.cCanonicalAddress_cgo),
}

// contract: original pointer/struct referenced must live longer than C.GoApi struct
// since this is only used internally, we can verify the code that this is the case
func buildAPI(api *GoAPI) C.GoApi {
	return C.GoApi{
		state:  (*C.api_t)(unsafe.Pointer(api)),
		vtable: api_vtable,
	}
}

//export cHumanAddress
func cHumanAddress(ptr *C.api_t, canon C.Buffer, human *C.Buffer, errOut *C.Buffer, used_gas *u64) (ret C.GoResult) {
	defer recoverPanic(&ret)
	if human == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}
	api := (*GoAPI)(unsafe.Pointer(ptr))
	c := receiveSlice(canon)
	h, cost, err := api.HumanAddress(c)
	*used_gas = u64(cost)
	if err != nil {
		// store the actual error message in the return buffer
		*errOut = allocateRust([]byte(err.Error()))
		return C.GoResult_User
	}
	if len(h) == 0 {
		panic(fmt.Sprintf("`api.HumanAddress()` returned an empty string for %q", c))
	}
	*human = allocateRust([]byte(h))
	return C.GoResult_Ok
}

//export cCanonicalAddress
func cCanonicalAddress(ptr *C.api_t, human C.Buffer, canon *C.Buffer, errOut *C.Buffer, used_gas *u64) (ret C.GoResult) {
	defer recoverPanic(&ret)

	if canon == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	api := (*GoAPI)(unsafe.Pointer(ptr))
	h := string(receiveSlice(human))
	c, cost, err := api.CanonicalAddress(h)
	*used_gas = u64(cost)
	if err != nil {
		// store the actual error message in the return buffer
		*errOut = allocateRust([]byte(err.Error()))
		return C.GoResult_User
	}
	if len(c) == 0 {
		panic(fmt.Sprintf("`api.CanonicalAddress()` returned an empty string for %q", h))
	}
	*canon = allocateRust(c)

	// If we do not set canon to a meaningful value, then the other side will interpret that as an empty result.
	return C.GoResult_Ok
}

/****** Go Querier ********/

var querier_vtable = C.Querier_vtable{
	query_external: (C.query_external_fn)(C.cQueryExternal_cgo),
}

// contract: original pointer/struct referenced must live longer than C.GoQuerier struct
// since this is only used internally, we can verify the code that this is the case
func buildQuerier(q *Querier) C.GoQuerier {
	return C.GoQuerier{
		state:  (*C.querier_t)(unsafe.Pointer(q)),
		vtable: querier_vtable,
	}
}

//export cQueryExternal
func cQueryExternal(ptr *C.querier_t, gasLimit C.uint64_t, usedGas *C.uint64_t, request C.Buffer, result *C.Buffer, errOut *C.Buffer) (ret C.GoResult) {
	defer recoverPanic(&ret)
	if ptr == nil || usedGas == nil || result == nil {
		// we received an invalid pointer
		return C.GoResult_BadArgument
	}

	// query the data
	querier := *(*Querier)(unsafe.Pointer(ptr))
	req := receiveSlice(request)

	gasBefore := querier.GasConsumed()
	res := types.RustQuery(querier, req, uint64(gasLimit))
	gasAfter := querier.GasConsumed()
	*usedGas = (C.uint64_t)(gasAfter - gasBefore)

	// serialize the response
	bz, err := json.Marshal(res)
	if err != nil {
		*errOut = allocateRust([]byte(err.Error()))
		return C.GoResult_Other
	}
	*result = allocateRust(bz)
	return C.GoResult_Ok
}
