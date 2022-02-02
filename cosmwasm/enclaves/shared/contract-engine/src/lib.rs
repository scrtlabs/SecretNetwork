// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate sgx_types;

mod contract_operations;
mod contract_validation;
mod db;
mod errors;
pub mod external;
mod gas;
mod io;
mod memory;
pub(crate) mod module_cache;
mod query_chain;
pub(crate) mod types;
mod wasm;

pub use contract_operations::{handle, init, query};

#[cfg(feature = "test")]
pub mod tests {
    use super::*;
    use crate::count_failures;

    pub fn run_tests() {
        println!();
        let mut failures = 0;

        count_failures!(failures, {
            types::tests::test_new_from_slice();
            // types::tests::test_msg_decrypt();
        });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }
}
