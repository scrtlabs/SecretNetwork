#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate alloc;
extern crate core;
extern crate sgx_types;

pub mod r#const;
pub mod ecalls;

pub mod wasm_messages;

pub use wasm_messages::VERIFIED_MESSAGES;

mod txs;

#[cfg(any(feature = "verify-validator-whitelist", feature = "test"))]
pub mod validator_whitelist;

use lazy_static::lazy_static;
use log::debug;

use tendermint_light_client_verifier::types::UntrustedBlockState;
use tendermint_light_client_verifier::{ProdVerifier, Verdict};

lazy_static! {
    static ref VERIFIER: ProdVerifier = ProdVerifier::default();
}

pub fn verify_block(untrusted_block: &UntrustedBlockState) -> bool {

    #[cfg(feature = "verify-validator-whitelist")]
    if !whitelisted_validators_in_block(untrusted_block) {
        debug!("Error verifying validators in block");
        return false;
    }

    match VERIFIER.verify_commit(untrusted_block) {
        Verdict::Success => true,
        Verdict::NotEnoughTrust(_) => {
            debug!("Error verifying header - not enough trust");
            false
        }
        Verdict::Invalid(e) => {
            debug!("Error verifying header - invalid block header: {:?}", e);
            false
        }
    }
}

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
