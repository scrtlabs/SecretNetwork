use crate::message_utils::try_get_decrypted_secret_msg;
use crate::types::{ParsedMessage, SecretMessage};
use cw_types_v1::ibc::IbcPacketReceiveMsg;
use enclave_ffi_types::EnclaveError;
use log::{trace, warn};

pub fn parse_plaintext_ibc_protocol_message(
    plaintext_message: &[u8],
) -> Result<ParsedMessage, EnclaveError> {
    let scrt_msg = SecretMessage {
        nonce: [0; 32],
        user_public_key: [0; 32],
        msg: plaintext_message.into(),
    };

    Ok(ParsedMessage {
        should_validate_sig_info: false,
        should_validate_input: false,
        was_msg_encrypted: false,
        should_encrypt_output: false,
        secret_msg: scrt_msg,
        decrypted_msg: plaintext_message.into(),
        data_for_validation: None,
    })
}

pub fn parse_ibc_receive_message(message: &[u8]) -> Result<ParsedMessage, EnclaveError> {
    // TODO: Maybe mark whether the message was encrypted or not.
    let mut parsed_encrypted_ibc_packet: IbcPacketReceiveMsg =
     serde_json::from_slice(message).map_err(|err| {
         warn!(
            "Got an error while trying to deserialize input bytes msg into IbcPacketReceiveMsg message {:?}: {}",
            String::from_utf8_lossy(message),
            err
        );
         EnclaveError::FailedToDeserialize
     })?;

    let (was_msg_encrypted, secret_msg) =
        match try_get_decrypted_secret_msg(parsed_encrypted_ibc_packet.packet.data.as_slice()) {
            Some(decrypted_msg) => {
                // IBC packet was encrypted

                trace!(
                    "ibc_packet_receive data before decryption: {:?}",
                    base64::encode(&message)
                );

                parsed_encrypted_ibc_packet.packet.data =
                    decrypted_msg.decrypted_msg.as_slice().into();
                (true, decrypted_msg.secret_msg)
            }
            None => {
                // Assume data is not encrypted

                trace!(
                    "ibc_packet_receive data was plaintext: {:?}",
                    base64::encode(&message)
                );

                (
                    false,
                    SecretMessage {
                        nonce: [0; 32],
                        user_public_key: [0; 32],
                        msg: message.into(),
                    },
                )
            }
        };

    Ok(ParsedMessage {
        should_validate_sig_info: false,
        should_validate_input: true,
        was_msg_encrypted,
        should_encrypt_output: was_msg_encrypted,
        secret_msg,
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

pub fn parse_ibc_hooks_incoming_transfer_message(
    message: &[u8],
) -> Result<ParsedMessage, EnclaveError> {
    Ok(ParsedMessage {
        should_validate_sig_info: false,
        should_validate_input: true,
        was_msg_encrypted: false,
        should_encrypt_output: false,
        secret_msg: SecretMessage {
            nonce: [0; 32],
            user_public_key: [0; 32],
            msg: message.into(),
        },
        decrypted_msg: message.into(),
        data_for_validation: None,
    })
}
