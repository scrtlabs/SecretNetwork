use super::rng::rand_slice;
use enclave_ffi_types::CryptoError;
use log::*;

pub const SEED_KEY_SIZE: usize = 32;

pub const PUBLIC_KEY_SIZE: usize = 64;
/// The size of the symmetric 256 bit key we use for encryption (in bytes).
pub const SYMMETRIC_KEY_SIZE: usize = 256 / 8;
/// The size of the master seed
pub const SEED_SIZE: usize = 32;
/// The size of secret keys

/// symmetric key we use for encryption.
pub type SymmetricKey = [u8; SYMMETRIC_KEY_SIZE];
/// StateKey is the key used for state encryption.
pub type StateKey = SymmetricKey;
/// DHKey is the key that results from the ECDH [`enigma_crypto::KeyPair::derive_key`](../replace_me)
pub type DhKey = SymmetricKey;

// pub type PubKey = [u8; COMPRESSED_PUBLIC_KEY_SIZE];

#[derive(Debug, Clone, Copy)]
pub struct AESKey(SymmetricKey);

impl AESKey {
    pub fn get(&self) -> &[u8; SYMMETRIC_KEY_SIZE] {
        return &self.0;
    }

    pub fn new_from_slice(privkey: &[u8; SYMMETRIC_KEY_SIZE]) -> Self {
        let mut key = [0u8; 32];
        key.clone_from_slice(privkey);
        Self { 0: key }
    }
}

#[derive(Debug, Clone, Copy)]
pub struct Seed([u8; SEED_SIZE]);

impl Seed {
    pub fn get(&self) -> &[u8; SEED_SIZE] {
        return &self.0;
    }

    pub fn new_from_slice(s: &[u8; SEED_SIZE]) -> Self {
        Self { 0: s.clone() }
    }

    pub fn new() -> Result<Self, CryptoError> {
        let mut sk_slice = [0; SEED_SIZE];
        rand_slice(&mut sk_slice)?;
        Ok(Self::new_from_slice(&sk_slice))
    }
}
