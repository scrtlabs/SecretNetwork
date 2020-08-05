mod contract_operations;
mod contract_validation;
mod db;
mod errors;
mod gas;
mod io;
mod memory;
mod query_chain;
mod runtime;
mod types;

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
