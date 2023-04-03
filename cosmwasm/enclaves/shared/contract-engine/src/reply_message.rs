use crate::types::{ParsedMessage, SecretMessage};
use cw_types_v010::encoding::Binary;
use cw_types_v1::results::{
    DecryptedReply, Event, Reply, SubMsgResponse, SubMsgResult, REPLY_ENCRYPTION_MAGIC_BYTES,
};
use enclave_ffi_types::EnclaveError;
use log::{trace, warn};

const HEX_ENCODED_HASH_SIZE: usize = 64;
const SIZE_OF_U64: usize = 8;

fn redact_custom_events(reply: &mut Reply) {
    reply.result = match &reply.result {
        SubMsgResult::Ok(r) => {
            let events: Vec<Event> = Default::default();

            // let filtered_attributes = vec!["contract_address".to_string(), "code_id".to_string()];
            // for ev in r.events.iter() {
            //     if !ev.ty.starts_with("wasm") {
            //         continue;
            //     }

            //     let mut new_ev = Event {
            //         ty: ev.ty.clone(),
            //         attributes: vec![],
            //     };

            //     for attr in &ev.attributes {
            //         if !filtered_attributes.contains(&attr.key) {
            //             new_ev.attributes.push(attr.clone());
            //         }
            //     }

            //     if !new_ev.attributes.is_empty() {
            //         events.push(new_ev);
            //     }
            // }

            SubMsgResult::Ok(SubMsgResponse {
                events,
                data: r.data.clone(),
            })
        }
        SubMsgResult::Err(_) => reply.result.clone(),
    };
}

fn get_data_from_reply(
    input_msg: &SecretMessage,
    response: SubMsgResponse,
) -> Result<Option<Binary>, EnclaveError> {
    match response.data {
        Some(data) => {
            let tmp_secret_msg_data = SecretMessage {
                nonce: input_msg.nonce,
                user_public_key: input_msg.user_public_key,
                msg: data.as_slice().to_vec(),
            };

            let base64_data = tmp_secret_msg_data.decrypt()?[HEX_ENCODED_HASH_SIZE..].to_vec();

            Ok(Some(Binary::from_base64(
                String::from_utf8(base64_data.clone())
                    .map_err(|err| {
                        warn!(
                            "Failed to parse result data as string {:?}: {}",
                            base64_data, err
                        );
                        EnclaveError::FailedToDeserialize
                    })?
                    .as_str(),
            )?))
        }
        None => Ok(None),
    }
}

// The message id of the reply is unique because it contains no only the id itself but also the encryption headers
// The encryption headers are important for us, they contain to path of the reply
fn parse_message_id_of_encrypted_reply(
    input_msg: &SecretMessage,
    parsed_reply: &Reply,
) -> Result<(u64, Vec<u8>), EnclaveError> {
    let tmp_secret_msg_id = SecretMessage {
        nonce: input_msg.nonce,
        user_public_key: input_msg.user_public_key,
        msg: parsed_reply.id.as_slice().to_vec(),
    };

    let mut tmp_decrypted_msg_id = tmp_secret_msg_id.decrypt()?;

    let mut data_for_validation: Vec<u8> = tmp_decrypted_msg_id[..HEX_ENCODED_HASH_SIZE].to_vec();
    tmp_decrypted_msg_id = tmp_decrypted_msg_id[HEX_ENCODED_HASH_SIZE..].to_vec();
    while tmp_decrypted_msg_id.len() >= REPLY_ENCRYPTION_MAGIC_BYTES.len()
        && tmp_decrypted_msg_id[0..(REPLY_ENCRYPTION_MAGIC_BYTES.len())]
            == *REPLY_ENCRYPTION_MAGIC_BYTES
    {
        data_for_validation.extend_from_slice(
            &tmp_decrypted_msg_id
                [0..(REPLY_ENCRYPTION_MAGIC_BYTES.len() + SIZE_OF_U64 + HEX_ENCODED_HASH_SIZE)],
        );

        tmp_decrypted_msg_id = tmp_decrypted_msg_id
            [(REPLY_ENCRYPTION_MAGIC_BYTES.len() + SIZE_OF_U64 + HEX_ENCODED_HASH_SIZE)..]
            .to_vec();
    }

    let msg_id = String::from_utf8(tmp_decrypted_msg_id.clone()).map_err(|err| {
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

    Ok((msg_id_as_num, data_for_validation))
}

fn wrap_results_as_parsed_message(
    input_msg: &SecretMessage,
    id: u64,
    result: SubMsgResult,
    reply: &Reply,
    data_for_validation: Vec<u8>,
) -> Result<ParsedMessage, EnclaveError> {
    let decrypted_reply = DecryptedReply { id, result };

    let decrypted_reply_as_vec = serde_json::to_vec(&decrypted_reply).map_err(|err| {
        warn!(
            "got an error while trying to serialize reply into bytes {:?}: {}",
            decrypted_reply, err
        );
        EnclaveError::FailedToSerialize
    })?;

    let serialized_encrypted_reply: Vec<u8> = serde_json::to_vec(&reply).map_err(|err| {
        warn!(
            "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
            reply, err
        );
        EnclaveError::FailedToSerialize
    })?;

    let reply_secret_msg = SecretMessage {
        nonce: input_msg.nonce,
        user_public_key: input_msg.user_public_key,
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

fn parse_encrypted_ok_reply(
    input_msg: &SecretMessage,
    parsed_reply: &mut Reply,
    response: SubMsgResponse,
) -> Result<ParsedMessage, EnclaveError> {
    let decrypted_msg_data = get_data_from_reply(input_msg, response.clone())?;

    // Now we need to create synthetic SecretMessage to fit the API in "handle"
    let result = SubMsgResult::Ok(SubMsgResponse {
        events: response.events,
        data: decrypted_msg_data,
    });

    let (id, data_for_validation) = parse_message_id_of_encrypted_reply(input_msg, parsed_reply)?;
    redact_custom_events(parsed_reply);

    wrap_results_as_parsed_message(input_msg, id, result, parsed_reply, data_for_validation)
}

fn parse_encrypted_error_reply(
    input_msg: &SecretMessage,
    parsed_reply: &mut Reply,
    response: String,
) -> Result<ParsedMessage, EnclaveError> {
    let (id, data_for_validation) = parse_message_id_of_encrypted_reply(input_msg, parsed_reply)?;

    let secret_msg = SecretMessage {
        nonce: input_msg.nonce,
        user_public_key: input_msg.user_public_key,
        msg: base64::decode(response.clone()).map_err(|err| {
            warn!(
                "got an error while trying to serialize err reply from base64 {:?}: {}",
                response, err
            );
            EnclaveError::FailedToSerialize
        })?,
    };

    let decrypted_error = secret_msg.decrypt()?;

    // Now we need to create synthetic SecretMessage to fit the API in "handle"
    let result = SubMsgResult::Err(
        String::from_utf8(decrypted_error[HEX_ENCODED_HASH_SIZE..].to_vec()).map_err(|err| {
            warn!(
                "Failed to parse error as string {:?}: {}",
                decrypted_error[HEX_ENCODED_HASH_SIZE..].to_vec(),
                err
            );
            EnclaveError::FailedToDeserialize
        })?,
    );

    wrap_results_as_parsed_message(input_msg, id, result, parsed_reply, data_for_validation)
}

fn parse_encrypted_reply_message(
    input_msg: &SecretMessage,
    parsed_reply: &mut Reply,
) -> Result<ParsedMessage, EnclaveError> {
    match parsed_reply.result.clone() {
        SubMsgResult::Ok(response) => parse_encrypted_ok_reply(input_msg, parsed_reply, response),
        SubMsgResult::Err(response) => {
            parse_encrypted_error_reply(input_msg, parsed_reply, response)
        }
    }
}

fn parse_plaintext_reply_message(
    input_msg: &SecretMessage,
    parsed_reply: &mut Reply,
) -> Result<ParsedMessage, EnclaveError> {
    let msg_id = String::from_utf8(parsed_reply.id.as_slice().to_vec()).map_err(|err| {
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

    redact_custom_events(parsed_reply);
    let serialized_reply: Vec<u8> = serde_json::to_vec(parsed_reply).map_err(|err| {
        warn!(
            "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
            parsed_reply, err
        );
        EnclaveError::FailedToSerialize
    })?;

    let reply_secret_msg = SecretMessage {
        nonce: input_msg.nonce,
        user_public_key: input_msg.user_public_key,
        msg: serialized_reply,
    };

    let serialized_reply: Vec<u8> = serde_json::to_vec(&decrypted_reply).map_err(|err| {
        warn!(
            "got an error while trying to serialize decrypted reply into bytes {:?}: {}",
            decrypted_reply, err
        );
        EnclaveError::FailedToSerialize
    })?;

    Ok(ParsedMessage {
        should_validate_sig_info: false,
        was_msg_encrypted: false,
        should_encrypt_output: parsed_reply.was_orig_msg_encrypted,
        secret_msg: reply_secret_msg,
        decrypted_msg: serialized_reply,
        data_for_validation: None,
    })
}

pub fn parse_reply_message(encrypted_message: &[u8]) -> Result<ParsedMessage, EnclaveError> {
    let orig_secret_msg = SecretMessage::from_slice(encrypted_message)?;
    let mut parsed_reply: Reply = serde_json::from_slice(&orig_secret_msg.msg).map_err(|err| {
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
            base64::encode(&encrypted_message)
        );

        return parse_plaintext_reply_message(&orig_secret_msg, &mut parsed_reply);
    }

    trace!(
        "reply input before decryption: {:?}",
        base64::encode(&encrypted_message)
    );

    parse_encrypted_reply_message(&orig_secret_msg, &mut parsed_reply)
}
