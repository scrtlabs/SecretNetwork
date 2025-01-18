use log::*;
use serde::{Deserialize, Serialize};

use enclave_crypto::{AESKey, Ed25519PublicKey, SIVEncryptable};
use enclave_ffi_types::EnclaveError;

use super::io::calc_encryption_key;

pub type IoNonce = [u8; 32];
#[derive(Serialize, Deserialize, PartialEq, Debug)]
pub struct SecretMessage {
    pub nonce: IoNonce,
    pub user_public_key: Ed25519PublicKey,
    pub msg: Vec<u8>,
}

pub struct ParsedMessage {
    pub should_verify_sig_info: bool,
    pub should_verify_input: bool,
    pub was_msg_encrypted: bool,
    pub should_encrypt_output: bool,
    pub secret_msg: SecretMessage,
    pub decrypted_msg: Vec<u8>,
    /// data_for_validation is only used when the input is an output of a reply
    pub data_for_validation: Option<Vec<u8>>,
}

pub struct DecryptedSecretMessage {
    pub secret_msg: SecretMessage,
    pub decrypted_msg: Vec<u8>,
}

impl SecretMessage {
    pub fn encrypt_in_place(&mut self) -> Result<(), EnclaveError> {
        self.msg = self
            .encryption_key()
            .encrypt_siv(self.msg.as_slice(), None)
            .map_err(|err| {
                error!("got an error while trying to encrypt the msg: {:?}", err);
                EnclaveError::EncryptionError
            })?;

        Ok(())
    }

    pub fn try_decrypt(&self) -> Option<Vec<u8>> {
        trace!("input before decryption: {:?}", base64::encode(&self.msg));
        let key = self.encryption_key();

        if let Ok(msg) = key.decrypt_siv(self.msg.as_slice(), None) {
            trace!(
                "input after decryption: {:?}",
                String::from_utf8_lossy(&msg)
            );

            return Some(msg.to_vec());
        }

        None
    }

    pub fn decrypt(&self) -> Result<Vec<u8>, EnclaveError> {
        match self.try_decrypt() {
            Some(msg) => Ok(msg),
            None => {
                error!("got an error while trying to decrypt the msg");
                Err(EnclaveError::DecryptionError)
            }
        }
    }

    pub fn encryption_key(&self) -> AESKey {
        calc_encryption_key(&self.nonce, &self.user_public_key)
    }

    pub fn from_base64(
        msg_b64: String,
        nonce: IoNonce,
        user_public_key: Ed25519PublicKey,
    ) -> Result<Self, EnclaveError> {
        let msg = base64::decode(msg_b64.to_owned().into_bytes()).map_err(|err| {
            error!(
                "got an error while trying to decode msg to next contract as base64 {:?}: {:?}",
                msg_b64, err
            );
            EnclaveError::FailedToDeserialize
        })?;

        Ok(SecretMessage {
            msg,
            nonce,
            user_public_key,
        })
    }

    pub fn from_slice(msg: &[u8]) -> Result<Self, EnclaveError> {
        // 32 bytes of nonce
        // 32 bytes of 25519 compressed public key
        // 16+ bytes of encrypted data

        if msg.len() < 82 {
            error!(
                "Encrypted message length {:?} is too short. Cannot parse",
                msg.len()
            );
            return Err(EnclaveError::DecryptionError);
        };

        let mut nonce = [0u8; 32];
        nonce.copy_from_slice(&msg[0..32]);

        let mut user_pubkey = [0u8; 32];
        user_pubkey.copy_from_slice(&msg[32..64]);

        debug!(
            "SecretMessage::from_slice nonce = {:?} pubkey = {:?}",
            nonce,
            hex::encode(user_pubkey)
        );

        Ok(SecretMessage {
            nonce,
            user_public_key: user_pubkey,
            msg: msg[64..].to_vec(),
        })
    }

    pub fn to_vec(&self) -> Vec<u8> {
        let mut packed_msg: Vec<u8> = self.nonce.to_vec();
        packed_msg.extend_from_slice(&self.user_public_key);
        packed_msg.extend_from_slice(self.msg.as_slice());
        packed_msg
    }
}

#[cfg(feature = "test")]
pub mod tests {
    use super::*;
    // use crate::crypto::{AESKey, SIVEncryptable, Seed, KEY_MANAGER};

    // todo: fix test vectors to actually work
    pub fn test_new_from_slice() {
        let nonce = [0u8; 32];
        let user_public_key = [0u8; 32];
        let msg = "{\"ok\": \"{\"balance\": \"108\"}\"}";

        let mut slice = nonce.to_vec();
        slice.extend_from_slice(&user_public_key);
        slice.extend_from_slice(msg.as_bytes());

        let secret_msg = SecretMessage {
            nonce,
            user_public_key,
            msg: msg.as_bytes().to_vec(),
        };

        let msg_from_slice = SecretMessage::from_slice(&slice).unwrap();

        assert_eq!(secret_msg, msg_from_slice);
    }
}
