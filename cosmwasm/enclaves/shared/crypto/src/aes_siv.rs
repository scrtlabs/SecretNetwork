/// AES-SIV encryption in rust - https://tools.ietf.org/html/rfc5297
///
/// This is a unique AES mode for deterministic encryption, where it is difficult to generate random
/// values. The risks of reusing a nonce are only such that encrypting the same data with the same nonce
/// and ad (additional-data - similar to AAD) is that it will give the same result. For this reason we
/// can use a best-effort to modify the AD, but misusing it somehow will not cause all hell to break loose.
///
/// Uses aes_siv crate, which has not been constant-time audited and other good stuff, but we assume
/// that the risk of using it is much lesser than the risk of using AES-GCM, or other nonce-collision
/// sensitive ciphers.
///
/// In SIV mode, the key is split in 2 - the upper half of the bits is taken into a PRF, while the
/// bottom (LSB) is taken as the private key. This is why the private key will be twice the length you
/// would expect it to be. 256/512 bit for Aes128/256 respectively.
///
/// The result of encrypted data will be the size of the data + 16 bytes, same as in GCM mode
use crate::keys::{AESKey, SymmetricKey};
use crate::traits::SIVEncryptable;
use crate::CryptoError;
use aes_siv::aead::generic_array::GenericArray;
use aes_siv::siv::Aes128Siv;
use log::*;

impl SIVEncryptable for AESKey {
    fn encrypt_siv(&self, plaintext: &[u8], ad: Option<&[&[u8]]>) -> Result<Vec<u8>, CryptoError> {
        aes_siv_encrypt(plaintext, ad, self.get())
    }

    fn decrypt_siv(&self, plaintext: &[u8], ad: Option<&[&[u8]]>) -> Result<Vec<u8>, CryptoError> {
        aes_siv_decrypt(plaintext, ad, self.get())
    }
}

fn aes_siv_encrypt(
    plaintext: &[u8],
    ad: Option<&[&[u8]]>,
    key: &SymmetricKey,
) -> Result<Vec<u8>, CryptoError> {
    let ad = ad.unwrap_or(&[&[]]);

    let mut cipher = Aes128Siv::new(GenericArray::clone_from_slice(key));
    cipher.encrypt(ad, plaintext).map_err(|e| {
        warn!("aes_siv_encrypt error: {:?}", e);
        CryptoError::EncryptionError
    })
}

fn aes_siv_decrypt(
    ciphertext: &[u8],
    ad: Option<&[&[u8]]>,
    key: &SymmetricKey,
) -> Result<Vec<u8>, CryptoError> {
    let ad = ad.unwrap_or(&[&[]]);

    let mut cipher = Aes128Siv::new(GenericArray::clone_from_slice(key));
    cipher.decrypt(ad, ciphertext).map_err(|e| {
        warn!("aes_siv_decrypt error: {:?}", e);
        CryptoError::DecryptionError
    })
}

#[cfg(feature = "test")]
pub mod tests {

    use super::{aes_siv_decrypt, aes_siv_encrypt};

    // todo: fix test vectors to actually work
    pub fn _test_aes_encrypt() {
        let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
        let aad: Vec<&[u8]> = vec![
            b"00112233445566778899aabbccddeeffdeaddadadeaddadaffeeddccbbaa99887766554433221100",
            b"102030405060708090a0",
            b"09f911029d74e35bd84156c5635688c0",
        ];
        let plaintext = b"7468697320697320736f6d6520706c61696e7465787420746f20656e6372797074207573696e67205349562d414553";
        let ciphertext = b"7bdb6e3b432667eb06f4d14bff2fbd0fcb900f2fddbe404326601965c889bf17dba77ceb094fa663b7a3f748ba8af829ea64ad544a272e9c485b62a3fd5c0d";

        let result = aes_siv_encrypt(plaintext, Some(&aad), &key).unwrap();

        assert_eq!(result.as_slice(), &ciphertext[..])
    }

    // todo: fix test vectors to actually work
    pub fn _test_aes_decrypt() {
        let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
        let aad: Vec<&[u8]> = vec![
            b"00112233445566778899aabbccddeeffdeaddadadeaddadaffeeddccbbaa99887766554433221100",
            b"102030405060708090a0",
            b"09f911029d74e35bd84156c5635688c0",
        ];
        let plaintext = b"7468697320697320736f6d6520706c61696e7465787420746f20656e6372797074207573696e67205349562d414553";
        let ciphertext = b"7bdb6e3b432667eb06f4d14bff2fbd0fcb900f2fddbe404326601965c889bf17dba77ceb094fa663b7a3f748ba8af829ea64ad544a272e9c485b62a3fd5c0d";

        let result = aes_siv_decrypt(ciphertext, Some(&aad), &key).unwrap();

        assert_eq!(result.as_slice(), &plaintext[..])
    }

    // todo: fix test vectors to actually work
    pub fn _test_aes_encrypt_empty_aad() {
        let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
        let aad: Vec<&[u8]> = vec![];
        let plaintext = b"7468697320697320736f6d6520706c61696e7465787420746f20656e6372797074207573696e67205349562d414553";
        let ciphertext = b"7bdb6e3b432667eb06f4d14bff2fbd0fcb900f2fddbe404326601965c889bf17dba77ceb094fa663b7a3f748ba8af829ea64ad544a272e9c485b62a3fd5c0d";

        let result = aes_siv_encrypt(plaintext, Some(&aad), &key).unwrap();

        assert_eq!(result.as_slice(), &ciphertext[..])
    }
}
