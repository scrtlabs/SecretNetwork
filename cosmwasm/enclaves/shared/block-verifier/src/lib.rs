#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate alloc;
extern crate sgx_types;

pub mod r#const;

pub mod wasm_messages;

pub use wasm_messages::VERIFIED_MESSAGES;

mod txs;

#[cfg(any(feature = "verify-validator-whitelist", feature = "test"))]
pub mod validator_whitelist;

pub mod submit_block_signatures;
mod verify;

#[cfg(feature = "test")]
pub mod tests {
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
            // kdf::tests::test_derive_key();
            // storage::tests::test_open();
            // storage::tests::test_seal();
            crate::wasm_messages::tests::parse_tx_basic();
            crate::wasm_messages::tests::parse_tx_multiple_msg();
            crate::wasm_messages::tests::parse_tx_multiple_msg_non_wasm();
            crate::wasm_messages::tests::parse_tx_multisig();
            crate::wasm_messages::tests::check_message_is_wasm();
            crate::wasm_messages::tests::check_message_is_reg();
            crate::wasm_messages::tests::check_parse_reg_bytes();
            crate::wasm_messages::tests::check_parse_reg_from_tx();
            crate::wasm_messages::tests::test_check_message_not_wasm();
            crate::wasm_messages::tests::test_wasm_msg_tracker();
            crate::wasm_messages::tests::test_wasm_msg_tracker_multiple_msgs();
            crate::validator_whitelist::tests::test_parse_validators();
        });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }
}
