use sgx_types;
use sgx_types::{sgx_enclave_id_t, sgx_status_t, SgxResult};

use crate::enclave::get_enclave;

extern "C" {
    pub fn ecall_run_tests(eid: sgx_enclave_id_t, retval: *mut u32) -> sgx_status_t;
}

pub fn run_tests() -> SgxResult<u32> {
    let enclave = get_enclave()?;
    let mut failed_tests = 0;
    let status = unsafe { ecall_run_tests(enclave.geteid(), &mut failed_tests) };
    match status {
        sgx_status_t::SGX_SUCCESS => Ok(failed_tests),
        other => Err(other),
    }
}
