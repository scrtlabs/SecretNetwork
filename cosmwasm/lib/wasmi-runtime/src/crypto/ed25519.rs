use super::keys::DhKey;

// super::traits::Ecdh;

use x25519_dalek;

use enclave_ffi_types::CryptoError;

use super::rng::rand_slice;

pub const SECRET_KEY_SIZE: usize = 32;
pub const PUBLIC_KEY_SIZE: usize = 32;

#[derive(Debug, Clone)]
pub struct KeyPair {
    secret_key: [u8; 32],
    public_key: [u8; 32],
}

impl KeyPair {
    pub fn new() -> Result<Self, CryptoError> {
        let mut sk_slice = [0; SECRET_KEY_SIZE];
        rand_slice(&mut sk_slice)?;

        let sk = x25519_dalek::StaticSecret::from(sk_slice);
        let pk = x25519_dalek::PublicKey::from(&sk);

        return Ok(Self {
            secret_key: sk.to_bytes(),
            public_key: pk.as_bytes().clone(),
        });
    }
    pub fn new_from_slice(privkey: [u8; SECRET_KEY_SIZE]) -> Result<Self, CryptoError> {
        let my_secret = x25519_dalek::StaticSecret::from(privkey);
        let pk = x25519_dalek::PublicKey::from(&my_secret);
        return Ok(Self {
            secret_key: privkey.clone(),
            public_key: pk.as_bytes().clone(),
        });
    }
    pub fn diffie_hellman(&self, your_public: &[u8; SECRET_KEY_SIZE]) -> DhKey {
        let my_secret = x25519_dalek::StaticSecret::from(self.secret_key);
        let pk = x25519_dalek::PublicKey::from(your_public.clone());
        let ss = my_secret.diffie_hellman(&pk);

        ss.as_bytes().clone()
    }
    pub fn get_privkey(&self) -> [u8; SECRET_KEY_SIZE] {
        self.secret_key.clone()
    }

    // This will return the raw 64 bytes public key.
    pub fn get_pubkey(&self) -> [u8; PUBLIC_KEY_SIZE] {
        self.public_key.clone()
    }
}
