use crate::types::{DecryptedSecretMessage, SecretMessage};
use log::trace;

pub fn get_secret_msg(message: &[u8]) -> SecretMessage {
    match SecretMessage::from_slice(message) {
        Ok(orig_secret_msg) => orig_secret_msg,
        Err(_) => {
            trace!(
                "Message is not SecretMessage (probably plaintext): {:?}",
                base64::encode(&message)
            );

            SecretMessage {
                nonce: [0; 32],
                user_public_key: [0; 32],
                msg: message.into(),
            }
        }
    }
}

pub fn try_get_decrypted_secret_msg(message: &[u8]) -> Option<DecryptedSecretMessage> {
    let secret_msg = get_secret_msg(message);
    match secret_msg.try_decrypt() {
        Some(decrypted_msg) => Some(DecryptedSecretMessage {
            secret_msg,
            decrypted_msg,
        }),
        None => None,
    }
}
