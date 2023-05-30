use crate::message_utils::try_get_decrypted_secret_msg;
use crate::types::{ParsedMessage, SecretMessage};
use enclave_ffi_types::EnclaveError;
use log::trace;

pub fn parse_execute_message(message: &[u8]) -> Result<ParsedMessage, EnclaveError> {
    if let Some(decrypted_secret_msg) = try_get_decrypted_secret_msg(message) {
        trace!(
            "execute input before decryption: {:?}",
            base64::encode(&message)
        );

        return Ok(ParsedMessage {
            should_verify_sig_info: true,
            should_verify_input: true,
            was_msg_encrypted: true,
            should_encrypt_output: true,
            secret_msg: decrypted_secret_msg.secret_msg,
            decrypted_msg: decrypted_secret_msg.decrypted_msg,
            data_for_validation: None,
        });
    }

    trace!(
        "execute input was plaintext: {:?}",
        base64::encode(&message)
    );

    let secret_msg = SecretMessage {
        nonce: [0; 32],
        user_public_key: [0; 32],
        msg: message.into(),
    };

    let decrypted_msg = secret_msg.msg.clone();

    Ok(ParsedMessage {
        should_verify_sig_info: true,
        should_verify_input: true,
        was_msg_encrypted: false,
        should_encrypt_output: false,
        secret_msg,
        decrypted_msg,
        data_for_validation: None,
    })
}
