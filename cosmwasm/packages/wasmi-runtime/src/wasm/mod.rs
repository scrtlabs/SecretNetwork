mod contract_operations;
mod contract_validation;
mod db;
mod errors;
mod gas;
mod io;
mod memory;
mod runtime;
mod types;

pub use contract_operations::{handle, init, query};

#[cfg(feature = "test")]
pub mod tests {
    use super::*;
    pub fn run_tests() {
        types::tests::test_new_from_slice();
        // types::tests::test_msg_decrypt();
    }
}
