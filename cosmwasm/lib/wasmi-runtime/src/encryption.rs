use crate::keys::SymmetricKey;
use enclave_ffi_types::CryptoError;
use ring::aead::{self, Aad, LessSafeKey, Nonce, UnboundKey};
use sgx_trts::trts::rsgx_read_rand;

static AES_MODE: &aead::Algorithm = &aead::AES_256_GCM;

/// The IV key byte size
const IV_SIZE: usize = 96 / 8;
/// Type alias for the IV byte array
type IV = [u8; IV_SIZE];

/// This function will encrypt a plaintext message and append the tag
/// The cyphertext can be decrypted with the `decrypt` function below.
fn encrypt(plaintext: &[u8], key: &SymmetricKey) -> Result<Vec<u8>, CryptoError> {
    let key =
        LessSafeKey::new(UnboundKey::new(&AES_MODE, key).map_err(|_| CryptoError::KeyError {})?);

    let mut in_out_buffer = plaintext.to_owned();
    let nonce = Nonce::assume_unique_for_key([0_u8; 12]); // TODO fix

    key.seal_in_place_append_tag(nonce, Aad::empty(), &mut in_out_buffer)
        .map_err(|_| CryptoError::EncryptionError)?;

    Ok(in_out_buffer.to_vec())
}

/// This function will decrypt a cipher text only if it was encrypted with the `encrypt` function above.
/// (data must be encrypted with `seal_in_place_append_tag`)
pub fn decrypt(ciphertext: &[u8], key: &SymmetricKey) -> Result<Vec<u8>, CryptoError> {
    let key =
        LessSafeKey::new(UnboundKey::new(&AES_MODE, key).map_err(|_| CryptoError::KeyError {})?);

    let mut ciphertext = ciphertext.to_owned();
    let nonce = Nonce::assume_unique_for_key([0_u8; 12]); // TODO fix

    let plaintext = key
        .open_in_place(nonce, Aad::empty(), &mut ciphertext)
        .map_err(|_| CryptoError::DecryptionError)?;

    Ok(plaintext.to_vec())
}
