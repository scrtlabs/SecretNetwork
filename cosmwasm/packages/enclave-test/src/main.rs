//! This file is a wrapper for tests running in the enclave.
use cosmwasm_sgx_vm::enclave_tests::run_tests;

fn main() -> Result<(), ()> {
    match run_tests() {
        Ok(failed_tests) => {
            println!("{} tests failed in enclave test suite", failed_tests);
            match failed_tests {
                0 => Ok(()),
                _ => Err(()),
            }
        }
        Err(status) => {
            println!("Enclave returned {}", status);
            Err(())
        }
    }
}
