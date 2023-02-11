// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
// use std::ffi::c_void;
// use std::panic;
// use std::sync::SgxMutex;

// use lazy_static::lazy_static;
// use log::*;

// use sgx_types::sgx_status_t;
pub mod report;
pub mod cert;

use enclave_ffi_types::HealthCheckResult;
use report::AttestationReport;
/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_health_check() -> HealthCheckResult {
    HealthCheckResult::Success
}
