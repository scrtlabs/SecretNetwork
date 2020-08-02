use sgx_types;
use sgx_types::{sgx_enclave_id_t, sgx_status_t};

use crate::enclave::get_enclave;

extern "C" {
    pub fn ecall_run_tests(eid: sgx_enclave_id_t) -> sgx_status_t;
}

pub fn run_tests() -> sgx_status_t {
    let enclave = match get_enclave() {
        Ok(enclave) => enclave,
        Err(status) => return status,
    };
    unsafe { ecall_run_tests(enclave.geteid()) }
}
