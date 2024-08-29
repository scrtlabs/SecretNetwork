//go:build !nosgx
// +build !nosgx

package api

// Check https://akrennmair.github.io/golang-cgo-slides/ to learn
// how this embedded C code works.

/*
#include "bindings.h"

// typedefs for _cgo functions (db)
typedef GoError (*query_external_fn)(querier_t *ptr, U8SliceView request, UnmanagedVector *result, UnmanagedVector *errOut);
GoError cQueryExternal_cgo(querier_t *ptr, U8SliceView request, UnmanagedVector *result, UnmanagedVector *errOut);

*/
import "C"

import (
	// "encoding/hex"
	"log"
	"reflect"
	"runtime/debug"

	types "github.com/SigmaGmbH/librustgo/types"

	// "github.com/holiman/uint256"
	"unsafe"
)

// Note: we have to include all exports in the same file (at least since they both import bindings.h),
// or get odd cgo build errors about duplicate definitions

func recoverPanic(ret *C.GoError) {
	if rec := recover(); rec != nil {
		// This is used to handle ErrorOutOfGas panics.
		//
		// What we do here is something that should not be done in the first place.
		// "A panic typically means something went unexpectedly wrong. Mostly we use it to fail fast
		// on errors that shouldn’t occur during normal operation, or that we aren’t prepared to
		// handle gracefully." says https://gobyexample.com/panic.
		// And 'Ask yourself "when this happens, should the application immediately crash?" If yes,
		// use a panic; otherwise, use an error.' says this popular answer on SO: https://stackoverflow.com/a/44505268.
		// Oh, and "If you're already worrying about discriminating different kinds of panics, you've lost sight of the ball."
		// (Rob Pike) from https://eli.thegreenplace.net/2018/on-the-uses-and-misuses-of-panics-in-go/
		//
		// We don't want to import Cosmos SDK and also cannot use interfaces to detect these
		// error types (as they have no methods). So, let's just rely on the descriptive names.
		name := reflect.TypeOf(rec).Name()
		switch name {
		// These three types are "thrown" (which is not a thing in Go 🙃) in panics from the gas module
		// (https://github.com/cosmos/cosmos-sdk/blob/v0.45.4/store/types/gas.go):
		// 1. ErrorOutOfGas
		// 2. ErrorGasOverflow
		// 3. ErrorNegativeGasConsumed
		//
		// In the baseapp, ErrorOutOfGas gets special treatment:
		// - https://github.com/cosmos/cosmos-sdk/blob/v0.45.4/baseapp/baseapp.go#L607
		// - https://github.com/cosmos/cosmos-sdk/blob/v0.45.4/baseapp/recovery.go#L50-L60
		// This turns the panic into a regular error with a helpful error message.
		//
		// The other two gas related panic types indicate programming errors and are handled along
		// with all other errors in https://github.com/cosmos/cosmos-sdk/blob/v0.45.4/baseapp/recovery.go#L66-L77.
		case "ErrorOutOfGas":
			// TODO: figure out how to pass the text in its `Descriptor` field through all the FFI
			*ret = C.GoError_OutOfGas
		default:
			log.Printf("Panic in Go callback: %#v\n", rec)
			debug.PrintStack()
			*ret = C.GoError_Panic
		}
	}
}

/***** GoQuerier ******/

var querier_vtable = C.Querier_vtable{
	query_external: (C.query_external_fn)(C.cQueryExternal_cgo),
}

// BuildConnector allows rust code interact with Go
// contract: original pointer/struct referenced must live longer than C.GoQuerier struct
// since this is only used internally, we can verify the code that this is the case
func BuildConnector(q types.Connector) C.GoQuerier {
	return C.GoQuerier{
		state:  (*C.querier_t)(unsafe.Pointer(&q)),
		vtable: querier_vtable,
	}
}

func buildEmptyConnector() C.GoQuerier {
	return C.GoQuerier{
		state: nil,
		vtable: querier_vtable,
	}
}

//export cQueryExternal
func cQueryExternal(ptr *C.querier_t, request C.U8SliceView, result *C.UnmanagedVector, errOut *C.UnmanagedVector) (ret C.GoError) {
	defer recoverPanic(&ret)

	if result == nil || errOut == nil {
		// we received an invalid pointer
		return C.GoError_BadArgument
	}
	if !(*result).is_none || !(*errOut).is_none {
		panic("Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one.")
	}

	req := CopyU8Slice(request)
	querier := *(*types.Connector)(unsafe.Pointer(ptr))
	response, err := querier.Query(req)

	if err != nil {
		*errOut = NewUnmanagedVector([]byte(err.Error()))
		return C.GoError_QuerierError
	}
	*result = NewUnmanagedVector(response)

	return C.GoError_None
}
