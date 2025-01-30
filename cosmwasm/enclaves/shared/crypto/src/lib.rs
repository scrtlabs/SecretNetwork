#![cfg_attr(not(target_env = "sgx"), no_std)]
#![cfg_attr(target_env = "sgx", feature(rustc_private))]

extern crate sgx_trts;
extern crate sgx_types;

// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
extern crate alloc;
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

pub mod consts;
mod errors;
pub(crate) mod kdf;
mod keys;
mod storage;
pub mod traits;

// mod aes_gcm;
mod aes_siv;
pub mod ed25519;
mod hmac;
pub mod secp256k1;

pub mod dcap;

mod rng;

pub mod hash;

pub use errors::{CryptoError, WasmApiCryptoError};
pub use keys::{AESKey, Seed, SymmetricKey, SEED_KEY_SIZE};

pub use ed25519::{Ed25519PublicKey, KeyPair, PUBLIC_KEY_SIZE, SECRET_KEY_SIZE};

pub use hash::sha::{sha_256, HASH_SIZE};
pub use traits::{Encryptable, Hmac, Kdf, SIVEncryptable, SealedKey, HMAC_SIGNATURE_SIZE};

pub use kdf::hkdf_sha_256;

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
        let failures = 0;

        count_failures!(failures, {
            // todo: add encryption and other tests here
        });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }
}
