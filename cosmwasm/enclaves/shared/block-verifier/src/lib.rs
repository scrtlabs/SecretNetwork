#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate sgx_types;

pub mod r#const;
pub mod ecalls;

#[cfg(all(feature = "SGX_MODE_HW", feature = "production", not(feature = "test")))]
pub mod validator_whitelist;
pub mod storage;
pub mod cosmos;

use lazy_static::lazy_static;
use log::debug;

use tendermint_light_client_verifier::types::UntrustedBlockState;
use tendermint_light_client_verifier::{ProdVerifier, Verdict};

lazy_static! {
    static ref VERIFIER: ProdVerifier = ProdVerifier::default();
}

pub fn verify_block(untrusted_block: &UntrustedBlockState) -> bool {

    #[cfg(all(feature = "SGX_MODE_HW", feature = "production", not(feature = "test")))]
    if !whitelisted_validators_in_block(untrusted_block) {
        debug!("Error verifying validators in block");
        return false;
    }

    match VERIFIER.verify_commit(untrusted_block) {
        Verdict::Success => true,
        Verdict::NotEnoughTrust(_) => {
            debug!("Error verifying header - not enough trust");
            false
        },
        Verdict::Invalid(e) => {
            debug!("Error verifying header - invalid block header: {:?}", e);
            false
        },
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
            crate::cosmos::tests::it_works();
        });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }
}
