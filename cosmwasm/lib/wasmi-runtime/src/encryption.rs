use crate::keys::SymmetricKey;
use enclave_ffi_types::CryptoError;
use ring::aead::{self, Aad, Nonce};
use sgx_trts::trts::rsgx_read_rand;

static AES_MODE: &aead::Algorithm = &aead::AES_256_GCM;

/// The IV key byte size
const IV_SIZE: usize = 96 / 8;
/// Type alias for the IV byte array
type IV = [u8; IV_SIZE];

//#[deprecated(note = "This function shouldn't be called directly unless you're implementing the Encryption trait, please use `encrypt()` instead")]
/// This function does the same as [`self::encrypt`] but accepts an IV.
/// it *shouldn't* be called directly. only from tests or [`crate::Encryption::encrypt_with_nonce`] implementations.
fn encrypt(message: &[u8], key: &SymmetricKey) -> Result<Vec<u8>, CryptoError> {
    let aes_encrypt =
        aead::SealingKey::new(&AES_MODE, key).map_err(|_| CryptoError::KeyError {})?;

    let mut in_out = message.to_owned();
    let tag_size = AES_MODE.tag_len(); // padding?
    in_out.extend(vec![0u8; tag_size]);
    let seal_size = aead::seal_in_place_append_tag(&aes_encrypt, Aad::empty(), &mut in_out)
        .map_err(|_| CryptoError::EncryptionError)?;

    in_out.truncate(seal_size);
    in_out.extend_from_slice(&iv);
    Ok(in_out)
}

/// This function will decrypt a cipher text only if it was encrypted with the `encrypt` function above.
/// Because it will try to get the IV from the last 12 bytes in the cipher text,
/// then ring will take the last 16 bytes as a MAC to check the integrity of the cipher text.
pub fn decrypt(cipheriv: &[u8], key: &SymmetricKey) -> Result<Vec<u8>, CryptoError> {
    if cipheriv.len() < IV_SIZE {
        return Err(CryptoError::ImproperEncryption);
    }
    let aes_decrypt =
        aead::OpeningKey::new(&AES_MODE, key).map_err(|_| CryptoError::KeyError {})?;

    let (ciphertext, iv) = cipheriv.split_at(cipheriv.len() - 12);
    let nonce = aead::Nonce::try_assume_unique_for_key(&iv).unwrap(); // This Cannot fail because split_at promises that iv.len()==12
    let mut ciphertext = ciphertext.to_owned();
    let decrypted_data = aead::open_in_place(&aes_decrypt, nonce, Aad::empty(), 0, &mut ciphertext);
    let decrypted_data = decrypted_data.map_err(|_| CryptoError::DecryptionError)?;

    Ok(decrypted_data.to_vec())
}
