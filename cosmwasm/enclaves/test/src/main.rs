mod enclave;
mod ocall_mock;

use sgx_types;
use sgx_types::{sgx_enclave_id_t, sgx_status_t, SgxResult};

use enclave::ENCLAVE_DOORBELL;

extern "C" {
    pub fn ecall_run_tests(eid: sgx_enclave_id_t, retval: *mut u32) -> sgx_status_t;
}

pub fn main() {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY).unwrap();
    let enclave = (*enclave_access_token).unwrap();

    let mut failed_tests = 0;
    let status = unsafe { ecall_run_tests(enclave.geteid(), &mut failed_tests) };
    let result = match status {
        sgx_status_t::SGX_SUCCESS => Ok(failed_tests),
        other => Err(other),
    };

    if result.is_ok() && failed_tests == 0 {
        println!("Done running tests, no errors");
    } else if result.is_ok() && failed_tests != 0 {
        println!("Done running tests, # of errors: {:?}", failed_tests)
    } else {
        println!("Error running tests: {:?}", result.unwrap_err());
    }
}
