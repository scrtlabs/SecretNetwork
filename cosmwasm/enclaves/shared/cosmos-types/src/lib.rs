// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

pub mod multisig;
pub mod single_address;
pub mod traits;
pub mod types;

#[cfg(feature = "test")]
pub mod tests {
    use crate::multisig;

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
            multisig::tests_decode_multisig_signature::test_decode_sig_sanity();
            multisig::tests_decode_multisig_signature::test_decode_long_leb128();
            multisig::tests_decode_multisig_signature::test_decode_wrong_long_leb128();
            multisig::tests_decode_multisig_signature::test_decode_malformed_sig_only_prefix();
            multisig::tests_decode_multisig_signature::test_decode_sig_length_zero();
            multisig::tests_decode_multisig_signature::test_decode_malformed_sig_wrong_length();
        });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }
}
