use log::{trace, warn};

use cw_types_v1::ibc::IbcPacketReceiveMsg;
use enclave_cosmos_types::types::HandleType;
use enclave_ffi_types::EnclaveError;

use crate::reply_message::{parse_reply_message, ParsedMessage};
use crate::types::SecretMessage;
pub struct DecryptedSecretMessage {
    pub secret_msg: SecretMessage,
    pub decrypted_msg: Vec<u8>,
}

pub fn get_secret_msg(message: &[u8]) -> SecretMessage {
    match SecretMessage::from_slice(message) {
        Ok(orig_secret_msg) => orig_secret_msg,
        Err(_) => {
            trace!(
                "Msg is not SecretMessage (probably plaintext): {:?}",
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
    match secret_msg.decrypt() {
        Ok(decrypted_msg) => Some(DecryptedSecretMessage {
            secret_msg,
            decrypted_msg,
        }),
        Err(_) => None,
    }
}

pub fn parse_encrypted_execute_message(
    encrypted_message: &[u8],
) -> Result<ParsedMessage, EnclaveError> {
    if let Some(decrypted_secret_msg) = try_get_decrypted_secret_msg(encrypted_message) {
        trace!(
            "execute input before decryption: {:?}",
            base64::encode(&encrypted_message)
        );

        return Ok(ParsedMessage {
            should_validate_sig_info: true,
            was_msg_encrypted: true,
            should_encrypt_output: true,
            secret_msg: decrypted_secret_msg.secret_msg,
            decrypted_msg: decrypted_secret_msg.decrypted_msg,
            data_for_validation: None,
        });
    }

    trace!(
        "execute input was plaintext: {:?}",
        base64::encode(&encrypted_message)
    );

    let secret_msg = SecretMessage {
        nonce: [0; 32],
        user_public_key: [0; 32],
        msg: encrypted_message.into(),
    };

    let decrypted_msg = secret_msg.msg.clone();

    Ok(ParsedMessage {
        should_validate_sig_info: true,
        was_msg_encrypted: false,
        should_encrypt_output: false,
        secret_msg,
        decrypted_msg,
        data_for_validation: None,
    })
}

// Parse the message that was passed to handle (Based on the assumption that it might be a reply or IBC as well)
pub fn parse_message(
    encrypted_message: &[u8],
    handle_type: &HandleType,
) -> Result<ParsedMessage, EnclaveError> {
    return match handle_type {
        HandleType::HANDLE_TYPE_EXECUTE => parse_encrypted_execute_message(encrypted_message),
        HandleType::HANDLE_TYPE_REPLY => parse_reply_message(encrypted_message),
        HandleType::HANDLE_TYPE_IBC_CHANNEL_OPEN
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CONNECT
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CLOSE
        | HandleType::HANDLE_TYPE_IBC_PACKET_ACK
        | HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT => {
            trace!(
                "parsing {} msg (Should always be plaintext): {:?}",
                HandleType::get_export_name(handle_type),
                base64::encode(&encrypted_message)
            );

            let scrt_msg = SecretMessage {
                nonce: [0; 32],
                user_public_key: [0; 32],
                msg: encrypted_message.into(),
            };

            let decrypted_msg = scrt_msg.msg.clone();

            Ok(ParsedMessage {
                should_validate_sig_info: false,
                was_msg_encrypted: false,
                should_encrypt_output: false,
                secret_msg: scrt_msg,
                decrypted_msg,
                data_for_validation: None,
            })
        }
        HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE => {
            // TODO: Maybe mark whether the message was encrypted or not.
            let mut parsed_encrypted_ibc_packet: IbcPacketReceiveMsg =
                serde_json::from_slice(encrypted_message).map_err(|err| {
                    warn!(
            "Got an error while trying to deserialize input bytes msg into IbcPacketReceiveMsg message {:?}: {}",
            String::from_utf8_lossy(encrypted_message),
            err
        );
                    EnclaveError::FailedToDeserialize
                })?;

            let tmp_secret_data =
                get_secret_msg(parsed_encrypted_ibc_packet.packet.data.as_slice());
            let mut was_msg_encrypted = false;
            let mut orig_secret_msg = tmp_secret_data;

            match orig_secret_msg.decrypt() {
                Ok(decrypted_msg) => {
                    // IBC packet was encrypted

                    trace!(
                        "ibc_packet_receive data before decryption: {:?}",
                        base64::encode(&encrypted_message)
                    );

                    parsed_encrypted_ibc_packet.packet.data = decrypted_msg.as_slice().into();
                    was_msg_encrypted = true;
                }
                Err(_) => {
                    // assume data is not encrypted

                    trace!(
                        "ibc_packet_receive data was plaintext: {:?}",
                        base64::encode(&encrypted_message)
                    );

                    orig_secret_msg = SecretMessage {
                        nonce: [0; 32],
                        user_public_key: [0; 32],
                        msg: encrypted_message.into(),
                    };
                }
            }
            Ok(ParsedMessage {
                should_validate_sig_info: false,
                was_msg_encrypted,
                should_encrypt_output: was_msg_encrypted,
                secret_msg: orig_secret_msg,
                decrypted_msg: serde_json::to_vec(&parsed_encrypted_ibc_packet).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize IbcPacketReceive msg into bytes {:?}: {}",
                        parsed_encrypted_ibc_packet, err
                    );
                    EnclaveError::FailedToSerialize
                })?,
                data_for_validation: None,
            })
        }
    };
}

pub fn is_ibc_msg(handle_type: HandleType) -> bool {
    match handle_type {
        HandleType::HANDLE_TYPE_EXECUTE | HandleType::HANDLE_TYPE_REPLY => false,
        HandleType::HANDLE_TYPE_IBC_CHANNEL_OPEN
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CONNECT
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CLOSE
        | HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE
        | HandleType::HANDLE_TYPE_IBC_PACKET_ACK
        | HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT => true,
    }
}
