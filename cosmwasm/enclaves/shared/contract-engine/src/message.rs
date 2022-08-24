use cosmos_proto::tx::signing::SignMode;
use cw_types_v010::encoding::Binary;

use log::{trace, warn};

use crate::types::SecretMessage;
use cw_types_v1::results::{DecryptedReply, Event, Reply, SubMsgResponse, SubMsgResult};
use enclave_cosmos_types::types::{HandleType, SigInfo};
use enclave_ffi_types::EnclaveError;

const HEX_ENCODED_HASH_SIZE: usize = 64;

pub struct ParsedMessage {
    pub should_validate_sig_info: bool,
    pub was_msg_encrypted: bool,
    pub secret_msg: SecretMessage,
    pub decrypted_msg: Vec<u8>,
    pub contract_hash_for_validation: Option<Vec<u8>>,
}

fn redact_custom_events(reply: &mut Reply) {
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

// Parse the message that was passed to handle (Based on the assumption that it might be a reply or IBC as well)
pub fn parse_message(
    message: &[u8],
    sig_info: &SigInfo,
    handle_type: &HandleType,
) -> Result<ParsedMessage, EnclaveError> {
    let orig_secret_msg = SecretMessage::from_slice(message)?;

    return match handle_type {
        HandleType::HANDLE_TYPE_EXECUTE => {
            let decrypted_msg = orig_secret_msg.decrypt()?;
            Ok(ParsedMessage {
                should_validate_sig_info: true,
                was_msg_encrypted: true,
                secret_msg: orig_secret_msg,
                decrypted_msg,
                contract_hash_for_validation: None,
            })
        }

        HandleType::HANDLE_TYPE_REPLY => {
            if sig_info.sign_mode == SignMode::SIGN_MODE_UNSPECIFIED {
                trace!("reply input is not encrypted");
                let decrypted_msg = orig_secret_msg.msg.clone();
                let mut reply: Reply = serde_json::from_slice(&decrypted_msg)
                    .map_err(|err| {
                        warn!(
                "reply got an error while trying to deserialize decrypted reply bytes into json {:?}: {}",
                String::from_utf8_lossy(&decrypted_msg),
                err
            );
                        EnclaveError::FailedToDeserialize
                    })?;

                let msg_id = String::from_utf8(reply.id.as_slice().to_vec()).map_err(|err| {
                    warn!(
                        "Failed to parse message id as string {:?}: {}",
                        reply.id.as_slice().to_vec(),
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
                    result: reply.result.clone(),
                };

                redact_custom_events(&mut reply);
                let serialized_encrypted_reply : Vec<u8> = serde_json::to_vec(&reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
                        reply, err
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
                    secret_msg: reply_secret_msg,
                    decrypted_msg: serialized_reply,
                    contract_hash_for_validation: None,
                });
            }

            // Here we are sure the reply is OK because only OK is encrypted
            trace!(
                "reply input before decryption: {:?}",
                base64::encode(&message)
            );
            let mut parsed_encrypted_reply: Reply = serde_json::from_slice(
                &orig_secret_msg.msg.as_slice().to_vec(),
            )
            .map_err(|err| {
                warn!(
            "reply got an error while trying to deserialize msg input bytes into json {:?}: {}",
            String::from_utf8_lossy(&orig_secret_msg.msg),
            err
            );
                EnclaveError::FailedToDeserialize
            })?;

            match parsed_encrypted_reply.result.clone() {
                SubMsgResult::Ok(response) => {
                    let decrypted_msg_data = match response.data {
                        Some(data) => {
                            let tmp_secret_msg_data = SecretMessage {
                                nonce: orig_secret_msg.nonce,
                                user_public_key: orig_secret_msg.user_public_key,
                                msg: data.as_slice().to_vec(),
                            };

                            Some(Binary(
                                tmp_secret_msg_data.decrypt()?[HEX_ENCODED_HASH_SIZE..].to_vec(),
                            ))
                        }
                        None => None,
                    };

                    let tmp_secret_msg_id = SecretMessage {
                        nonce: orig_secret_msg.nonce,
                        user_public_key: orig_secret_msg.user_public_key,
                        msg: parsed_encrypted_reply.id.as_slice().to_vec(),
                    };

                    let tmp_decrypted_msg_id = tmp_secret_msg_id.decrypt()?;

                    // Now we need to create synthetic SecretMessage to fit the API in "handle"
                    let result = SubMsgResult::Ok(SubMsgResponse {
                        events: response.events,
                        data: decrypted_msg_data,
                    });

                    let msg_id =
                        String::from_utf8(tmp_decrypted_msg_id[HEX_ENCODED_HASH_SIZE..].to_vec())
                            .map_err(|err| {
                            warn!(
                                "Failed to parse message id as string {:?}: {}",
                                tmp_decrypted_msg_id[HEX_ENCODED_HASH_SIZE..].to_vec(),
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

                    redact_custom_events(&mut parsed_encrypted_reply);
                    let serialized_encrypted_reply : Vec<u8> = serde_json::to_vec(&parsed_encrypted_reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
                        parsed_encrypted_reply, err
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
                        secret_msg: reply_secret_msg,
                        decrypted_msg: decrypted_reply_as_vec,
                        contract_hash_for_validation: Some(
                            tmp_decrypted_msg_id[..HEX_ENCODED_HASH_SIZE].to_vec(),
                        ),
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
                        msg: parsed_encrypted_reply.id.as_slice().to_vec(),
                    };

                    let tmp_decrypted_msg_id = tmp_secret_msg_id.decrypt()?;

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

                    let msg_id =
                        String::from_utf8(tmp_decrypted_msg_id[HEX_ENCODED_HASH_SIZE..].to_vec())
                            .map_err(|err| {
                            warn!(
                                "Failed to parse message id as string {:?}: {}",
                                tmp_decrypted_msg_id[HEX_ENCODED_HASH_SIZE..].to_vec(),
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

                    let serialized_encrypted_reply : Vec<u8> = serde_json::to_vec(&parsed_encrypted_reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
                        parsed_encrypted_reply, err
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
                        secret_msg: reply_secret_msg,
                        decrypted_msg: decrypted_reply_as_vec,
                        contract_hash_for_validation: Some(
                            tmp_decrypted_msg_id[..HEX_ENCODED_HASH_SIZE].to_vec(),
                        ),
                    })
                }
            }
        }
    };
}
