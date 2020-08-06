// +build !secretcli

package api

/*
#include "bindings.h"
*/
import "C"

import "unsafe"

func allocateRust(data []byte) C.Buffer {
	var ret C.Buffer
	if data == nil {
		// Just return a null buffer
		ret = C.Buffer{
			ptr: u8_ptr(nil),
			len: usize(0),
			cap: usize(0),
		}
		// in Go, accessing the 0-th element of an empty array triggers a panic. That is why in the case
		// of an empty `[]byte` we can't get the internal heap pointer to the underlying array as we do
		// below with `&data[0]`.
		// https://play.golang.org/p/xvDY3g9OqUk
		// Additionally, the pointer field in a Rust vector is a NonNull pointer. This means that when
		// the vector is empty and no heap allocation is made, it needs to put _some_ value there instead.
		// At the time of writing, it uses the alignment of the generic type T, which in this case equals 1.
		// But because that is an internal detail that we can't rely on in future versions, we still call out
		// to Rust and ask it to build an empty vector for us.
		// https://play.rust-lang.org/?version=stable&mode=debug&edition=2018&gist=01ced0731171c8226e2c28634a7e41d7
	} else if len(data) == 0 {
		// This will create an empty vector
		ret = C.allocate_rust(u8_ptr(nil), usize(0))
	} else {
		// This will allocate a proper vector with content and return a description of it
		ret = C.allocate_rust(u8_ptr(unsafe.Pointer(&data[0])), usize(len(data)))
	}
	return ret
}

func sendSlice(s []byte) C.Buffer {
	if s == nil {
		return C.Buffer{ptr: u8_ptr(nil), len: usize(0), cap: usize(0)}
	}
	return C.Buffer{
		ptr: u8_ptr(C.CBytes(s)),
		len: usize(len(s)),
		cap: usize(len(s)),
	}
}

// Take an owned vector that was passed to us, copy it, and then free it on the Rust side.
// This should only be used for vectors that will never be observed again on the Rust side
func receiveVector(b C.Buffer) []byte {
	if bufIsNil(b) {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(b.ptr), cint(b.len))
	C.free_rust(b)
	return res
}

// Copy the contents of a vector that was allocated on the Rust side.
// Unlike receiveVector, we do not free it, because it will be manually
// freed on the Rust side after control returns to it.
//This should be used in places like callbacks from Rust to Go.
func receiveSlice(b C.Buffer) []byte {
	if bufIsNil(b) {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(b.ptr), cint(b.len))
	return res
}

func freeAfterSend(b C.Buffer) {
	if !bufIsNil(b) {
		C.free(unsafe.Pointer(b.ptr))
	}
}

func bufIsNil(b C.Buffer) bool {
	return b.ptr == u8_ptr(nil)
}
