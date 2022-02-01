// similar trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;
// This annotation is here to trick the IDE into ignoring the extern crate, and instead pull in sgx_types from our
// Cargo.toml. By importing sgx_types using `extern crate` but without letting it resolve in Cargo.toml when compiling
// to SGX, we make the compiler pull it in from the target root, which contains the sgx_types listed in Xargo.toml.
// This in turn silences errors about using the same types from two versions of the same crate.
// (go ahead, try to remove this block and change the Cargo.toml import to a normal one)
#[cfg(target_env = "sgx")]
extern crate sgx_types;

mod errors;
pub(crate) mod kdf;
pub mod key_manager;
mod keys;
mod storage;
pub mod traits;

// mod aes_gcm;
mod aes_siv;
pub mod hash;
mod hmac;
mod rng;

mod consts;
mod ed25519;
pub mod secp256k1;

pub use errors::CryptoError;
pub use key_manager::Keychain;
pub use key_manager::KEY_MANAGER;
pub use keys::{AESKey, Seed, SymmetricKey, SEED_KEY_SIZE};

pub use ed25519::{Ed25519PublicKey, KeyPair, PUBLIC_KEY_SIZE, SECRET_KEY_SIZE};

pub use hash::sha::{sha_256, HASH_SIZE};
pub use traits::{Encryptable, Hmac, Kdf, SIVEncryptable, SealedKey, HMAC_SIGNATURE_SIZE};

#[cfg(feature = "test")]
pub mod tests {
    use super::*;
    use crate::count_failures;

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
