//go:build nosgx
// +build nosgx

package api

/*
#include "bindings.h"
#include <stdio.h>

// imports (querier)
GoError cQueryExternal(querier_t *ptr, U8SliceView request, UnmanagedVector *result, UnmanagedVector *errOut);

// Gateway functions (querier)
GoError cQueryExternal_cgo(querier_t *ptr, U8SliceView request, UnmanagedVector *result, UnmanagedVector *errOut) {
    return cQueryExternal(ptr, request, result, errOut);
}

*/
import "C"

// We need these gateway functions to allow calling back to a go function from the c code.
// At least I didn't discover a cleaner way.
// Also, this needs to be in a different file than `callbacks.go`, as we cannot create functions
// in the same file that has //export directives. Only import header types
