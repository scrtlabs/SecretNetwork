package api

/*
#include "bindings.h"

// typedefs for _cgo functions (db)
typedef int64_t (*read_db_fn)(db_t *ptr, Buffer key, Buffer val);
typedef void (*write_db_fn)(db_t *ptr, Buffer key, Buffer val);
// and api

// forward declarations (db)
int64_t cGet_cgo(db_t *ptr, Buffer key, Buffer val);
void cSet_cgo(db_t *ptr, Buffer key, Buffer val);
// and api
*/
import "C"

import "unsafe"

// Note: we have to include all exports in the same file (at least since they both import bindings.h),
// or get odd cgo build errors about duplicate definitions

/****** DB ********/

type KVStore interface {
	Get(key []byte) []byte
	Set(key, value []byte)
}

var db_vtable = C.DB_vtable{
	read_db:  (C.read_db_fn)(C.cGet_cgo),
	write_db: (C.write_db_fn)(C.cSet_cgo),
}

// contract: original pointer/struct referenced must live longer than C.DB struct
// since this is only used internally, we can verify the code that this is the case
func buildDB(kv KVStore) C.DB {
	return C.DB{
		state:  (*C.db_t)(unsafe.Pointer(&kv)),
		vtable: db_vtable,
	}
}

//export cGet
func cGet(ptr *C.db_t, key C.Buffer, val C.Buffer) i64 {
	kv := *(*KVStore)(unsafe.Pointer(ptr))
	k := receiveSlice(key)
	v := kv.Get(k)
	if len(v) == 0 {
		return 0
	}
	return writeToBuffer(val, v)
}

//export cSet
func cSet(ptr *C.db_t, key C.Buffer, val C.Buffer) {
	kv := *(*KVStore)(unsafe.Pointer(ptr))
	k := receiveSlice(key)
	v := receiveSlice(val)
	kv.Set(k, v)
}
