use crate::keys::SymmetricKey;
use enclave_ffi_types::CryptoError;
use ring::aead::{self, Aad, Nonce};
use sgx_trts::trts::rsgx_read_rand;

static AES_MODE: &aead::Algorithm = &aead::AES_256_GCM;

/// The IV key byte size
const IV_SIZE: usize = 96 / 8;
/// Type alias for the IV byte array
type IV = [u8; IV_SIZE];

/// This function will encrypt a plaintext message and append the tag
/// The cyphertext can be decrypted with the `decrypt` function below.
fn encrypt(message: &[u8], key: &SymmetricKey) -> Result<Vec<u8>, CryptoError> {
    let key = aead::SealingKey::new(&AES_MODE, key).map_err(|_| CryptoError::KeyError {})?;

    let mut in_out = message.to_owned();
    let tag_size = AES_MODE.tag_len(); // authenticated tag (includes the IV)
    in_out.extend(vec![0u8; tag_size]);
    let seal_size = aead::seal_in_place_append_tag(&key, Aad::empty(), &mut in_out)
        .map_err(|_| CryptoError::EncryptionError)?;

    in_out.truncate(seal_size);
    Ok(in_out)
}

/// This function will decrypt a cipher text only if it was encrypted with the `encrypt` function above.
/// (data must be encrypted with `seal_in_place_append_tag`)
pub fn decrypt(ciphertext: &[u8], key: &SymmetricKey) -> Result<Vec<u8>, CryptoError> {
    let key = aead::OpeningKey::new(&AES_MODE, key).map_err(|_| CryptoError::KeyError {})?;

    let mut ciphertext = ciphertext.to_owned();
    let plaintext = key
        .open_in_place(Aad::empty(), &mut ciphertext)
        .map_err(|_| CryptoError::DecryptionError)?;

    Ok(plaintext.to_vec())
}
