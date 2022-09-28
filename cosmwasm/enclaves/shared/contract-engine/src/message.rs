use log::{trace, warn};

use cw_types_v010::encoding::Binary;
use cw_types_v1::ibc::IbcPacketReceiveMsg;
use cw_types_v1::results::{
    DecryptedReply, Event, Reply, SubMsgResponse, SubMsgResult, REPLY_ENCRYPTION_MAGIC_BYTES,
};
use enclave_cosmos_types::types::HandleType;
use enclave_ffi_types::EnclaveError;

use crate::types::SecretMessage;

const HEX_ENCODED_HASH_SIZE: usize = 64;
const SIZE_OF_U64: usize = 8;

pub struct ParsedMessage {
    pub should_validate_sig_info: bool,
    pub was_msg_encrypted: bool,
    pub should_encrypt_output: bool,
    pub secret_msg: SecretMessage,
    pub decrypted_msg: Vec<u8>,
    pub data_for_validation: Option<Vec<u8>>,
}

pub struct DecryptedSecretMessage {
    pub secret_msg: SecretMessage,
    pub decrypted_msg: Vec<u8>,
}

pub fn redact_custom_events(reply: &mut Reply) {
    reply.result = match &reply.result {
        SubMsgResult::Ok(r) => {
            let mut events: Vec<Event> = Default::default();

            let filtered_attributes = vec!["contract_address".to_string(), "code_id".to_string()];
            for ev in r.events.iter() {
                if !ev.ty.starts_with("wasm") {
                    continue;
                }

                let mut new_ev = Event {
                    ty: ev.ty.clone(),
                    attributes: vec![],
                };

                for attr in &ev.attributes {
                    if !filtered_attributes.contains(&attr.key) {
                        new_ev.attributes.push(attr.clone());
                    }
                }

                if !new_ev.attributes.is_empty() {
                    events.push(new_ev);
                }
            }

            SubMsgResult::Ok(SubMsgResponse {
                events,
                data: r.data.clone(),
            })
        }
        SubMsgResult::Err(_) => reply.result.clone(),
    };
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

// Parse the message that was passed to handle (Based on the assumption that it might be a reply or IBC as well)
pub fn parse_message(
    message: &[u8],
    handle_type: &HandleType,
) -> Result<ParsedMessage, EnclaveError> {
    return match handle_type {
        HandleType::HANDLE_TYPE_EXECUTE => match try_get_decrypted_secret_msg(message) {
            Some(decrypted_secret_msg) => {
                trace!(
                    "execute input before decryption: {:?}",
                    base64::encode(&message)
                );

                Ok(ParsedMessage {
                    should_validate_sig_info: true,
                    was_msg_encrypted: true,
                    should_encrypt_output: true,
                    secret_msg: decrypted_secret_msg.secret_msg,
                    decrypted_msg: decrypted_secret_msg.decrypted_msg,
                    data_for_validation: None,
                })
            }
            None => {
                trace!(
                    "execute input was plaintext: {:?}",
                    base64::encode(&message)
                );

                let secret_msg = get_secret_msg(message);
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
        },
        HandleType::HANDLE_TYPE_REPLY => {
            let orig_secret_msg = SecretMessage::from_slice(message)?;
            let mut parsed_reply: Reply =
                serde_json::from_slice(&orig_secret_msg.msg).map_err(|err| {
                    warn!(
                    "reply got an error while trying to deserialize reply bytes into json {:?}: {}",
                    String::from_utf8_lossy(&orig_secret_msg.msg.clone()),
                    err
                );
                    EnclaveError::FailedToDeserialize
                })?;

            if !parsed_reply.is_encrypted {
                trace!(
                    "reply input is not encrypted: {:?}",
                    base64::encode(&message)
                );

                let msg_id =
                    String::from_utf8(parsed_reply.id.as_slice().to_vec()).map_err(|err| {
                        warn!(
                            "Failed to parse message id as string {:?}: {}",
                            parsed_reply.id.as_slice().to_vec(),
                            err
                        );
                        EnclaveError::FailedToDeserialize
                    })?;

                let msg_id_as_num = match msg_id.parse::<u64>() {
                    Ok(m) => m,
                    Err(err) => {
                        warn!("Failed to parse message id as number {}: {}", msg_id, err);
                        return Err(EnclaveError::FailedToDeserialize);
                    }
                };

                let decrypted_reply = DecryptedReply {
                    id: msg_id_as_num,
                    result: parsed_reply.result.clone(),
                };

                redact_custom_events(&mut parsed_reply);
                let serialized_encrypted_reply : Vec<u8> = serde_json::to_vec(&parsed_reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
                        parsed_reply, err
                    );
                    EnclaveError::FailedToSerialize
                })?;

                let reply_secret_msg = SecretMessage {
                    nonce: orig_secret_msg.nonce,
                    user_public_key: orig_secret_msg.user_public_key,
                    msg: serialized_encrypted_reply,
                };

                let serialized_reply: Vec<u8> = serde_json::to_vec(&decrypted_reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize decrypted reply into bytes {:?}: {}",
                        decrypted_reply, err
                    );
                    EnclaveError::FailedToSerialize
                })?;

                return Ok(ParsedMessage {
                    should_validate_sig_info: false,
                    was_msg_encrypted: false,
                    should_encrypt_output: parsed_reply.was_orig_msg_encrypted,
                    secret_msg: reply_secret_msg,
                    decrypted_msg: serialized_reply,
                    data_for_validation: None,
                });
            }

            trace!(
                "reply input before decryption: {:?}",
                base64::encode(&message)
            );

            match parsed_reply.result.clone() {
                SubMsgResult::Ok(response) => {
                    let decrypted_msg_data = match response.data {
                        Some(data) => {
                            let tmp_secret_msg_data = SecretMessage {
                                nonce: orig_secret_msg.nonce,
                                user_public_key: orig_secret_msg.user_public_key,
                                msg: data.as_slice().to_vec(),
                            };

                            let base64_data =
                                tmp_secret_msg_data.decrypt()?[HEX_ENCODED_HASH_SIZE..].to_vec();

                            Some(Binary::from_base64(
                                String::from_utf8(base64_data.clone())
                                    .map_err(|err| {
                                        warn!(
                                            "Failed to parse result data as string {:?}: {}",
                                            base64_data, err
                                        );
                                        EnclaveError::FailedToDeserialize
                                    })?
                                    .as_str(),
                            )?)
                        }
                        None => None,
                    };

                    let tmp_secret_msg_id = SecretMessage {
                        nonce: orig_secret_msg.nonce,
                        user_public_key: orig_secret_msg.user_public_key,
                        msg: parsed_reply.id.as_slice().to_vec(),
                    };

                    let mut tmp_decrypted_msg_id = tmp_secret_msg_id.decrypt()?;

                    // Now we need to create synthetic SecretMessage to fit the API in "handle"
                    let result = SubMsgResult::Ok(SubMsgResponse {
                        events: response.events,
                        data: decrypted_msg_data,
                    });

                    let mut data_for_validation: Vec<u8> =
                        tmp_decrypted_msg_id[..HEX_ENCODED_HASH_SIZE].to_vec();
                    tmp_decrypted_msg_id = tmp_decrypted_msg_id[HEX_ENCODED_HASH_SIZE..].to_vec();
                    while tmp_decrypted_msg_id.len() >= REPLY_ENCRYPTION_MAGIC_BYTES.len()
                        && tmp_decrypted_msg_id[0..(REPLY_ENCRYPTION_MAGIC_BYTES.len())]
                            == *REPLY_ENCRYPTION_MAGIC_BYTES
                    {
                        data_for_validation.extend_from_slice(
                            &tmp_decrypted_msg_id[0..(REPLY_ENCRYPTION_MAGIC_BYTES.len()
                                + SIZE_OF_U64
                                + HEX_ENCODED_HASH_SIZE)],
                        );

                        tmp_decrypted_msg_id = tmp_decrypted_msg_id[(REPLY_ENCRYPTION_MAGIC_BYTES
                            .len()
                            + SIZE_OF_U64
                            + HEX_ENCODED_HASH_SIZE)..]
                            .to_vec();
                    }

                    let msg_id =
                        String::from_utf8(tmp_decrypted_msg_id.clone()).map_err(|err| {
                            warn!(
                                "Failed to parse message id as string {:?}: {}",
                                tmp_decrypted_msg_id, err
                            );
                            EnclaveError::FailedToDeserialize
                        })?;

                    let msg_id_as_num = match msg_id.parse::<u64>() {
                        Ok(m) => m,
                        Err(err) => {
                            warn!("Failed to parse message id as number {}: {}", msg_id, err);
                            return Err(EnclaveError::FailedToDeserialize);
                        }
                    };

                    let decrypted_reply = DecryptedReply {
                        id: msg_id_as_num,
                        result,
                    };

                    let decrypted_reply_as_vec =
                        serde_json::to_vec(&decrypted_reply).map_err(|err| {
                            warn!(
                                "got an error while trying to serialize reply into bytes {:?}: {}",
                                decrypted_reply, err
                            );
                            EnclaveError::FailedToSerialize
                        })?;

                    redact_custom_events(&mut parsed_reply);
                    let serialized_encrypted_reply : Vec<u8> = serde_json::to_vec(&parsed_reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
                        parsed_reply, err
                    );
                    EnclaveError::FailedToSerialize
                })?;

                    let reply_secret_msg = SecretMessage {
                        nonce: orig_secret_msg.nonce,
                        user_public_key: orig_secret_msg.user_public_key,
                        msg: serialized_encrypted_reply,
                    };

                    Ok(ParsedMessage {
                        should_validate_sig_info: true,
                        was_msg_encrypted: true,
                        should_encrypt_output: true,
                        secret_msg: reply_secret_msg,
                        decrypted_msg: decrypted_reply_as_vec,
                        data_for_validation: Some(data_for_validation),
                    })
                }
                SubMsgResult::Err(response) => {
                    let secret_msg = SecretMessage {
                        nonce: orig_secret_msg.nonce,
                        user_public_key: orig_secret_msg.user_public_key,
                        msg: base64::decode(response.clone()).map_err(|err| {
                            warn!(
                                "got an error while trying to serialize err reply from base64 {:?}: {}",
                                    response, err
                            );
                            EnclaveError::FailedToSerialize
                        })?
                    };

                    let decrypted_error = secret_msg.decrypt()?;

                    let tmp_secret_msg_id = SecretMessage {
                        nonce: orig_secret_msg.nonce,
                        user_public_key: orig_secret_msg.user_public_key,
                        msg: parsed_reply.id.as_slice().to_vec(),
                    };

                    let mut tmp_decrypted_msg_id = tmp_secret_msg_id.decrypt()?;

                    // Now we need to create synthetic SecretMessage to fit the API in "handle"
                    let result = SubMsgResult::Err(
                        String::from_utf8(decrypted_error[HEX_ENCODED_HASH_SIZE..].to_vec())
                            .map_err(|err| {
                                warn!(
                                    "Failed to parse error as string {:?}: {}",
                                    decrypted_error[HEX_ENCODED_HASH_SIZE..].to_vec(),
                                    err
                                );
                                EnclaveError::FailedToDeserialize
                            })?,
                    );

                    let mut data_for_validation: Vec<u8> =
                        tmp_decrypted_msg_id[..HEX_ENCODED_HASH_SIZE].to_vec();
                    tmp_decrypted_msg_id = tmp_decrypted_msg_id[HEX_ENCODED_HASH_SIZE..].to_vec();
                    while tmp_decrypted_msg_id.len() >= REPLY_ENCRYPTION_MAGIC_BYTES.len()
                        && tmp_decrypted_msg_id[0..(REPLY_ENCRYPTION_MAGIC_BYTES.len())]
                            == *REPLY_ENCRYPTION_MAGIC_BYTES
                    {
                        data_for_validation.extend_from_slice(
                            &tmp_decrypted_msg_id[0..(REPLY_ENCRYPTION_MAGIC_BYTES.len()
                                + SIZE_OF_U64
                                + HEX_ENCODED_HASH_SIZE)],
                        );

                        tmp_decrypted_msg_id = tmp_decrypted_msg_id[(REPLY_ENCRYPTION_MAGIC_BYTES
                            .len()
                            + SIZE_OF_U64
                            + HEX_ENCODED_HASH_SIZE)..]
                            .to_vec();
                    }

                    let msg_id =
                        String::from_utf8(tmp_decrypted_msg_id.clone()).map_err(|err| {
                            warn!(
                                "Failed to parse message id as string {:?}: {}",
                                tmp_decrypted_msg_id, err
                            );
                            EnclaveError::FailedToDeserialize
                        })?;

                    let msg_id_as_num = match msg_id.parse::<u64>() {
                        Ok(m) => m,
                        Err(err) => {
                            warn!("Failed to parse message id as number {}: {}", msg_id, err);
                            return Err(EnclaveError::FailedToDeserialize);
                        }
                    };

                    let decrypted_reply = DecryptedReply {
                        id: msg_id_as_num,
                        result,
                    };

                    let decrypted_reply_as_vec =
                        serde_json::to_vec(&decrypted_reply).map_err(|err| {
                            warn!(
                                "got an error while trying to serialize reply into bytes {:?}: {}",
                                decrypted_reply, err
                            );
                            EnclaveError::FailedToSerialize
                        })?;

                    let serialized_encrypted_reply : Vec<u8> = serde_json::to_vec(&parsed_reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
                        parsed_reply, err
                    );
                    EnclaveError::FailedToSerialize
                })?;

                    let reply_secret_msg = SecretMessage {
                        nonce: orig_secret_msg.nonce,
                        user_public_key: orig_secret_msg.user_public_key,
                        msg: serialized_encrypted_reply,
                    };

                    Ok(ParsedMessage {
                        should_validate_sig_info: true,
                        was_msg_encrypted: true,
                        should_encrypt_output: true,
                        secret_msg: reply_secret_msg,
                        decrypted_msg: decrypted_reply_as_vec,
                        data_for_validation: Some(data_for_validation),
                    })
                }
            }
        }
        HandleType::HANDLE_TYPE_IBC_CHANNEL_OPEN
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CONNECT
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CLOSE
        | HandleType::HANDLE_TYPE_IBC_PACKET_ACK
        | HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT => {
            trace!(
                "parsing {} msg (Should always be plaintext): {:?}",
                HandleType::get_export_name(&handle_type),
                base64::encode(&message)
            );

            let scrt_msg = SecretMessage {
                nonce: [0; 32],
                user_public_key: [0; 32],
                msg: message.into(),
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
                serde_json::from_slice(&message.to_vec()).map_err(|err| {
                    warn!(
            "Got an error while trying to deserialize input bytes msg into IbcPacketReceiveMsg message {:?}: {}",
            String::from_utf8_lossy(&message),
            err
        );
                    EnclaveError::FailedToDeserialize
                })?;

            let tmp_secret_data =
                get_secret_msg(parsed_encrypted_ibc_packet.packet.data.as_slice());
            let mut was_msg_encrypted = false;
            let orig_secret_msg = tmp_secret_data;

            match orig_secret_msg.decrypt() {
                Ok(decrypted_msg) => {
                    // IBC packet was encrypted

                    trace!(
                        "ibc_packet_receive data before decryption: {:?}",
                        base64::encode(&message)
                    );

                    parsed_encrypted_ibc_packet.packet.data = decrypted_msg.as_slice().into();
                    was_msg_encrypted = true;
                }
                Err(_) => {
                    // assume data is not encrypted

                    trace!(
                        "ibc_packet_receive data was plaintext: {:?}",
                        base64::encode(&message)
                    );
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
