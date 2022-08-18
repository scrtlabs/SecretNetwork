use log::*;
use serde::de::DeserializeOwned;
use serde::Serialize;
use std::fmt::Debug;

use enclave_ffi_types::{Ctx, EnclaveError};

use crate::contract_validation::{ReplyParams, ValidatedMessage};
use crate::external::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::wasm::CosmWasmApiVersion;
use cosmos_proto::tx::signing::SignMode;
use cosmwasm_v010_types::types::CanonicalAddr;
use enclave_cosmos_types::types::{ContractCode, HandleType, SigInfo};
use enclave_cosmwasm_v010_types as cosmwasm_v010_types;
use enclave_cosmwasm_v010_types::encoding::Binary;
use enclave_cosmwasm_v1_types::addresses::Addr;
use enclave_cosmwasm_v1_types::ibc::{
    IbcPacketAckMsg, IbcPacketReceiveMsg, IbcPacketTimeoutMsg, IbcPacketTrait,
};
use enclave_cosmwasm_v1_types::results::{
    DecryptedReply, Event, Reply, SubMsgResponse, SubMsgResult,
};
use enclave_cosmwasm_v1_types::timestamp::Timestamp;

use enclave_crypto::{Ed25519PublicKey, HASH_SIZE};
use enclave_utils::coalesce;

use super::contract_validation::{
    extract_contract_key, generate_encryption_key, validate_contract_key, validate_msg,
    verify_params, ContractKey,
};
use super::gas::WasmCosts;
use super::io::{encrypt_output, finalize_raw_output, RawWasmOutput};
use super::module_cache::create_module_instance;
use super::types::{IoNonce, SecretMessage};
use super::wasm::{ContractInstance, ContractOperation, Engine};

const HEX_ENCODED_HASH_SIZE: usize = HASH_SIZE * 2;

/*
Each contract is compiled with these functions already implemented in wasm:
fn cosmwasm_api_0_6() -> i32;  // Seems unused, but we should support it anyways
fn allocate(size: usize) -> *mut c_void;
fn deallocate(pointer: *mut c_void);
fn init(env_ptr: *mut c_void, msg_ptr: *mut c_void) -> *mut c_void
fn handle(env_ptr: *mut c_void, msg_ptr: *mut c_void) -> *mut c_void
fn query(msg_ptr: *mut c_void) -> *mut c_void

Re `init`, `handle` and `query`: We need to pass `env` & `msg`
down to the wasm implementations, but because they are buffers
we need to allocate memory regions inside the VM's instance and copy
`env` & `msg` into those memory regions inside the VM's instance.
*/

pub fn init(
    context: Ctx,       // need to pass this to read_db & write_db
    gas_limit: u64,     // gas limit for this execution
    used_gas: &mut u64, // out-parameter for gas used in execution
    contract: &[u8],    // contract wasm bytes
    env: &[u8],         // blockchain state
    msg: &[u8],         // probably function call and args
    sig_info: &[u8],    // info about signature verification
) -> Result<InitSuccess, EnclaveError> {
    let contract_code = ContractCode::new(contract);

    let mut env_v010: cosmwasm_v010_types::types::Env =
        serde_json::from_slice(env).map_err(|err| {
            warn!(
                "init got an error while trying to deserialize env input bytes into json {:?}: {}",
                String::from_utf8_lossy(&env),
                err
            );
            EnclaveError::FailedToDeserialize
        })?;
    env_v010.contract_code_hash = hex::encode(contract_code.hash());

    let canonical_contract_address = CanonicalAddr::from_human(&env_v010.contract.address).map_err(|err| {
        warn!(
            "init got an error while trying to deserialize env_v010.contract.address from bech32 string to bytes {:?}: {}",
            env_v010.contract.address, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    trace!("init env_v010: {:?}", env_v010);

    let canonical_sender_address = CanonicalAddr::from_human(&env_v010.message.sender).map_err(|err| {
        warn!(
            "init got an error while trying to deserialize env_v010.message.sender from bech32 string to bytes {:?}: {}",
            env_v010.message.sender, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let contract_key = generate_encryption_key(
        &env_v010,
        contract_code.hash(),
        &(canonical_contract_address.0).0,
    )?;
    trace!("init contract key: {:?}", hex::encode(contract_key));

    let parsed_sig_info: SigInfo = serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "init got an error while trying to deserialize env input bytes into json {:?}: {}",
            String::from_utf8_lossy(&sig_info),
            err
        );
        EnclaveError::FailedToDeserialize
    })?;

    trace!("init input before decryption: {:?}", base64::encode(&msg));
    let secret_msg = SecretMessage::from_slice(msg)?;

    verify_params(&parsed_sig_info, &env_v010, &secret_msg)?;

    let decrypted_msg = secret_msg.decrypt()?;

    let ValidatedMessage {
        validated_msg,
        reply_params,
    } = validate_msg(&decrypted_msg, contract_code.hash(), None)?;

    trace!(
        "init input after decryption: {:?}",
        String::from_utf8_lossy(&validated_msg)
    );

    let mut engine = start_engine(
        context,
        gas_limit,
        contract_code,
        &contract_key,
        ContractOperation::Init,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    let (contract_env_bytes, contract_msg_info_bytes) =
        env_to_env_msg_info_bytes(&engine, &mut env_v010)?;

    let env_ptr = engine.write_to_memory(&contract_env_bytes)?;
    let msg_info_ptr = engine.write_to_memory(&contract_msg_info_bytes)?;
    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.init(env_ptr, msg_info_ptr, msg_ptr)?;
        let output = engine.extract_vector(vec_ptr)?;
        // TODO: copy cosmwasm's structures to enclave
        // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/init_handle.rs#L129
        // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/query.rs#L13
        let output = encrypt_output(
            output,
            &secret_msg,
            &canonical_contract_address,
            &env_v010.contract_code_hash,
            reply_params,
            &canonical_sender_address,
            false,
        )?;

        Ok(output)
    })
    .map_err(|err| {
        *used_gas = engine.gas_used();
        err
    })?;

    *used_gas = engine.gas_used();
    // todo: can move the key to somewhere in the output message if we want

    Ok(InitSuccess {
        output,
        contract_key,
    })
}

pub struct ParsedMessage {
    pub should_validate_sig_info: bool,
    pub was_msg_encrypted: bool,
    pub secret_msg: SecretMessage,
    pub decrypted_msg: Vec<u8>,
    pub contract_hash_for_validation: Option<Vec<u8>>,
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

                if new_ev.attributes.len() > 0 {
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

            return SecretMessage {
                nonce: [0; 32],
                user_public_key: [0; 32],
                msg: message.into(),
            };
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

pub fn parse_ibc_packet<T>(_t: T, message: &[u8], function_name: &str) -> ParsedMessage
where
    T: IbcPacketTrait<Data = Binary> + Serialize + DeserializeOwned + Debug,
{
    let mut parsed_encrypted_ibc_packet: T = serde_json::from_slice(&message.as_slice().to_vec())
        .map_err(|err| {
        warn!(
            "{} msg got an error while trying to deserialize msg input bytes into json {:?}: {}",
            function_name,
            String::from_utf8_lossy(&orig_secret_msg.msg),
            err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let tmp_secret_data = get_secret_msg(parsed_encrypted_ibc_packet.get_packet().as_slice());
    let was_msg_encrypted = false;
    let orig_secret_msg = tmp_secret_data;

    match tmp_secret_data.decrypt() {
        Ok(decrypted_msg) => {
            // IBC packet was encrypted

            trace!(
                "{} data before decryption: {:?}",
                function_name,
                base64::encode(&message)
            );

            parsed_encrypted_ibc_packet.set_packet(decrypted_msg.as_slice().into());
            was_msg_encrypted = true;
        }
        Err(_) => {
            // assume data is not encrypted

            trace!(
                "{} data was plaintext: {:?}",
                function_name,
                base64::encode(&message)
            );
        }
    }

    let tmp_secret_ack = get_secret_msg(parsed_encrypted_ibc_packet.get_ack().as_slice());

    match tmp_secret_ack.decrypt() {
        Ok(ack_data) => {
            parsed_encrypted_ibc_packet.set_ack(ack_data.as_slice().into());
            was_msg_encrypted = true;

            orig_secret_msg = tmp_secret_ack;
        }
        Err(_) => {}
    }

    ParsedMessage {
        should_validate_sig_info: false,
        was_msg_encrypted,
        secret_msg: orig_secret_msg,
        decrypted_msg: serde_json::to_vec(&parsed_encrypted_ibc_packet).map_err(|err| {
            warn!(
                "got an error while trying to serialize {} msg into bytes {:?}: {}",
                function_name, parsed_encrypted_ibc_packet, err
            );
            EnclaveError::FailedToSerialize
        })?,
        contract_hash_for_validation: None,
    }
}

// Parse the message that was passed to handle (Based on the assumption that it might be a reply or IBC as well)
pub fn parse_message(
    message: &[u8],
    sig_info: &SigInfo,
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
                    secret_msg: decrypted_secret_msg.secret_msg,
                    decrypted_msg: decrypted_secret_msg.decrypted_msg,
                    contract_hash_for_validation: None,
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
                    secret_msg,
                    decrypted_msg,
                    contract_hash_for_validation: None,
                })
            }
        },
        HandleType::HANDLE_TYPE_REPLY => {
            let orig_secret_msg = SecretMessage::from_slice(message)?;

            if sig_info.sign_mode == SignMode::SIGN_MODE_UNSPECIFIED {
                trace!(
                    "reply input is not encrypted: {:?}",
                    base64::encode(&message)
                );
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
        HandleType::HANDLE_TYPE_IBC_CHANNEL_OPEN
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CONNECT
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CLOSE => {
            trace!(
                "parsing {} msg (Should always be plaintext): {:?}",
                HandleType::to_export_name(&handle_type),
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
                secret_msg: scrt_msg,
                decrypted_msg,
                contract_hash_for_validation: None,
            })
        }
        HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE => {
            // LIORRR TODO: Maybe mark whether the message was encrypted or not.
            let orig_secret_msg = get_secret_msg(message);
            Ok(parse_ibc_packet(
                IbcPacketReceiveMsg::default(),
                message,
                "ibc_packet_receive",
            ))
        }
        HandleType::HANDLE_TYPE_IBC_PACKET_ACK => {
            // LIORRR TODO: Maybe mark whether the message was encrypted or not.
            let orig_secret_msg = get_secret_msg(message);
            Ok(parse_ibc_packet(
                IbcPacketAckMsg::default(),
                message,
                "ibc_packet_receive",
            ))
        }
        HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT => {
            // LIORRR TODO: Maybe mark whether the message was encrypted or not.
            let orig_secret_msg = get_secret_msg(message);
            Ok(parse_ibc_packet(
                IbcPacketTimeoutMsg::default(),
                message,
                "ibc_packet_timeout",
            ))
        }
    };
}

#[cfg_attr(feature = "cargo-clippy", allow(clippy::too_many_arguments))]
pub fn handle(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
    sig_info: &[u8],
    handle_type: u8,
) -> Result<HandleSuccess, EnclaveError> {
    let contract_code = ContractCode::new(contract);

    let mut env_v010: cosmwasm_v010_types::types::Env =
        serde_json::from_slice(env).map_err(|err| {
            warn!(
            "handle got an error while trying to deserialize env input bytes into json {:?}: {}",
            env, err
        );
            EnclaveError::FailedToDeserialize
        })?;
    env_v010.contract_code_hash = hex::encode(contract_code.hash());

    trace!("handle env_v010: {:?}", env_v010);

    let canonical_contract_address = CanonicalAddr::from_human(&env_v010.contract.address).map_err(|err| {
        warn!(
            "got an error while trying to deserialize env_v010.contract.address from bech32 string to bytes {:?}: {}",
            env_v010.contract.address, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let canonical_sender_address = CanonicalAddr::from_human(&env_v010.message.sender).map_err(|err| {
        warn!(
            "init got an error while trying to deserialize env_v010.message.sender from bech32 string to bytes {:?}: {}",
            env_v010.message.sender, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let contract_key = extract_contract_key(&env_v010)?;

    if !validate_contract_key(&contract_key, &canonical_contract_address, &contract_code) {
        warn!("got an error while trying to deserialize output bytes");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    let parsed_sig_info: SigInfo = serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "handle got an error while trying to deserialize sig info input bytes into json {:?}: {}",
            String::from_utf8_lossy(&sig_info),
            err
        );
        EnclaveError::FailedToDeserialize
    })?;

    // The flow of handle is now used for multiple messages (such ash Handle, Reply)
    // When the message is handle, we expect it always to be encrypted while in Reply for example it might be plaintext
    let parsed_handle_type = HandleType::try_from(handle_type)?;

    let ParsedMessage {
        should_validate_sig_info,
        was_msg_encrypted,
        secret_msg,
        decrypted_msg,
        contract_hash_for_validation,
    } = parse_message(msg, &parsed_sig_info, &parsed_handle_type)?;

    // There is no signature to verify when the input isn't signed.
    // Receiving unsigned messages is only possible in Handle. (Init tx are always signed)
    // All of these functions go through handle but the data isn't signed:
    //  Reply (that is not WASM reply)
    if should_validate_sig_info {
        // Verify env parameters against the signed tx
        verify_params(&parsed_sig_info, &env_v010, &secret_msg)?;
    }

    let mut validated_msg = decrypted_msg.clone();
    let mut reply_params: Option<ReplyParams> = None;
    if was_msg_encrypted {
        let x = validate_msg(
            &decrypted_msg,
            contract_code.hash(),
            contract_hash_for_validation,
        )?;
        validated_msg = x.validated_msg;
        reply_params = x.reply_params;
    }

    trace!(
        "handle input afer decryption: {:?}",
        String::from_utf8_lossy(&validated_msg)
    );

    trace!("Successfully authenticated the contract!");

    trace!("Handle: Contract Key: {:?}", hex::encode(contract_key));

    // Although the operation here is not always handle it is irrelevant in this case
    // because it only helps to decide whether to check floating points or not
    // In this case we want to do the same as in Handle both for Reply and for others so we can always pass "Handle".
    let mut engine = start_engine(
        context,
        gas_limit,
        contract_code,
        &contract_key,
        ContractOperation::Handle,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    let (contract_env_bytes, contract_msg_info_bytes) =
        env_to_env_msg_info_bytes(&engine, &mut env_v010)?;

    let env_ptr = engine.write_to_memory(&contract_env_bytes)?;
    let msg_info_ptr = engine.write_to_memory(&contract_msg_info_bytes)?;
    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.handle(env_ptr, msg_info_ptr, msg_ptr, parsed_handle_type)?;

        let mut output = engine.extract_vector(vec_ptr)?;

        debug!(
            "(2) nonce just before encrypt_output: nonce = {:?} pubkey = {:?}",
            secret_msg.nonce, secret_msg.user_public_key
        );

        if was_msg_encrypted {
            output = encrypt_output(
                output,
                &secret_msg,
                &canonical_contract_address,
                &env_v010.contract_code_hash,
                reply_params,
                &canonical_sender_address,
                false,
            )?;
        } else {
            let raw_output: RawWasmOutput = serde_json::from_slice(&output).map_err(|err| {
                warn!("got an error while trying to deserialize output bytes into json");
                trace!("output: {:?} error: {:?}", output, err);
                EnclaveError::FailedToDeserialize
            })?;

            let finalized_output = finalize_raw_output(raw_output, false);

            output = serde_json::to_vec(&finalized_output).map_err(|err| {
                debug!(
                    "got an error while trying to serialize output json into bytes {:?}: {}",
                    finalized_output, err
                );
                EnclaveError::FailedToSerialize
            })?;
        }

        Ok(output)
    })
    .map_err(|err| {
        *used_gas = engine.gas_used();
        err
    })?;

    *used_gas = engine.gas_used();
    Ok(HandleSuccess { output })
}

pub fn query(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
) -> Result<QuerySuccess, EnclaveError> {
    let contract_code = ContractCode::new(contract);

    let mut env_v010: cosmwasm_v010_types::types::Env =
        serde_json::from_slice(env).map_err(|err| {
            warn!(
                "query got an error while trying to deserialize env input bytes into json {:?}: {}",
                env, err
            );
            EnclaveError::FailedToDeserialize
        })?;
    env_v010.contract_code_hash = hex::encode(contract_code.hash());

    trace!("query env_v010: {:?}", env_v010);

    let canonical_contract_address = CanonicalAddr::from_human(&env_v010.contract.address).map_err(|err| {
        warn!(
            "got an error while trying to deserialize env_v010.contract.address from bech32 string to bytes {:?}: {}",
            env_v010.contract.address, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let contract_key = extract_contract_key(&env_v010)?;

    if !validate_contract_key(&contract_key, &canonical_contract_address, &contract_code) {
        warn!("query got an error while trying to validate contract key");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    trace!("successfully authenticated the contract!");
    trace!("query contract key: {:?}", hex::encode(contract_key));

    trace!("query input before decryption: {:?}", base64::encode(&msg));
    let secret_msg = SecretMessage::from_slice(msg)?;
    let decrypted_msg = secret_msg.decrypt()?;
    trace!(
        "query input afer decryption: {:?}",
        String::from_utf8_lossy(&decrypted_msg)
    );
    let ValidatedMessage { validated_msg, .. } =
        validate_msg(&decrypted_msg, contract_code.hash(), None)?;

    let mut engine = start_engine(
        context,
        gas_limit,
        contract_code,
        &contract_key,
        ContractOperation::Query,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    let (contract_env_bytes, _ /* no msg_info in query */) =
        env_to_env_msg_info_bytes(&engine, &mut env_v010)?;

    let env_ptr = engine.write_to_memory(&contract_env_bytes)?;
    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.query(env_ptr, msg_ptr)?;

        let output = engine.extract_vector(vec_ptr)?;

        let output = encrypt_output(
            output,
            &secret_msg,
            &CanonicalAddr(Binary(Vec::new())), // Not used for queries (can't init a new contract from a query)
            &"".to_string(), // Not used for queries (can't call a sub-message from a query),
            None,            // Not used for queries (Query response is not replied to the caller),
            &CanonicalAddr(Binary(Vec::new())), // Not used for queries (used only for replies)
            true,
        )?;
        Ok(output)
    })
    .map_err(|err| {
        *used_gas = engine.gas_used();
        err
    })?;

    *used_gas = engine.gas_used();
    Ok(QuerySuccess { output })
}

fn start_engine(
    context: Ctx,
    gas_limit: u64,
    contract_code: ContractCode,
    contract_key: &ContractKey,
    operation: ContractOperation,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<Engine, EnclaveError> {
    let module = create_module_instance(contract_code, operation)?;

    // Set the gas costs for wasm op-codes (there is an inline stack_height limit in WasmCosts)
    let wasm_costs = WasmCosts::default();

    let contract_instance = ContractInstance::new(
        context,
        module.clone(),
        gas_limit,
        wasm_costs,
        *contract_key,
        operation,
        nonce,
        user_public_key,
    )?;

    Ok(Engine::new(contract_instance, module))
}

fn env_to_env_msg_info_bytes(
    engine: &Engine,
    env_v010: &mut cosmwasm_v010_types::types::Env,
) -> Result<(Vec<u8>, Vec<u8>), EnclaveError> {
    match engine.contract_instance.cosmwasm_api_version {
        CosmWasmApiVersion::V010 => {
            // Assaf: contract_key is irrelevant inside the contract,
            // but existing v0.10 contracts might expect it to be populated :facepalm:,
            // therefore we are going to leave it populated :shrug:.
            // env_v010.contract_key = None;

            // in v0.10 the timestamp passed from Go was unix time in seconds
            // 10.16 time is unix time in nanoseconds, so now Go passes here unix time in nanoseconds
            // but v0.10 contracts still expect time to be in unix seconds,
            // so we need to convert it from nanoseconds to seconds
            env_v010.block.time = Timestamp::from_nanos(env_v010.block.time).seconds();

            let env_v010_bytes = serde_json::to_vec(env_v010).map_err(|err| {
                warn!(
                    "got an error while trying to serialize env_v010 (cosmwasm v0.10) into bytes {:?}: {}",
                    env_v010, err
                );
                EnclaveError::FailedToSerialize
            })?;

            let msg_info_v010_bytes: Vec<u8> = vec![]; // in v0.10 msg_info is inside env

            Ok((env_v010_bytes, msg_info_v010_bytes))
        }
        CosmWasmApiVersion::V1 => {
            let env_v1 = enclave_cosmwasm_v1_types::types::Env {
                block: enclave_cosmwasm_v1_types::types::BlockInfo {
                    height: env_v010.block.height,
                    time: Timestamp::from_nanos(env_v010.block.time),
                    chain_id: env_v010.block.chain_id.clone(),
                },
                contract: enclave_cosmwasm_v1_types::types::ContractInfo {
                    address: Addr(env_v010.contract.address.0.clone()),
                    code_hash: env_v010.contract_code_hash.clone(),
                },
            };

            let env_v1_bytes =  serde_json::to_vec(&env_v1).map_err(|err| {
                warn!(
                    "got an error while trying to serialize env_v1 (CosmWasm v1) into bytes {:?}: {}",
                    env_v1, err
                );
                EnclaveError::FailedToSerialize
            })?;

            let msg_info_v1 = enclave_cosmwasm_v1_types::types::MessageInfo {
                sender: Addr(env_v010.message.sender.0.clone()),
                funds: env_v010
                    .message
                    .sent_funds
                    .iter()
                    .map(|coin| {
                        enclave_cosmwasm_v1_types::coins::Coin::new(
                            coin.amount.u128(),
                            coin.denom.clone(),
                        )
                    })
                    .collect::<Vec<enclave_cosmwasm_v1_types::coins::Coin>>(),
            };

            let msg_info_v1_bytes =  serde_json::to_vec(&msg_info_v1).map_err(|err| {
                warn!(
                    "got an error while trying to serialize msg_info_v1 (CosmWasm v1) into bytes {:?}: {}",
                    msg_info_v1, err
                );
                EnclaveError::FailedToSerialize
            })?;

            Ok((env_v1_bytes, msg_info_v1_bytes))
        }
    }
}
