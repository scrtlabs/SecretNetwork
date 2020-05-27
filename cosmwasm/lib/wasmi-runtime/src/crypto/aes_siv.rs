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
///
use crate::crypto::keys::{AESKey, SymmetricKey};
use crate::crypto::traits::SIVEncryptable;
use aes_siv::aead::generic_array::GenericArray;
use aes_siv::siv::Aes128Siv;
use enclave_ffi_types::CryptoError;

impl SIVEncryptable for AESKey {
    fn encrypt_siv(&self, plaintext: &[u8], ad: &Vec<&[u8]>) -> Result<Vec<u8>, CryptoError> {
        aes_siv_encrypt(plaintext, ad, self.get())
    }

    fn decrypt_siv(&self, plaintext: &[u8], ad: &Vec<&[u8]>) -> Result<Vec<u8>, CryptoError> {
        aes_siv_decrypt(plaintext, ad, self.get())
    }
}

fn aes_siv_encrypt(
    plaintext: &[u8],
    iv: &Vec<&[u8]>,
    key: &SymmetricKey,
) -> Result<Vec<u8>, CryptoError> {
    let mut cipher = Aes128Siv::new(GenericArray::clone_from_slice(key));
    let ciphertext = match cipher.encrypt(iv, plaintext) {
        Ok(res) => res,
        Err(e) => {
            return Err(CryptoError::EncryptionError);
        }
    };
    Ok(ciphertext)
}

fn aes_siv_decrypt(
    ciphertext: &[u8],
    iv: &Vec<&[u8]>,
    key: &SymmetricKey,
) -> Result<Vec<u8>, CryptoError> {
    let mut cipher = Aes128Siv::new(GenericArray::clone_from_slice(key));
    let plaintext = match cipher.decrypt(iv, ciphertext) {
        Ok(res) => res,
        Err(e) => {
            return Err(CryptoError::DecryptionError);
        }
    };
    Ok(plaintext)
}
