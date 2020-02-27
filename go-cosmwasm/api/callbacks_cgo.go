package api

/*
#include "bindings.h"
#include <stdio.h>

// imports (db)
void cSet(db_t *ptr, Buffer key, Buffer val);
int64_t cGet(db_t *ptr, Buffer key, Buffer val);
// imports (api)
int32_t cHumanAddress(api_t *ptr, Buffer canon, Buffer human);
int32_t cCanonicalAddress(api_t *ptr, Buffer human, Buffer canon);

// Gateway functions (db)
int64_t cGet_cgo(db_t *ptr, Buffer key, Buffer val) {
	return cGet(ptr, key, val);
}
void cSet_cgo(db_t *ptr, Buffer key, Buffer val) {
	cSet(ptr, key, val);
}

// Gateway functions (api)
int32_t cCanonicalAddress_cgo(api_t *ptr, Buffer human, Buffer canon) {
    return cCanonicalAddress(ptr, human, canon);
}
int32_t cHumanAddress_cgo(api_t *ptr, Buffer canon, Buffer human) {
    return cHumanAddress(ptr, canon, human);
}
*/
import "C"

// We need these gateway functions to allow calling back to a go function from the c code.
// At least I didn't discover a cleaner way.
// Also, this needs to be in a different file than `callbacks.go`, as we cannot create functions
// in the same file that has //export directives. Only import header types
