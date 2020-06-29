use ring::digest;

pub const HASH_SIZE: usize = 32;

pub fn sha_256(data: &[u8]) -> [u8; HASH_SIZE] {
    let hash = digest::digest(&digest::SHA256, data);

    let mut result = [0u8; HASH_SIZE];
    result.copy_from_slice(hash.as_ref());

    result
}

// #[cfg(feature = "test")]
// pub mod tests {
//     use super::{
//         Keychain, CONSENSUS_SEED_SEALING_PATH, KEY_MANAGER, REGISTRATION_KEY_SEALING_PATH,
//     };
//     use crate::crypto::{Kdf, KeyPair, Seed};
//     use crate::crypto::CryptoError;
//
//     // todo: fix test vectors to actually work
//     fn test_derive_key() {
//         let seed = Seed::new_from_slice(&[10u8; 32]);
//
//         let kdf1 = seed.derive_key_from_this(&1.to_be_bytes());
//         let kdf2 = seed.derive_key_from_this(&2.to_be_bytes());
//
//         assert_eq!(kdf1, b"SOME VALUE");
//         assert_eq!(kdf2, b"SOME VALUE");
//     }
// }
