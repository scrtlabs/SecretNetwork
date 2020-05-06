use enclave_ffi_types::CryptoError;
use secp256k1::ecdh::SharedSecret;
use secp256k1::key::{PublicKey, SecretKey};
use secp256k1::{All, Secp256k1};
use sgx_trts::trts::rsgx_read_rand;

// pub use crate::hash::Hash256;
/// The size of the symmetric 256 bit key we use for encryption (in bytes).
pub const SYMMETRIC_KEY_SIZE: usize = 256 / 8;
/// symmetric key we use for encryption.
pub type SymmetricKey = [u8; SYMMETRIC_KEY_SIZE];
/// StateKey is the key used for state encryption.
pub type StateKey = SymmetricKey;
/// DHKey is the key that results from the ECDH [`enigma_crypto::KeyPair::derive_key`](../replace_me)
pub type DhKey = SymmetricKey;
/// ContractAddress is the address of contracts in the Enigma Network.
// pub type ContractAddress = Hash256;
/// PubKey is a public key that is used for ECDSA signing.
pub type PubKey = [u8; 64];

pub struct KeyPair {
    context: Secp256k1<All>,
    pubkey: PublicKey,
    privkey: SecretKey,
}

impl KeyPair {
    /// This will generate a fresh pair of Public and Private keys.
    /// it will use the available randomness from [crate::rand]
    pub fn new() -> Result<KeyPair, CryptoError> {
        // This loop is important to make sure that the resulting public key isn't a point in infinity(at the curve).
        // So if the Resulting public key is bad we need to generate a new random private key and try again until it succeeds.
        loop {
            let context = Secp256k1::new();
            let mut me: [u8; 32] = [0; 32];
            rand_slice(&mut me)?;
            if let Ok(privkey) = SecretKey::from_slice(&me) {
                let pubkey = PublicKey::from_secret_key(&context, &privkey);
                return Ok(KeyPair {
                    context,
                    privkey,
                    pubkey,
                });
            }
        }
    }

    /// This function will create a Pair of keys from an array of 32 bytes.
    /// Please don't use it to generate a new key, if you want a new key use `KeyPair::new()`
    /// Because `KeyPair::new()` will make sure it uses a good random source and will loop private keys until it's a good key.
    /// (and it's best to isolate the generation of keys to one place)
    // pub fn from_slice(privkey: &[u8; 32]) -> Result<KeyPair, CryptoError> {
    //     let privkey = SecretKey::parse(&privkey).map_err(|e| CryptoError::KeyError {})?;
    //     let pubkey = PublicKey::from_secret_key(&privkey);

    //     Ok(KeyPair { privkey, pubkey })
    // }

    /// This function does an ECDH(point multiplication) between one's private key and the other one's public key.
    ///
    pub fn derive_key(&self, _pubarr: &PubKey) -> Result<DhKey, CryptoError> {
        let mut pubarr: [u8; 65] = [0; 65];
        pubarr[0] = 4;
        pubarr[1..].copy_from_slice(&_pubarr[..]);

        let pubkey = PublicKey::from_slice(&pubarr).map_err(|e| CryptoError::KeyError {})?;

        let shared = SharedSecret::new(&pubkey, &self.privkey);

        let mut result = [0u8; 32];
        result.copy_from_slice(shared.as_ref());
        Ok(result)
    }

    /// This will return the raw 32 bytes private key. use carefully.
    pub fn get_privkey(&self) -> &[u8] {
        &self.privkey[..]
    }

    // This will return the raw 64 bytes public key.
    pub fn get_pubkey(&self) -> [u8; 65] {
        self.pubkey.serialize_uncompressed()
    }

    fn pubkey_object_to_pubkey(key: &PublicKey) -> PubKey {
        let mut sliced_pubkey: [u8; 64] = [0; 64];
        sliced_pubkey.clone_from_slice(&key.serialize()[1..65]);
        sliced_pubkey
    }
}

fn rand_slice(rand: &mut [u8]) -> Result<(), CryptoError> {
    // let mut rng = thread_rng();
    // rng.try_fill(rand)
    //     .map_err(|e| CryptoError::RandomError { err: e })
    rsgx_read_rand(rand).map_err(|e| CryptoError::RandomError {})
}
