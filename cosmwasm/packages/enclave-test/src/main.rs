//! This file is a wrapper for tests running in the enclave.
use cosmwasm_sgx_vm::enclave_tests::run_tests;

fn main() {
    let status = run_tests();
    println!("Enclave returned {}", status);
}
