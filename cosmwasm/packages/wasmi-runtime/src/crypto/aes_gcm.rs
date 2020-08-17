// use crate::crypto::keys::{AESKey, SymmetricKey};
// use crate::crypto::traits::Encryptable;
// use crate::crypto::CryptoError;
// use ring::aead::{self, Aad, LessSafeKey, Nonce, UnboundKey};
//
// static AES_MODE: &aead::Algorithm = &aead::AES_256_GCM;
//
// // /// The IV key byte size
// // const IV_SIZE: usize = 96 / 8;
// // /// Type alias for the IV byte array
// // type IV = [u8; IV_SIZE];
//
// impl Encryptable for AESKey {
//     fn encrypt(&self, plaintext: &[u8]) -> Result<Vec<u8>, CryptoError> {
//         encrypt(plaintext, self.get())
//     }
//
//     fn decrypt(&self, ciphertext: &[u8]) -> Result<Vec<u8>, CryptoError> {
//         decrypt(ciphertext, self.get())
//     }
// }
//
// /// This function will encrypt a plaintext message and append the tag
// /// The cyphertext can be decrypted with the `decrypt` function below.
// fn encrypt(plaintext: &[u8], key: &SymmetricKey) -> Result<Vec<u8>, CryptoError> {
//     let key =
//         LessSafeKey::new(UnboundKey::new(&AES_MODE, key).map_err(|_| CryptoError::KeyError {})?);
//
//     let mut in_out_buffer = plaintext.to_owned();
//     let nonce = Nonce::assume_unique_for_key([0_u8; 12]); // TODO fix
//
//     key.seal_in_place_append_tag(nonce, Aad::empty(), &mut in_out_buffer)
//         .map_err(|_| CryptoError::EncryptionError)?;
//
//     Ok(in_out_buffer.to_vec())
// }
//
// /// This function will decrypt a cipher text only if it was encrypted with the `encrypt` function above.
// /// (data must be encrypted with `seal_in_place_append_tag`)
// fn decrypt(ciphertext: &[u8], key: &SymmetricKey) -> Result<Vec<u8>, CryptoError> {
//     let key =
//         LessSafeKey::new(UnboundKey::new(&AES_MODE, key).map_err(|_| CryptoError::KeyError {})?);
//
//     let mut ciphertext = ciphertext.to_owned();
//     let nonce = Nonce::assume_unique_for_key([0_u8; 12]); // TODO fix
//
//     let plaintext = key
//         .open_in_place(nonce, Aad::empty(), &mut ciphertext)
//         .map_err(|_| CryptoError::DecryptionError)?;
//
//     Ok(plaintext.to_vec())
// }
//
// // #[cfg(feature = "test")]
// // pub mod tests {
// //
// //     use super::{decrypt, encrypt};
// //
// //     // todo: fix test vectors to actually work
// //     fn test_aes_encrypt() {
// //         let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
// //         let aad: Vec<&[u8]> = vec![
// //             b"00112233445566778899aabbccddeeffdeaddadadeaddadaffeeddccbbaa99887766554433221100",
// //             b"102030405060708090a0",
// //             b"09f911029d74e35bd84156c5635688c0",
// //         ];
// //         let plaintext = b"7468697320697320736f6d6520706c61696e7465787420746f20656e6372797074207573696e67205349562d414553";
// //         let ciphertext = b"7bdb6e3b432667eb06f4d14bff2fbd0fcb900f2fddbe404326601965c889bf17dba77ceb094fa663b7a3f748ba8af829ea64ad544a272e9c485b62a3fd5c0d";
// //
// //         let result = aes_siv_encrypt(plaintext, &aad, &key).unwrap();
// //
// //         assert_eq!(result.as_slice(), &ciphertext)
// //
// //     }
// //
// //     // todo: fix test vectors to actually work
// //     fn test_aes_decrypt() {
// //         let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
// //         let aad: Vec<&[u8]> = vec![
// //             b"00112233445566778899aabbccddeeffdeaddadadeaddadaffeeddccbbaa99887766554433221100",
// //             b"102030405060708090a0",
// //             b"09f911029d74e35bd84156c5635688c0",
// //         ];
// //         let plaintext = b"7468697320697320736f6d6520706c61696e7465787420746f20656e6372797074207573696e67205349562d414553";
// //         let ciphertext = b"7bdb6e3b432667eb06f4d14bff2fbd0fcb900f2fddbe404326601965c889bf17dba77ceb094fa663b7a3f748ba8af829ea64ad544a272e9c485b62a3fd5c0d";
// //
// //         let result = aes_siv_decrypt(ciphertext, &aad, &key).unwrap();
// //
// //         assert_eq!(result.as_slice(), &plaintext)
// //
// //     }
// //
// //     // todo: fix test vectors to actually work
// //     fn test_aes_encrypt_empty_aad() {
// //         let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
// //         let aad: Vec<&[u8]> = vec![];
// //         let plaintext = b"7468697320697320736f6d6520706c61696e7465787420746f20656e6372797074207573696e67205349562d414553";
// //         let ciphertext = b"7bdb6e3b432667eb06f4d14bff2fbd0fcb900f2fddbe404326601965c889bf17dba77ceb094fa663b7a3f748ba8af829ea64ad544a272e9c485b62a3fd5c0d";
// //
// //         let result = aes_siv_encrypt(plaintext, &aad, &key).unwrap();
// //
// //         assert_eq!(result.as_slice(), &ciphertext)
// //
// //     }
// // }
