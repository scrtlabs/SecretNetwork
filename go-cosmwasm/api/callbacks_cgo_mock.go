// +build secretcli

package api

//
///*
//#include "bindings.h"
//#include <stdio.h>
//
//// imports (db)
//GoResult cSet(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer val);
//GoResult cGet(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer *val);
//GoResult cDelete(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key);
//GoResult cScan(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer start, Buffer end, int32_t order, GoIter *out);
//// imports (iterator)
//GoResult cNext(iterator_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer *key, Buffer *val);
//// imports (api)
//GoResult cHumanAddress(api_t *ptr, Buffer canon, Buffer *human);
//GoResult cCanonicalAddress(api_t *ptr, Buffer human, Buffer *canon);
//// imports (querier)
//GoResult cQueryExternal(querier_t *ptr, uint64_t *used_gas, Buffer request, Buffer *result);
//
//// Gateway functions (db)
//GoResult cGet_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer *val) {
//	return cGet(ptr, gas_meter, used_gas, key, val);
//}
//GoResult cSet_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key, Buffer val) {
//	return cSet(ptr, gas_meter, used_gas, key, val);
//}
//GoResult cDelete_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer key) {
//	return cDelete(ptr, gas_meter, used_gas, key);
//}
//GoResult cScan_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer start, Buffer end, int32_t order, GoIter *out) {
//	return cScan(ptr, gas_meter, used_gas, start, end, order, out);
//}
//
//// Gateway functions (iterator)
//GoResult cNext_cgo(iterator_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, Buffer *key, Buffer *val) {
//	return cNext(ptr, gas_meter, used_gas, key, val);
//}
//
//// Gateway functions (api)
//GoResult cCanonicalAddress_cgo(api_t *ptr, Buffer human, Buffer *canon) {
//    return cCanonicalAddress(ptr, human, canon);
//}
//GoResult cHumanAddress_cgo(api_t *ptr, Buffer canon, Buffer *human) {
//    return cHumanAddress(ptr, canon, human);
//}
//
//// Gateway functions (querier)
//GoResult cQueryExternal_cgo(querier_t *ptr, uint64_t *used_gas, Buffer request, Buffer *result) {
//    return cQueryExternal(ptr, used_gas, request, result);
//}
//*/
//import "C"
//
//// We need these gateway functions to allow calling back to a go function from the c code.
//// At least I didn't discover a cleaner way.
//// Also, this needs to be in a different file than `callbacks.go`, as we cannot create functions
//// in the same file that has //export directives. Only import header types
