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

#[cfg(feature = "test")]
pub mod tests {
    use crate::hmac;
    use crate::keys::AESKey;
    use crate::HMAC_SIGNATURE_SIZE;

    pub fn test_hmac_sha256() {
        // Create a key for HMAC
        let key_data = [
            0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b,
            0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b,
            0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b,
            0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b, 0x0b,
        ];
        
        let key = AESKey::new_from_slice(&key_data);
        
        // Test data: "Hi There"
        let data = b"Hi There";
        
        // Compute HMAC
        let hmac_result = key.sign_sha_256(data);
        
        // Ensure the result is the expected size
        assert_eq!(hmac_result.len(), HMAC_SIGNATURE_SIZE);
        
        // Verify HMAC for different messages produces different results
        let data2 = b"Different message";
        let hmac_result2 = key.sign_sha_256(data2);
        
        // Should not be equal
        assert_ne!(hmac_result, hmac_result2);
        
        // Verify HMAC with same key and same message produces the same result
        let hmac_result_repeat = key.sign_sha_256(data);
        assert_eq!(hmac_result, hmac_result_repeat);
        
        // Verify different keys produce different HMACs for the same message
        let key_data2 = [
            0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c,
            0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c,
            0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c,
            0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c, 0x0c,
        ];
        
        let key2 = AESKey::new_from_slice(&key_data2);
        let hmac_result3 = key2.sign_sha_256(data);
        
        // Different key should produce different HMAC
        assert_ne!(hmac_result, hmac_result3);
    }
}
