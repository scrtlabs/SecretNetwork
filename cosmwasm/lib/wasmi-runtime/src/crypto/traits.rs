use crate::crypto::keys::DhKey;
use enclave_ffi_types::{CryptoError, EnclaveError};

pub const HMAC_SIGNATURE_SIZE: usize = 32;

pub trait Encryptable {
    fn encrypt(&self, plaintext: &[u8]) -> Result<Vec<u8>, CryptoError>;
    fn decrypt(&self, ciphertext: &[u8]) -> Result<Vec<u8>, CryptoError>;
}

pub trait SIVEncryptable {
    fn encrypt_siv(&self, plaintext: &[u8], ad: &Vec<&[u8]>) -> Result<Vec<u8>, CryptoError>;
    fn decrypt_siv(&self, plaintext: &[u8], ad: &Vec<&[u8]>) -> Result<Vec<u8>, CryptoError>;
}

pub trait SealedKey
where
    Self: std::marker::Sized,
{
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError>;
    fn unseal(filepath: &str) -> Result<Self, EnclaveError>;
}

pub trait Rng {
    fn rand_slice(buf: &mut [u8]) -> Result<(), CryptoError>;
}

pub trait Kdf {
    fn derive_key_from_this(&self, data: &[u8]) -> Self;
}

pub trait Hmac {
    fn sign_sha_256(&self, to_sign: &[u8]) -> [u8; HMAC_SIGNATURE_SIZE];
}

// pub trait Ecdh {
//     fn new() -> Result<Self, CryptoError> {}
//     fn new_from_slice(privkey: &[u8]) -> Result<Self, CryptoError> {}
//     fn derive_key(&self, your_public: &[u8]) -> DhKey {}
// }
