package api

/*
#include <string.h> // memcpy
#include "bindings.h"

// memcpy helper
int64_t write_to_buffer(Buffer dest, uint8_t *data, int64_t len) {
    if (len > dest.cap) {
    	return -dest.cap;
    }
	memcpy(dest.ptr, data, len);
	return len;
}

*/
import "C"

import "unsafe"

func writeToBuffer(buf C.Buffer, data []byte) i64 {
	return C.write_to_buffer(buf, u8_ptr(unsafe.Pointer(&data[0])), i64(len(data)))
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

func receiveSlice(b C.Buffer) []byte {
	if emptyBuf(b) {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(b.ptr), cint(b.len))
	C.free_rust(b)
	return res
}

func freeAfterSend(b C.Buffer) {
	if !emptyBuf(b) {
		C.free(unsafe.Pointer(b.ptr))
	}
}

func emptyBuf(b C.Buffer) bool {
	return b.ptr == u8_ptr(nil) || b.len == usize(0) || b.cap == usize(0)
}
