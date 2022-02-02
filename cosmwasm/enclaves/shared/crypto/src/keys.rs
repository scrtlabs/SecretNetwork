use super::rng::rand_slice;
use super::traits::{AlignedMemory, ExportECKey, EC_256_PRIVATE_KEY_SIZE};

use crate::errors::CryptoError;

use crate::ed25519::Ed25519PrivateKey;
use sgx_types::sgx_align_ec256_private_t;

pub const SEED_KEY_SIZE: usize = 32;

/// The size of the symmetric 256 bit key we use for encryption (in bytes).
pub const SYMMETRIC_KEY_SIZE: usize = 256 / 8;
/// The size of the master seed
//pub const SEED_SIZE: usize = 32;
/// The size of secret keys

type AlignedKey = sgx_align_ec256_private_t;
type AlignedSeed = sgx_align_ec256_private_t;

/// symmetric key we use for encryption.
pub type SymmetricKey = [u8; SYMMETRIC_KEY_SIZE];
/// StateKey is the key used for state encryption.
// pub type StateKey = SymmetricKey;
/// DHKey is the key that results from the ECDH [`enigma_crypto::KeyPair::derive_key`](../replace_me)
pub type DhKey = SymmetricKey;

// pub type PubKey = [u8; COMPRESSED_PUBLIC_KEY_SIZE];

// #[derive(Debug, Clone, Copy)]
// pub struct AESKey(SymmetricKey);

#[repr(C, align(64))]
#[derive(Clone, Copy, Default)]
pub struct AESKey {
    pub key: AlignedKey,
}

impl AESKey {
    #[allow(dead_code)]
    fn key_len() -> usize {
        SYMMETRIC_KEY_SIZE
    }

    pub fn get(&self) -> &[u8; SYMMETRIC_KEY_SIZE] {
        &self.key.key.r as &[u8; SYMMETRIC_KEY_SIZE]
    }

    pub fn new_from_slice(privkey: &[u8; SYMMETRIC_KEY_SIZE]) -> Self {
        let mut key = AESKey::default();

        key.as_mut().copy_from_slice(privkey);

        key
    }
}

impl AsMut<[u8; SYMMETRIC_KEY_SIZE]> for AESKey {
    fn as_mut(&mut self) -> &mut [u8; SYMMETRIC_KEY_SIZE] {
        &mut self.key.key.r as &mut [u8; SYMMETRIC_KEY_SIZE]
    }
}

impl AlignedMemory for AESKey {}
impl ExportECKey for AESKey {
    fn key_ref(&self) -> &[u8; EC_256_PRIVATE_KEY_SIZE] {
        self.get()
    }
}

impl From<Ed25519PrivateKey> for AESKey {
    fn from(value: Ed25519PrivateKey) -> Self {
        let mut secret_key = AESKey::default();
        secret_key.as_mut().copy_from_slice(value.key_ref());

        secret_key
    }
}

#[repr(C, align(64))]
#[derive(Clone, Copy, Default)]
pub struct Seed {
    pub key: AlignedSeed,
}

impl Seed {
    #[allow(dead_code)]
    fn key_len() -> usize {
        SEED_KEY_SIZE
    }

    pub fn as_slice(&self) -> &[u8; SEED_KEY_SIZE] {
        &self.key.key.r as &[u8; SEED_KEY_SIZE]
    }

    pub fn new() -> Result<Self, CryptoError> {
        let mut seed = Seed::default();

        rand_slice(seed.as_mut())?;
        Ok(seed)
    }
}

impl AsMut<[u8; SEED_KEY_SIZE]> for Seed {
    fn as_mut(&mut self) -> &mut [u8; SEED_KEY_SIZE] {
        &mut self.key.key.r as &mut [u8; SEED_KEY_SIZE]
    }
}

impl From<Ed25519PrivateKey> for Seed {
    fn from(value: Ed25519PrivateKey) -> Self {
        let mut secret_key = Seed::default();
        secret_key.as_mut().copy_from_slice(value.key_ref());

        secret_key
    }
}

impl AlignedMemory for Seed {}
