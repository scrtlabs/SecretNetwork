use crate::error::Error;
use crate::key_manager::{PUBLIC_KEY_SIZE, SEED_SIZE, utils};
use crate::encryption::{encrypt_deoxys, decrypt_deoxys};
use sgx_types::SgxResult;
use std::vec::Vec;

pub const PRIVATE_KEY_SIZE: usize = 32;

/// RegistrationKey handles all operations with registration key such as derivation of public key,
/// derivation of encryption key, etc.
pub struct RegistrationKey {
    inner: x25519_dalek::StaticSecret,
}

impl RegistrationKey {
    /// Generates public key for seed sharing
    pub fn public_key(&self) -> x25519_dalek::PublicKey {
        x25519_dalek::PublicKey::from(&self.inner)
    }

    /// Generates random registration key
    pub fn random() -> SgxResult<Self> {
        let random_key = utils::random_bytes32()?;

        Ok( Self {
            inner: x25519_dalek::StaticSecret::from(random_key),
        })
    }

    /// Performs Diffie-Hellman derivation of encryption key for master key encryption
    /// * public_key - User public key
    pub fn diffie_hellman(
        &self,
        public_key: x25519_dalek::PublicKey,
    ) -> x25519_dalek::SharedSecret {
        self.inner.diffie_hellman(&public_key)
    }
}

/// TransactionEncryptionKey is used to decrypt incoming transaction data and to encrypt enclave output
pub struct TransactionEncryptionKey {
    inner: x25519_dalek::StaticSecret,
}

impl TransactionEncryptionKey {
    pub fn encrypt(
        &self,
        user_public_key: Vec<u8>,
        plaintext: Vec<u8>,
        salt: Vec<u8>,
    ) -> Result<Vec<u8>, Error> {
        // Check if user_public_key has correct length
        if user_public_key.len() != PUBLIC_KEY_SIZE {
            return Err(Error::encryption_err(format!(
                "[Encryption] Got public key with incorrect length. Expected: {:?}, Got: {:?}",
                user_public_key.len(),
                PUBLIC_KEY_SIZE
            )));
        }

        let public_key: [u8; PUBLIC_KEY_SIZE] = user_public_key.as_slice().try_into().map_err(|_| {
            Error::encryption_err("[Encryption] Wrong public key size")
        })?;

        let public_key = x25519_dalek::PublicKey::from(public_key);
        // Derive shared key
        let shared_key = self.inner.diffie_hellman(&public_key);
        // Derive encryption key from shared key
        let encryption_key = utils::derive_key(shared_key.as_bytes(), b"IOEncryptionKeyV1");

        encrypt_deoxys(&encryption_key, plaintext, Some(salt))
    }

    pub fn decrypt(&self, user_public_key: Vec<u8>, ciphertext: Vec<u8>) -> Result<Vec<u8>, Error> {
        // Check if user_public_key has correct length
        if user_public_key.len() != PUBLIC_KEY_SIZE {
            return Err(Error::decryption_err(format!(
                "[Encryption] Got public key with incorrect length. Expected: {:?}, Got: {:?}",
                user_public_key.len(),
                PUBLIC_KEY_SIZE
            )));
        }

        let public_key: [u8; PUBLIC_KEY_SIZE] = user_public_key.as_slice().try_into().map_err(|_| {
            Error::encryption_err("[Encryption] Wrong public key size")
        })?;

        let public_key = x25519_dalek::PublicKey::from(public_key);
        // Derive shared key
        let shared_key = self.inner.diffie_hellman(&public_key);
        // Derive encryption key from shared key
        let encryption_key = utils::derive_key(shared_key.as_bytes(), b"IOEncryptionKeyV1");

        decrypt_deoxys(&encryption_key, ciphertext)
    }

    pub fn public_key(&self) -> Vec<u8> {
        let public_key = x25519_dalek::PublicKey::from(&self.inner);
        public_key.as_bytes().to_vec()
    }
}

impl From<[u8; SEED_SIZE]> for TransactionEncryptionKey {
    fn from(input: [u8; SEED_SIZE]) -> Self {
        Self {
            inner: x25519_dalek::StaticSecret::from(input)
        }
    }
}

/// StateEncryptionKey is used to encrypt and decrypt storage values
pub struct StateEncryptionKey {
    inner: [u8; PRIVATE_KEY_SIZE],
}

impl StateEncryptionKey {
    /// Encrypts provided storage value using encryption key, derived from
    /// StateEncryptionKey and contract address using KDF. Therefore, each contract state
    /// is encrypted using unique key.
    pub fn encrypt(
        &self,
        contract_address: Vec<u8>,
        encryption_salt: Vec<u8>,
        storage_value: Vec<u8>
    ) -> Result<Vec<u8>, Error> {
        // Derive encryption key for this contract
        let contract_key = utils::derive_key(&self.inner, &contract_address);
        // Encrypt contract state using contract encryption key
        encrypt_deoxys(&contract_key, storage_value, Some(encryption_salt))
    }

    /// Decrypts provided storage value using encryption key, derived from StateEncryptionKey
    /// and contract address.
    pub fn decrypt(
        &self,
        contract_address: Vec<u8>,
        encrypted_storage_value: Vec<u8>,
    ) -> Result<Vec<u8>, Error> {
        // Derive encryption key for this contract
        let contract_key = utils::derive_key(&self.inner, &contract_address);
        // Decrypt contract state using contract encryption key
        decrypt_deoxys(&contract_key, encrypted_storage_value)
    }
}

impl From<[u8; SEED_SIZE]> for StateEncryptionKey {
    fn from(input: [u8; SEED_SIZE]) -> Self {
        Self { inner: input }
    }
}
