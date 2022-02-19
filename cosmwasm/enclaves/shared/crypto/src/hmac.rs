use super::{AESKey, Hmac, HMAC_SIGNATURE_SIZE};
use ring::hmac;

impl Hmac for AESKey {
    fn sign_sha_256(&self, to_sign: &[u8]) -> [u8; HMAC_SIGNATURE_SIZE] {
        // let mut input_bytes: Vec<u8> = self.get().to_vec();
        // input_bytes.extend_from_slice(extra_key_info);

        let key = hmac::Key::new(hmac::HMAC_SHA256, self.get());
        let tag = hmac::sign(&key, to_sign);

        let mut result: [u8; HMAC_SIGNATURE_SIZE] = [0u8; HMAC_SIGNATURE_SIZE];

        result.copy_from_slice(tag.as_ref());

        result
    }
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
