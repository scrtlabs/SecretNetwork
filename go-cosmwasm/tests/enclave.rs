//! This file is a wrapper for tests running in the enclave.
use go_cosmwasm::enclave_tests::{ecall_run_tests, get_enclave, sgx_types::sgx_status_t};

/// Safe wrapper for `ecall_run_tests`.
///
/// I wanted to define this function in `cosmwasm-sgx-vm` but for some reason it kept complaining
/// that `ecall_run_tests` was not defined...
pub fn run_tests() -> sgx_status_t {
    let enclave = match get_enclave() {
        Ok(enclave) => enclave,
        Err(status) => return status,
    };
    unsafe { ecall_run_tests(enclave.geteid()) }
}

#[test]
fn test_enclave() {
    let status = run_tests();
    println!();
    println!("Enclave returned {}", status);
}
