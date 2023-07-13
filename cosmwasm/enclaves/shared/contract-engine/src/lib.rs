#![feature(stmt_expr_attributes)]
#![feature(vec_into_raw_parts)]

// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate sgx_types;

mod contract_operations;
mod contract_validation;
mod cosmwasm_config;
mod db;
mod errors;
mod execute_message;
pub mod external;
mod gas;
mod ibc_denom_utils;
mod ibc_message;
mod input_validation;
mod io;
mod message;
mod message_utils;
mod query_chain;
mod random;
mod reply_message;
pub(crate) mod types;
#[cfg(feature = "wasm3")]
mod wasm3;

pub use contract_operations::{handle, init, query};
pub use contract_validation::check_msg_in_current_block;

#[cfg(feature = "test")]
pub mod tests {
    use crate::types;

    /// Catch failures like the standard test runner, and print similar information per test.
    /// Tests can only fail by panicking, not by returning a `Result` type.
    #[macro_export]
    macro_rules! count_failures {
        ( $counter: ident, { $($test: expr;)* } ) => {
            $(
                print!("test {} ... ", std::stringify!($test));
                match std::panic::catch_unwind(|| $test) {
                    Ok(_) => println!("ok"),
                    Err(_) => {
                        $counter += 1;
                        println!("FAILED");
                    }
                }
            )*
        }
    }

    pub fn run_tests() {
        println!();
        let mut failures = 0;

        count_failures!(failures, {
            types::tests::test_new_from_slice();
        });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }
}
