use log::*;

use crate::traits::VerifyingKey;
use crate::CryptoError;
use secp256k1::Secp256k1;
use sha2::{Digest as Sha2Digest, Sha256};

pub const SECP256K1_PREFIX: [u8; 4] = [235, 90, 233, 135];

#[derive(Debug, Clone, PartialEq)]
pub struct Secp256k1PubKey(pub Vec<u8>);

impl Secp256k1PubKey {
    pub fn new(bytes: Vec<u8>) -> Self {
        Self(bytes)
    }
}

impl VerifyingKey for Secp256k1PubKey {
    fn verify_bytes(&self, bytes: &[u8], sig: &[u8]) -> Result<(), CryptoError> {
        // Signing ref: https://docs.cosmos.network/master/spec/_ics/ics-030-signed-messages.html#preliminary
        let sign_bytes_hash = Sha256::digest(bytes);
        let msg = secp256k1::Message::from_slice(sign_bytes_hash.as_slice()).map_err(|err| {
            warn!("Failed to create a secp256k1 message from tx: {:?}", err);
            CryptoError::VerificationError
        })?;

        let verifier = Secp256k1::verification_only();

        // Create `secp256k1`'s types
        let sec_signature = secp256k1::Signature::from_compact(sig).map_err(|err| {
            warn!("Malformed signature: {:?}", err);
            CryptoError::VerificationError
        })?;
        let sec_public_key =
            secp256k1::PublicKey::from_slice(self.0.as_slice()).map_err(|err| {
                warn!("Malformed public key: {:?}", err);
                CryptoError::VerificationError
            })?;

        verifier
            .verify(&msg, &sec_signature, &sec_public_key)
            .map_err(|err| {
                warn!(
                    "Failed to verify signatures for the given transaction: {:?}",
                    err
                );
                CryptoError::VerificationError
            })?;

        trace!("successfully verified this signature: {:?}", sig);
        Ok(())
    }
}

// TODO: Can we get rid of this comment below?

// use super::keys::SECRET_KEY_SIZE;
// use super::KeyPair;
// use crate::crypto::CryptoError;
//
// use secp256k1::ecdh::SharedSecret;
// use secp256k1::key::{PublicKey, SecretKey};
// use secp256k1::{All, Secp256k1};
//
// /// PubKey is a public key that is used for ECDSA signing.
// pub type PubKey = [u8; UNCOMPRESSED_PUBLIC_KEY_SIZE];
//
// pub const SECRET_KEY_SIZE: usize = secp256k1::constants::SECRET_KEY_SIZE;
// /// The size of uncomressed public keys
// pub const UNCOMPRESSED_PUBLIC_KEY_SIZE: usize = secp256k1::constants::UNCOMPRESSED_PUBLIC_KEY_SIZE;
// pub const COMPRESSED_PUBLIC_KEY_SIZE: usize = 33;
//
// #[derive(Debug, Clone)]
// pub struct KeyPair {
//     context: Secp256k1<All>,
//     pubkey: PublicKey,
//     privkey: SecretKey,
// }
//
// impl KeyPair {
//     /// This will generate a fresh pair of Public and Private keys.
//     /// it will use the available randomness from [crate::rand]
//     pub fn new() -> Result<Self, CryptoError> {
//         // This loop is important to make sure that the resulting public key isn't a point in infinity(at the curve).
//         // So if the Resulting public key is bad we need to generate a new random private key and try again until it succeeds.
//         loop {
//             let context = Secp256k1::new();
//             let mut sk_slice = [0; SECRET_KEY_SIZE];
//             rand_slice(&mut sk_slice)?;
//             if let Ok(privkey) = SecretKey::from_slice(&sk_slice) {
//                 let pubkey = PublicKey::from_secret_key(&context, &privkey);
//                 return Ok(KeyPair {
//                     context,
//                     privkey,
//                     pubkey,
//                 });
//             }
//         }
//     }
//
//     /// This function will create a Pair of keys from an array of 32 bytes.
//     /// Please don't use it to generate a new key, if you want a new key use `KeyPair::new()`
//     /// Because `KeyPair::new()` will make sure it uses a good random source and will loop private keys until it's a good key.
//     /// (and it's best to isolate the generation of keys to one place)
//     pub fn new_from_slice(privkey: &[u8; SECRET_KEY_SIZE]) -> Result<Self, CryptoError> {
//         let context = Secp256k1::new();
//
//         let privkey = SecretKey::from_slice(privkey).map_err(|e| CryptoError::KeyError {})?;
//         let pubkey = PublicKey::from_secret_key(&context, &privkey);
//
//         Ok(KeyPair {
//             context,
//             privkey,
//             pubkey,
//         })
//     }
//
//     /// This function does an ECDH(point multiplication) between one's private key and the other one's public key
//     pub fn derive_key(&self, pubarr: &[u8]) -> Result<DhKey, CryptoError> {
//         if pubarr.len() != UNCOMPRESSED_PUBLIC_KEY_SIZE
//             && pubarr.len() != COMPRESSED_PUBLIC_KEY_SIZE
//         {
//             error!("Public key invalid length - must be 65 or 33 bytes");
//             return Err(CryptoError::KeyError {});
//         }
//
//         let pubkey = PublicKey::from_slice(pubarr).map_err(|e| {
//             error!("Error creating public key {:?}", e);
//             CryptoError::KeyError {}
//         })?;
//
//         info!(
//             "Derive key public: {:?}",
//             &pubkey.serialize_uncompressed().to_vec().as_slice()
//         );
//         // SharedSecret::
//         info!("Derive key private: {:?}", &self.privkey);
//         let shared = SharedSecret::new(&pubkey, &self.privkey);
//
//         if shared.len() != SYMMETRIC_KEY_SIZE {
//             error!(
//                 "Error creating shared secret. Size mismatch {:?}",
//                 shared.len()
//             );
//             return Err(CryptoError::KeyError {});
//         }
//
//         let mut result = [0u8; SYMMETRIC_KEY_SIZE];
//         result.copy_from_slice(shared.as_ref());
//         Ok(result)
//     }
//
//     /// This will return the raw 32 bytes private key. use carefully.
//     pub fn get_privkey(&self) -> &[u8] {
//         &self.privkey[..]
//     }
//
//     // This will return the raw 64 bytes public key.
//     pub fn get_pubkey(&self) -> PubKey {
//         self.pubkey.serialize_uncompressed()
//     }
// }
//
// #[cfg(feature = "test")]
// pub mod tests {
//
//     use super::{KeyPair, Seed, SymmetricKey, SEED_SIZE};
//     use crate::crypto::{PUBLIC_KEY_SIZE, UNCOMPRESSED_PUBLIC_KEY_SIZE};
//     use crate::crypto::CryptoError;
//
//     fn test_seed_from_slice() {
//         let seed = Seed::new_from_slice(b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");
//
//         assert_eq!(seed.0, b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");
//         assert_eq!(seed.get(), b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");
//     }
//
//     fn test_seed_new() {
//         let seed = Seed::new();
//         let zero_slice = [0u8; SEED_SIZE];
//         assert_ne!(seed.0, zero_slice)
//     }
//
//     // todo: replace public key with real value
//     fn test_keypair_from_slice() {
//         let kp = KeyPair::new_from_slice(b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA").unwrap();
//
//         assert_eq!(kp.get_privkey(), b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");
//         assert_eq!(
//             kp.get_pubkey(),
//             b"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
//         );
//     }
//
//     // this obviously isn't a cryptanalysis, but hey, at least make sure the random doesn't generate all zeros
//     fn test_keypair_new() {
//         let kp = KeyPair::new().unwrap();
//         let zero_slice = [0u8; SEED_SIZE];
//         assert_ne!(kp.get_privkey(), zero_slice);
//     }
//
//     // this obviously isn't a cryptanalysis, but hey, at least make sure the random doesn't generate all zeros
//     fn test_ecdh() {
//         let kp = KeyPair::new_from_slice(b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA").unwrap();
//
//         let zero_slice = [10u8; UNCOMPRESSED_PUBLIC_KEY_SIZE];
//
//         let dhkey = kp.derive_key(&zero_slice).unwrap();
//
//         assert_eq!(dhkey, b"SOME EXPECTED KEY");
//     }
// }
