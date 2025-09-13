use super::keys::DhKey;

use super::traits::{AlignedMemory, ExportECKey, EC_256_PRIVATE_KEY_SIZE};
use sgx_types::sgx_align_ec256_private_t;

// use x25519_dalek;

use crate::CryptoError;

use super::rng::rand_slice;

pub const SECRET_KEY_SIZE: usize = 32;
pub const PUBLIC_KEY_SIZE: usize = 32;

type AlignedEc256PrivateKey = sgx_align_ec256_private_t;

pub type Ed25519PublicKey = [u8; 32];

#[repr(C, align(64))]
#[derive(Clone, Copy, Default)]
pub struct Ed25519PrivateKey {
    pub key: AlignedEc256PrivateKey,
}

impl Ed25519PrivateKey {
    pub fn to_owned(self) -> AlignedEc256PrivateKey {
        self.key
    }

    pub fn get_mut(&mut self) -> &mut [u8; SECRET_KEY_SIZE] {
        &mut self.key.key.r as &mut [u8; SECRET_KEY_SIZE]
    }
}

impl ExportECKey for Ed25519PrivateKey {
    fn key_ref(&self) -> &[u8; EC_256_PRIVATE_KEY_SIZE] {
        &self.key.key.r as &[u8; EC_256_PRIVATE_KEY_SIZE]
    }
}

#[derive(Clone, Copy, Default)]
pub struct KeyPair {
    secret_key: Ed25519PrivateKey,
    public_key: Ed25519PublicKey,
}

impl KeyPair {
    pub fn sk_to_pk(sk: &Ed25519PrivateKey) -> Ed25519PublicKey {
        let sk_raw = x25519_dalek::StaticSecret::from((*sk).to_owned().key.r as [u8; 32]);
        let pk = x25519_dalek::PublicKey::from(&sk_raw);
        *pk.as_bytes()
    }

    pub fn from_sk(sk: Ed25519PrivateKey) -> Self {
        Self {
            secret_key: sk,
            public_key: Self::sk_to_pk(&sk),
        }
    }

    pub fn new() -> Result<Self, CryptoError> {
        let mut secret_key = Ed25519PrivateKey::default();
        rand_slice(secret_key.get_mut())?;
        Ok(Self::from_sk(secret_key))
    }

    pub fn diffie_hellman(&self, your_public: &[u8; SECRET_KEY_SIZE]) -> DhKey {
        let my_secret =
            x25519_dalek::StaticSecret::from(self.secret_key.to_owned().key.r as [u8; 32]);
        let pk = x25519_dalek::PublicKey::from(*your_public);
        let ss = my_secret.diffie_hellman(&pk);

        *ss.as_bytes()
    }
    pub fn get_privkey(&self) -> &[u8; SECRET_KEY_SIZE] {
        self.secret_key.key_ref()
    }

    // This will return the raw 64 bytes public key.
    pub fn get_pubkey(&self) -> [u8; PUBLIC_KEY_SIZE] {
        self.public_key
    }
}

// struct AlignedEcKey<T: AlignedMemory + ExportECKey>(T);

impl AlignedMemory for Ed25519PrivateKey {}

impl<T: AlignedMemory + ExportECKey> From<T> for KeyPair {
    fn from(value: T) -> Self {
        let mut secret_key = Ed25519PrivateKey::default();
        secret_key.get_mut().copy_from_slice(value.key_ref());

        Self::from_sk(secret_key)
    }
}

#[cfg(feature = "test")]
pub mod tests {
    use super::*;
    use cosmos_proto::tx::signing::SignMode;

    pub fn test_keypair_generation() {
        // Generate a keypair
        let keypair = KeyPair::new().unwrap();
        
        // Verify the public key is not all zeros
        let public_key = keypair.public_key();
        let is_nonzero = public_key.iter().any(|&byte| byte != 0);
        assert!(is_nonzero, "Public key should not be all zeros");
        
        // Verify the secret key is not all zeros
        let secret_key = keypair.secret_key();
        let is_nonzero = secret_key.iter().any(|&byte| byte != 0);
        assert!(is_nonzero, "Secret key should not be all zeros");
        
        // Verify that creating a keypair from bytes results in the same keypair
        let secret_key_bytes = keypair.secret_key();
        let recreated_keypair = KeyPair::from_secret(&secret_key_bytes).unwrap();
        
        // Public keys should match
        assert_eq!(keypair.public_key(), recreated_keypair.public_key());
    }
    
    pub fn test_signing_and_verification() {
        // Generate a keypair
        let keypair = KeyPair::new().unwrap();
        
        // Message to sign
        let message = b"This is a test message to sign";
        
        // Sign the message
        let signature = keypair.sign(message).unwrap();
        
        // Verify the signature size is correct
        assert_eq!(signature.len(), 64); // Ed25519 signatures are 64 bytes
        
        // Get the public key
        let public_key = Ed25519PublicKey::from_slice(&keypair.public_key()).unwrap();
        
        // Verify the signature with the public key
        let result = public_key.verify_bytes(message, &signature, SignMode::SIGN_MODE_UNSPECIFIED);
        assert!(result.is_ok(), "Signature verification should succeed");
        
        // Modify the message and verify that verification fails
        let modified_message = b"This is a modified test message";
        let result = public_key.verify_bytes(modified_message, &signature, SignMode::SIGN_MODE_UNSPECIFIED);
        assert!(result.is_err(), "Signature verification should fail with modified message");
        
        // Test with a different keypair
        let different_keypair = KeyPair::new().unwrap();
        let different_public_key = Ed25519PublicKey::from_slice(&different_keypair.public_key()).unwrap();
        
        // Verify that different key fails to verify
        let result = different_public_key.verify_bytes(message, &signature, SignMode::SIGN_MODE_UNSPECIFIED);
        assert!(result.is_err(), "Signature verification should fail with different key");
    }
}
