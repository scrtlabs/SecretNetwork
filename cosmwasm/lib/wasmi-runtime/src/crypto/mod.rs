mod kdf;
pub mod key_manager;
mod keys;
mod storage;
pub mod traits;

// mod aes_gcm;
mod aes_siv;
mod hmac;
mod rng;
mod sha;

mod ed25519;
mod secp256k1;

pub use key_manager::Keychain;
pub use key_manager::KEY_MANAGER;
pub use keys::{AESKey, Seed, SymmetricKey, SEED_KEY_SIZE};

pub use ed25519::{Ed25519PublicKey, KeyPair, PUBLIC_KEY_SIZE, SECRET_KEY_SIZE};

pub use sha::{sha_256, HASH_SIZE};
pub use traits::{Encryptable, Hmac, Kdf, SIVEncryptable, SealedKey, HMAC_SIGNATURE_SIZE};
