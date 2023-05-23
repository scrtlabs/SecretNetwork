use crate::contract_validation::ReplyParams;
use core::fmt;

/// This contains all the user-facing functions. In these functions we will be using
/// the consensus_io_exchange_keypair and a user-generated key to create a symmetric key
/// that is unique to the user and the enclave
///
use super::types::{IoNonce, SecretMessage};
use cw_types_v010::encoding::Binary;
use cw_types_v010::types::{CanonicalAddr, Coin, LogAttribute};
use cw_types_v1::results::{Event, Reply, ReplyOn, SubMsg, SubMsgResponse, SubMsgResult};

use enclave_ffi_types::EnclaveError;

use enclave_crypto::{AESKey, Ed25519PublicKey, Kdf, SIVEncryptable, KEY_MANAGER};

use log::*;
use serde::{Deserialize, Serialize};
use serde_json::json;
use serde_json::Value;
use sha2::Digest;

/// The internal_reply_enclave_sig is being passed with the reply (Only if the reply is wasm reply)
/// This is used by the receiver of the reply to:
/// a. Verify the sender (Cotnract address)
/// b. Authenticate the reply.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(untagged)]
pub enum RawWasmOutput {
    Err {
        #[serde(rename = "Err")]
        err: Value,
        internal_msg_id: Option<Binary>,
        internal_reply_enclave_sig: Option<Binary>,
    },
    QueryOkV010 {
        #[serde(rename = "Ok")]
        ok: String,
    },
    QueryOkV1 {
        #[serde(rename = "ok")]
        ok: String,
    },
    OkV010 {
        #[serde(rename = "Ok")]
        ok: cw_types_v010::types::ContractResult,
        internal_reply_enclave_sig: Option<Binary>,
        internal_msg_id: Option<Binary>,
    },
    OkV1 {
        #[serde(rename = "Ok")]
        ok: cw_types_v1::results::Response,
        internal_reply_enclave_sig: Option<Binary>,
        internal_msg_id: Option<Binary>,
    },
    OkIBCPacketReceive {
        #[serde(rename = "Ok")]
        ok: cw_types_v1::ibc::IbcReceiveResponse,
    },
    OkIBCOpenChannel {
        #[serde(rename = "Ok")]
        ok: cw_types_v1::ibc::IbcChannelOpenResponse,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct V010WasmOutput {
    #[serde(rename = "Ok")]
    pub ok: Option<cw_types_v010::types::ContractResult>,
    #[serde(rename = "Err")]
    pub err: Option<Value>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct V1WasmOutput {
    #[serde(rename = "Ok")]
    pub ok: Option<cw_types_v1::results::Response>,
    #[serde(rename = "Err")]
    pub err: Option<Value>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct IBCOutput {
    #[serde(rename = "ok")]
    pub ok: Option<cw_types_v1::ibc::IbcBasicResponse>,
    #[serde(rename = "Err")]
    pub err: Option<Value>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct IBCReceiveOutput {
    #[serde(rename = "ok")]
    pub ok: Option<cw_types_v1::ibc::IbcReceiveResponse>,
    #[serde(rename = "Err")]
    pub err: Option<Value>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct IBCOpenChannelOutput {
    #[serde(rename = "ok")]
    pub ok: Option<String>,
    #[serde(rename = "Err")]
    pub err: Option<Value>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct QueryOutput {
    #[serde(rename = "Ok")]
    pub ok: Option<String>,
    #[serde(rename = "Err")]
    pub err: Option<Value>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, Default)]
pub struct WasmOutput {
    pub v010: Option<V010WasmOutput>,
    pub v1: Option<V1WasmOutput>,
    pub ibc_basic: Option<IBCOutput>,
    pub ibc_packet_receive: Option<IBCReceiveOutput>,
    pub ibc_open_channel: Option<IBCOpenChannelOutput>,
    pub query: Option<QueryOutput>,
    pub internal_reply_enclave_sig: Option<Binary>,
    pub internal_msg_id: Option<Binary>,
}

pub fn calc_encryption_key(nonce: &IoNonce, user_public_key: &Ed25519PublicKey) -> AESKey {
    let enclave_io_key = KEY_MANAGER.get_consensus_io_exchange_keypair().unwrap();

    let tx_encryption_ikm = enclave_io_key.current.diffie_hellman(user_public_key);

    let tx_encryption_key = AESKey::new_from_slice(&tx_encryption_ikm).derive_key_from_this(nonce);

    // trace!("rust tx_encryption_key {:?}", tx_encryption_key.get());

    tx_encryption_key
}

fn encrypt_serializable<T>(
    key: &AESKey,
    val: &T,
    reply_params: &Option<Vec<ReplyParams>>,
    should_append_all_reply_params: bool,
) -> Result<String, EnclaveError>
where
    T: ?Sized + Serialize,
{
    let serialized: String = serde_json::to_string(val).map_err(|err| {
        debug!("got an error while trying to encrypt output error {}", err);
        EnclaveError::EncryptionError
    })?;

    let trimmed = serialized.trim_start_matches('"').trim_end_matches('"');

    encrypt_preserialized_string(key, trimmed, reply_params, should_append_all_reply_params)
}

// use this to encrypt a String that has already been serialized.  When that is the case, if
// encrypt_serializable is called instead, it will get double serialized, and any escaped
// characters will be double escaped
fn encrypt_preserialized_string(
    key: &AESKey,
    val: &str,
    reply_params: &Option<Vec<ReplyParams>>,
    should_append_all_reply_params: bool,
) -> Result<String, EnclaveError> {
    let serialized = match reply_params {
        Some(v) => {
            let mut ser = vec![];
            ser.extend_from_slice(&v[0].recipient_contract_hash);
            if should_append_all_reply_params {
                for item in v.iter().skip(1) {
                    ser.extend_from_slice(cw_types_v1::results::REPLY_ENCRYPTION_MAGIC_BYTES);
                    ser.extend_from_slice(&item.sub_msg_id.to_be_bytes());
                    ser.extend_from_slice(item.recipient_contract_hash.as_slice());
                }
            }
            ser.extend_from_slice(val.as_bytes());
            ser
        }
        None => val.as_bytes().to_vec(),
    };
    let encrypted_data = key
        .encrypt_siv(serialized.as_slice(), None)
        .map_err(|err| {
            debug!(
                "got an error while trying to encrypt output error {:?}: {}",
                err, err
            );
            EnclaveError::EncryptionError
        })?;

    Ok(b64_encode(encrypted_data.as_slice()))
}

fn b64_encode(data: &[u8]) -> String {
    base64::encode(data)
}

#[allow(clippy::too_many_arguments)]
pub fn post_process_output(
    output: Vec<u8>,
    secret_msg: &SecretMessage,
    contract_addr: &CanonicalAddr,
    contract_hash: &str,
    reply_params: Option<Vec<ReplyParams>>,
    sender_addr: &CanonicalAddr,
    is_query_output: bool,
    is_ibc_output: bool,
) -> Result<Vec<u8>, EnclaveError> {
    let mut raw_output = deserialize_output(output)?;
    raw_output = attach_reply_headers_to_submsgs(raw_output, contract_hash, &reply_params)?;
    raw_output = encrypt_output(
        raw_output,
        secret_msg,
        contract_addr,
        &reply_params,
        is_ibc_output,
    )?;
    raw_output = create_callback_sig_for_submsgs(raw_output, contract_addr)?;
    raw_output = adapt_output_for_reply(raw_output, &reply_params, secret_msg, sender_addr)?;

    let output = finalize_raw_output(raw_output, is_query_output, is_ibc_output, true)?;
    Ok(output)
}

/// Converts a RawWasmOutput to the final interface that the go code expects, WasmOutput
/// (serialized to bytes)
///
/// # Arguments
///
/// * `raw_output` - A RawWasmOutput enum, containing the raw output of the Wasm contract.
/// * `is_query_output` - A boolean indicating whether the output is a query output.
/// * `is_ibc` - A boolean indicating whether the output is related to IBC.
/// * `is_msg_encrypted` - A boolean indicating whether the message is encrypted.
///
/// # Returns
///
/// A Result containing a vector of bytes representing the serialized WasmOutput, or an EnclaveError.
pub fn finalize_raw_output(
    raw_output: RawWasmOutput,
    is_query_output: bool,
    is_ibc: bool,
    is_msg_encrypted: bool,
) -> Result<Vec<u8>, EnclaveError> {
    let mut wasm_output = WasmOutput::default();

    match raw_output {
        RawWasmOutput::Err {
            err,
            internal_msg_id,
            internal_reply_enclave_sig,
        } => {
            if is_query_output {
                wasm_output.query = Some(QueryOutput {
                    ok: None,
                    err: Some(err),
                });
            } else {
                wasm_output.v010 = Some(V010WasmOutput {
                    err: match is_msg_encrypted {
                        true => Some(err),
                        // FIXME: probably no need for formatting the error here, otherwise it is
                        //  double-formatted, but I didn't want to change the data
                        false => Some(format_generic_error_message(err)),
                    },
                    ok: None,
                });
                wasm_output.internal_reply_enclave_sig = internal_reply_enclave_sig;
                wasm_output.internal_msg_id = internal_msg_id;
            }
        }
        RawWasmOutput::OkV010 {
            ok,
            internal_reply_enclave_sig,
            internal_msg_id,
        } => {
            wasm_output.v010 = Some(V010WasmOutput {
                err: None,
                ok: Some(ok),
            });
            wasm_output.internal_reply_enclave_sig = internal_reply_enclave_sig;
            wasm_output.internal_msg_id = internal_msg_id;
        }
        RawWasmOutput::OkV1 {
            ok,
            internal_reply_enclave_sig,
            internal_msg_id,
        } => {
            wasm_output.internal_reply_enclave_sig = internal_reply_enclave_sig;
            wasm_output.internal_msg_id = internal_msg_id;

            if is_ibc {
                wasm_output.ibc_basic = Some(IBCOutput {
                    err: None,
                    ok: Some(cw_types_v1::ibc::IbcBasicResponse::new(
                        ok.messages,
                        ok.attributes,
                        ok.events,
                    )),
                });
            } else {
                wasm_output.v1 = Some(V1WasmOutput {
                    err: None,
                    ok: Some(ok),
                });
            }
        }
        RawWasmOutput::QueryOkV010 { ok } | RawWasmOutput::QueryOkV1 { ok } => {
            wasm_output.query = Some(QueryOutput {
                ok: Some(ok),
                err: None,
            });
        }
        RawWasmOutput::OkIBCPacketReceive { ok } => {
            wasm_output.ibc_packet_receive = Some(IBCReceiveOutput {
                err: None,
                ok: Some(ok),
            });
        }
        RawWasmOutput::OkIBCOpenChannel { ok } => {
            wasm_output.ibc_open_channel = Some(IBCOpenChannelOutput {
                err: None,
                ok: match ok {
                    Some(o) => Some(o.version),
                    None => Some("".to_string()),
                },
            });
        }
    };

    trace!("WasmOutput: {:?}", wasm_output);

    let serialized_output = serde_json::to_vec(&wasm_output).map_err(|err| {
        debug!(
            "got an error while trying to serialize wasm_output into json bytes {:?}: {}",
            wasm_output, err
        );
        EnclaveError::FailedToSerialize
    })?;

    Ok(serialized_output)
}

pub fn manipulate_callback_sig_for_plaintext(
    contract_addr: &CanonicalAddr,
    output: Vec<u8>,
) -> Result<RawWasmOutput, EnclaveError> {
    let mut raw_output: RawWasmOutput = serde_json::from_slice(&output).map_err(|err| {
        warn!("got an error while trying to deserialize output bytes into json");
        trace!("output: {:?} error: {:?}", output, err);
        EnclaveError::FailedToDeserialize
    })?;

    match &mut raw_output {
        RawWasmOutput::OkV1 { ok, .. } => {
            for sub_msg in &mut ok.messages {
                if let cw_types_v1::results::CosmosMsg::Wasm(wasm_msg) = &mut sub_msg.msg {
                    match wasm_msg {
                        cw_types_v1::results::WasmMsg::Execute {
                            callback_sig,
                            msg,
                            funds,
                            ..
                        }
                        | cw_types_v1::results::WasmMsg::Instantiate {
                            callback_sig,
                            msg,
                            funds,
                            ..
                        } => {
                            *callback_sig = Some(create_callback_signature(
                                contract_addr,
                                &msg.0,
                                &funds
                                    .iter()
                                    .map(|coin| cw_types_v010::types::Coin {
                                        denom: coin.denom.clone(),
                                        amount: cw_types_v010::math::Uint128(coin.amount.u128()),
                                    })
                                    .collect::<Vec<cw_types_v010::types::Coin>>()[..],
                            ));
                        }
                    }
                }
            }
        }
        RawWasmOutput::OkIBCPacketReceive { ok } => {
            for sub_msg in &mut ok.messages {
                if let cw_types_v1::results::CosmosMsg::Wasm(wasm_msg) = &mut sub_msg.msg {
                    match wasm_msg {
                        cw_types_v1::results::WasmMsg::Execute {
                            callback_sig,
                            msg,
                            funds,
                            ..
                        }
                        | cw_types_v1::results::WasmMsg::Instantiate {
                            callback_sig,
                            msg,
                            funds,
                            ..
                        } => {
                            *callback_sig = Some(create_callback_signature(
                                contract_addr,
                                &msg.as_slice().to_vec(),
                                &funds
                                    .iter()
                                    .map(|coin| Coin {
                                        denom: coin.denom.clone(),
                                        amount: cw_types_v010::math::Uint128(coin.amount.u128()),
                                    })
                                    .collect::<Vec<Coin>>()[..],
                            ));
                        }
                    }
                }
            }
        }
        _ => {}
    }

    Ok(raw_output)
}

pub fn set_attributes_to_plaintext(attributes: &mut Vec<LogAttribute>) {
    for attr in attributes {
        attr.encrypted = false;
    }
}

pub fn set_all_logs_to_plaintext(raw_output: &mut RawWasmOutput) {
    match raw_output {
        RawWasmOutput::OkV1 { ok, .. } => {
            set_attributes_to_plaintext(&mut ok.attributes);
            for ev in &mut ok.events {
                set_attributes_to_plaintext(&mut ev.attributes);
            }
        }
        RawWasmOutput::OkIBCPacketReceive { ok } => {
            set_attributes_to_plaintext(&mut ok.attributes);
            for ev in &mut ok.events {
                set_attributes_to_plaintext(&mut ev.attributes);
            }
        }
        _ => {}
    }
}

fn deserialize_output(output: Vec<u8>) -> Result<RawWasmOutput, EnclaveError> {
    trace!(
        "output as received from contract: {:?}",
        String::from_utf8_lossy(&output)
    );

    let output: RawWasmOutput = serde_json::from_slice(&output).map_err(|err| {
        warn!("got an error while trying to deserialize output bytes from json");
        debug!("output: {:?} error: {:?}", output, err);
        EnclaveError::FailedToDeserialize
    })?;

    info!("Output after deserialization: {:?}", output);

    Ok(output)
}

/// Encrypts the output of a contract, this causes it to be known only to the User who triggered it,
/// and the enclave itself.
/// The encryption uses a symmetric key which is known only to the user and the enclave, as it is
/// derived from a diffie-hellman exchange between the user and the enclave's respective public keys.
///
/// # Arguments
///
/// * `output` - A `RawWasmOutput` that represents the output of a Contract.
/// * `secret_msg` - A `SecretMessage` struct that contains the nonce and user public key needed for encryption.
/// * `contract_addr` - The address of the contract whose output we are processing.
/// * `reply_params` - An optional vector describing the caller chain. Needed because the
///         immediate caller to this contract will be appended to every field.
fn encrypt_output(
    mut output: RawWasmOutput,
    secret_msg: &SecretMessage,
    contract_addr: &CanonicalAddr,
    reply_params: &Option<Vec<ReplyParams>>,
    is_ibc_output: bool,
) -> Result<RawWasmOutput, EnclaveError> {
    // The output we receive from a contract could be a reply to a caller contract (via the "reply" endpoint).
    // Therefore if reply_recipient_contract_hash is "Some", we append it to any encrypted data besides submessages that are irrelevant for replies.
    // More info in: https://github.com/CosmWasm/cosmwasm/blob/v1.0.0/packages/std/src/results/submessages.rs#L192-L198
    let encryption_key = calc_encryption_key(&secret_msg.nonce, &secret_msg.user_public_key);
    info!(
        "message nonce and public key for encryption: {:?} {:?}",
        secret_msg.nonce, secret_msg.user_public_key
    );

    match &mut output {
        RawWasmOutput::Err { err, .. } => {
            let encrypted_err = encrypt_serializable(&encryption_key, err, reply_params, false)?;
            *err = format_generic_error_message(Value::String(encrypted_err));
        }
        RawWasmOutput::QueryOkV010 { ok } | RawWasmOutput::QueryOkV1 { ok } => {
            *ok = encrypt_serializable(&encryption_key, ok, reply_params, false)?;
        }
        RawWasmOutput::OkV010 { ok, .. } => {
            for msg in &mut ok.messages {
                // Encrypt all Wasm messages (keeps Bank, Staking, etc.. as is)
                if let cw_types_v010::types::CosmosMsg::Wasm(wasm_msg) = msg {
                    encrypt_v010_wasm_msg(
                        wasm_msg,
                        secret_msg.nonce,
                        secret_msg.user_public_key,
                        contract_addr,
                    )?;
                }
            }

            // v0.10: The logs that will be emitted as part of a "wasm" event.
            for log in ok.log.iter_mut().filter(|log| log.encrypted) {
                log.key = encrypt_preserialized_string(&encryption_key, &log.key, &None, false)?;
                log.value =
                    encrypt_preserialized_string(&encryption_key, &log.value, &None, false)?;
            }

            if let Some(data) = &mut ok.data {
                *data = Binary::from_base64(&encrypt_serializable(
                    &encryption_key,
                    data,
                    reply_params,
                    false,
                )?)?;
            }
        }
        RawWasmOutput::OkV1 { ok, .. } => {
            encrypt_v1_non_result_fields(
                &mut ok.messages,
                &mut ok.attributes,
                &mut ok.events,
                secret_msg,
            )?;
            if let Some(data) = &mut ok.data {
                if is_ibc_output {
                    warn!("IBC output should not contain any data");
                    return Err(EnclaveError::InternalError);
                }

                *data = Binary::from_base64(&encrypt_serializable(
                    &encryption_key,
                    data,
                    reply_params,
                    false,
                )?)?;
            }
        }
        RawWasmOutput::OkIBCPacketReceive { ok } => {
            encrypt_v1_non_result_fields(
                &mut ok.messages,
                &mut ok.attributes,
                &mut ok.events,
                secret_msg,
            )?;

            ok.acknowledgement = Binary::from_base64(&encrypt_serializable(
                &encryption_key,
                &ok.acknowledgement,
                reply_params,
                false,
            )?)?;
        }
        RawWasmOutput::OkIBCOpenChannel { ok: _ } => {}
    };

    Ok(output)
}

fn encrypt_v1_non_result_fields<T: Clone + fmt::Debug + PartialEq>(
    messages: &mut [SubMsg<T>],
    attributes: &mut [LogAttribute],
    events: &mut [Event],
    secret_msg: &SecretMessage,
) -> Result<(), EnclaveError> {
    let encryption_key = calc_encryption_key(&secret_msg.nonce, &secret_msg.user_public_key);

    for sub_msg in messages.iter_mut() {
        encrypt_wasm_submsg(sub_msg, secret_msg)?;
    }

    // v1: The attributes that will be emitted as part of a "wasm" event.
    for attr in attributes.iter_mut().filter(|attr| attr.encrypted) {
        attr.key = encrypt_preserialized_string(&encryption_key, &attr.key, &None, false)?;
        attr.value = encrypt_preserialized_string(&encryption_key, &attr.value, &None, false)?;
    }

    // v1: Extra, custom events separate from the main wasm one. These will have "wasm-"" prepended to the type.
    for event in events.iter_mut() {
        for attr in event.attributes.iter_mut().filter(|attr| attr.encrypted) {
            attr.key = encrypt_preserialized_string(&encryption_key, &attr.key, &None, false)?;
            attr.value = encrypt_preserialized_string(&encryption_key, &attr.value, &None, false)?;
        }
    }

    Ok(())
}

fn encrypt_wasm_submsg<T: Clone + fmt::Debug + PartialEq>(
    sub_msg: &mut SubMsg<T>,
    secret_msg: &SecretMessage,
) -> Result<(), EnclaveError> {
    // Messages other than Wasm (Bank, Staking, etc.) are kept plaintext
    if let cw_types_v1::results::CosmosMsg::Wasm(wasm_msg) = &mut sub_msg.msg {
        match wasm_msg {
            cw_types_v1::results::WasmMsg::Instantiate { msg, .. }
            | cw_types_v1::results::WasmMsg::Execute { msg, .. } => {
                let mut msg_to_encrypt = SecretMessage {
                    msg: msg.as_slice().to_vec(),
                    nonce: secret_msg.nonce,
                    user_public_key: secret_msg.user_public_key,
                };
                msg_to_encrypt.encrypt_in_place()?;
                *msg = Binary::from(msg_to_encrypt.to_vec().as_slice());
            }
        }
    };

    Ok(())
}

/// Attaches reply headers to submessages of a Contract output.
/// We cannot use the normal field msg_id of the submessage since it is private data that goes
/// out of the contract, therefore we encrypt it and put it inside the data of the submessage itself,
/// along with all other msg_ids in the call chain. This ensures only the calling contract knows the
/// reply's msg_id.
///
/// # Arguments
///
/// * `output` - A `RawWasmOutput` that represents the output of a Contract.
/// * `reply_params` - An optional vector describing the caller chain.
/// * `contract_hash` - The hash of the code of the contract whose output we are processing.
fn attach_reply_headers_to_submsgs(
    mut output: RawWasmOutput,
    contract_hash: &str,
    reply_params: &Option<Vec<ReplyParams>>,
) -> Result<RawWasmOutput, EnclaveError> {
    let sub_msgs = match &mut output {
        RawWasmOutput::OkV1 { ok, .. } => &mut ok.messages,
        RawWasmOutput::OkIBCPacketReceive { ok } => &mut ok.messages,
        _ => return Ok(output),
    };

    for sub_msg in sub_msgs {
        if let cw_types_v1::results::CosmosMsg::Wasm(wasm_msg) = &mut sub_msg.msg {
            attach_reply_headers_to_v1_wasm_msg(
                wasm_msg,
                &sub_msg.reply_on,
                sub_msg.id,
                contract_hash,
                reply_params,
            )?;

            // The ID can be extracted from the encrypted wasm msg
            // We don't encrypt it here to remain with the same type (u64)
            sub_msg.id = 0;
        }

        sub_msg.was_msg_encrypted = true;
    }

    Ok(output)
}

/// Attaches callback signatures to every submessage of a Contract output.
/// These are needed to prove to the callee contract that the execution message it receives
/// was authored by this contract, inside the enclave.
///
/// # Arguments
///
/// * `output` - A `RawWasmOutput` that represents the output of a Contract.
/// * `contract_addr` - The address of the contract whose output we are processing.
fn create_callback_sig_for_submsgs(
    mut output: RawWasmOutput,
    contract_addr: &CanonicalAddr,
) -> Result<RawWasmOutput, EnclaveError> {
    let sub_msgs = match &mut output {
        RawWasmOutput::OkV1 { ok, .. } => &mut ok.messages,
        RawWasmOutput::OkIBCPacketReceive { ok } => &mut ok.messages,
        _ => return Ok(output),
    };

    for sub_msg in sub_msgs {
        if let cw_types_v1::results::CosmosMsg::Wasm(wasm_msg) = &mut sub_msg.msg {
            match wasm_msg {
                cw_types_v1::results::WasmMsg::Execute {
                    msg,
                    callback_sig,
                    funds,
                    ..
                }
                | cw_types_v1::results::WasmMsg::Instantiate {
                    msg,
                    callback_sig,
                    funds,
                    ..
                } => {
                    *callback_sig = Some(create_callback_signature(
                        contract_addr,
                        &SecretMessage::from_slice(msg.as_slice())?.msg,
                        &funds
                            .iter()
                            .map(|coin| Coin {
                                denom: coin.denom.clone(),
                                amount: cw_types_v010::math::Uint128(coin.amount.u128()),
                            })
                            .collect::<Vec<Coin>>()[..],
                    ));
                }
            }
        }
    }

    Ok(output)
}

/// Adapt the output of a contract to be returned as a Reply message.
/// If the contract's execution was not called from another contract, the output is returned as is.
/// Otherwise, the fields `internal_msg_id` and `internal_reply_enclave_sig` of the output are populated.
///     `internal_msg_id` contains all the message_ids in the reply params, encrypted.
///     `internal_reply_enclave_sig` is a signature of the `SubMsgResult` message that should be
///         created for this output outside the enclave.
///
/// # Arguments
///
/// * `output` - A `RawWasmOutput` that represents the output of a Contract.
/// * `reply_params` - An optional vector describing the caller chain.
/// * `secret_msg` - A `SecretMessage` struct that contains the nonce and user public key needed for encryption.
/// * `sender_addr` - The address of the sender of the message.
fn adapt_output_for_reply(
    mut output: RawWasmOutput,
    reply_params: &Option<Vec<ReplyParams>>,
    secret_msg: &SecretMessage,
    sender_addr: &CanonicalAddr,
) -> Result<RawWasmOutput, EnclaveError> {
    if reply_params.is_none() {
        // This message was not called from another contract,
        // no need to adapt output as a reply
        return Ok(output);
    }

    let encryption_key = calc_encryption_key(&secret_msg.nonce, &secret_msg.user_public_key);

    let output_result;
    let should_append_reply_params;

    match &output {
        RawWasmOutput::Err { err, .. } => {
            let mut encrypted_error_message = err["generic_err"]["msg"].to_string();

            // remove surrounding quotes
            encrypted_error_message.pop();
            encrypted_error_message.remove(0);

            output_result = SubMsgResult::Err(encrypted_error_message);
            should_append_reply_params = true;
        }
        RawWasmOutput::OkV010 { ok, .. } => {
            output_result = SubMsgResult::Ok(SubMsgResponse {
                events: vec![],
                data: ok.data.clone(),
            });

            should_append_reply_params = false;
        }
        RawWasmOutput::OkV1 { ok, .. } => {
            output_result = SubMsgResult::Ok(SubMsgResponse {
                events: vec![],
                data: ok.data.clone(),
            });

            should_append_reply_params = true;
        }
        _ => return Ok(output),
    }

    match &mut output {
        RawWasmOutput::Err {
            internal_msg_id,
            internal_reply_enclave_sig,
            ..
        }
        | RawWasmOutput::OkV010 {
            internal_msg_id,
            internal_reply_enclave_sig,
            ..
        }
        | RawWasmOutput::OkV1 {
            internal_msg_id,
            internal_reply_enclave_sig,
            ..
        } => {
            let (msg_id, callback_sig) = get_reply_info_for_output(
                output_result,
                reply_params,
                encryption_key,
                sender_addr,
                should_append_reply_params,
            )?;

            *internal_msg_id = Some(msg_id);
            *internal_reply_enclave_sig = Some(callback_sig);
        }
        _ => {}
    }

    Ok(output)
}

fn get_reply_info_for_output(
    output_result: SubMsgResult,
    reply_params: &Option<Vec<ReplyParams>>,
    encryption_key: AESKey,
    sender_addr: &CanonicalAddr,
    should_append_all_reply_params: bool,
) -> Result<(Binary, Binary), EnclaveError> {
    let encrypted_id = Binary::from_base64(&encrypt_preserialized_string(
        &encryption_key,
        &reply_params.as_ref().unwrap()[0].sub_msg_id.to_string(),
        reply_params,
        should_append_all_reply_params,
    )?)?;

    let reply = Reply {
        id: encrypted_id.clone(),
        result: output_result,
        was_orig_msg_encrypted: true,
        is_encrypted: true,
    };

    let reply_json = serde_json::to_vec(&reply).map_err(|err| {
        warn!(
            "got an error while trying to serialize reply into bytes for internal_reply_enclave_sig  {:?}: {}",
            reply, err
        );
        EnclaveError::FailedToSerialize
    })?;

    let sig = Binary::from(create_callback_signature(sender_addr, &reply_json, &[]).as_slice());

    trace!(
        "Generated internal callback signature for msg {:?} signature is: {:?}",
        String::from_utf8_lossy(reply_json.as_slice()),
        sig
    );

    Ok((encrypted_id, sig))
}

fn encrypt_v010_wasm_msg(
    wasm_msg: &mut cw_types_v010::types::WasmMsg,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
    contract_addr: &CanonicalAddr,
) -> Result<(), EnclaveError> {
    match wasm_msg {
        cw_types_v010::types::WasmMsg::Execute {
            msg,
            callback_code_hash,
            callback_sig,
            send,
            ..
        }
        | cw_types_v010::types::WasmMsg::Instantiate {
            msg,
            callback_code_hash,
            callback_sig,
            send,
            ..
        } => {
            let mut hash_appended_msg = callback_code_hash.as_bytes().to_vec();
            hash_appended_msg.extend_from_slice(msg.as_slice());

            let mut msg_to_pass = SecretMessage::from_base64(
                Binary(hash_appended_msg).to_base64(),
                nonce,
                user_public_key,
            )?;

            msg_to_pass.encrypt_in_place()?;
            *msg = Binary::from(msg_to_pass.to_vec().as_slice());

            *callback_sig = Some(create_callback_signature(
                contract_addr,
                &msg_to_pass.msg,
                send,
            ));
        }
    }

    Ok(())
}

#[allow(clippy::too_many_arguments)]
fn attach_reply_headers_to_v1_wasm_msg(
    wasm_msg: &mut cw_types_v1::results::WasmMsg,
    reply_on: &ReplyOn,
    msg_id: u64, // In every submessage there is a field called "id", currently used only by "reply".
    reply_recipient_contract_hash: &str,
    reply_params: &Option<Vec<ReplyParams>>,
) -> Result<(), EnclaveError> {
    match wasm_msg {
        cw_types_v1::results::WasmMsg::Execute { msg, code_hash, .. }
        | cw_types_v1::results::WasmMsg::Instantiate { msg, code_hash, .. } => {
            // On cosmwasm v1, submessages execute contracts whose results are sent back to the original caller by using "Reply".
            // Such submessages should be encrypted, but they weren't initially meant to be sent back to the enclave as an input of another contract.
            // To support "sending back" behavior, the enclave expects every encrypted input to be prepended by the recipient's contract hash.
            // In this context, we prepend the message with both hashes to signal to the next wasm call that its output is going to be an input to this contract as a "Reply".
            // When decrypting the input, the enclave will try to parse the message as usual, but if the message (after reading the first code-hash) can't be parsed into json,
            // then it will treat the next 64 bytes as a recipient code-hash and prepend this code-hash to its output.
            let mut hash_appended_msg = code_hash.as_bytes().to_vec();
            if *reply_on != ReplyOn::Never {
                hash_appended_msg
                    .extend_from_slice(cw_types_v1::results::REPLY_ENCRYPTION_MAGIC_BYTES);
                hash_appended_msg.extend_from_slice(&msg_id.to_be_bytes());
                hash_appended_msg.extend_from_slice(reply_recipient_contract_hash.as_bytes());
            }

            if let Some(r) = reply_params {
                for param in r.iter() {
                    hash_appended_msg
                        .extend_from_slice(cw_types_v1::results::REPLY_ENCRYPTION_MAGIC_BYTES);
                    hash_appended_msg.extend_from_slice(&param.sub_msg_id.to_be_bytes());
                    hash_appended_msg.extend_from_slice(param.recipient_contract_hash.as_slice());
                }
            }

            hash_appended_msg.extend_from_slice(msg.as_slice());

            *msg = Binary::from(hash_appended_msg.as_slice());
        }
    }

    Ok(())
}

pub fn create_callback_signature(
    _contract_addr: &CanonicalAddr,
    msg_to_sign: &Vec<u8>,
    funds_to_send: &[Coin],
) -> Vec<u8> {
    // Hash(Enclave_secret | sender(current contract) | msg_to_pass | sent_funds)
    let mut callback_sig_bytes = KEY_MANAGER
        .get_consensus_callback_secret()
        .unwrap()
        .current
        .get()
        .to_vec();

    //callback_sig_bytes.extend(contract_addr.as_slice());
    callback_sig_bytes.extend(msg_to_sign.as_slice());
    callback_sig_bytes.extend(serde_json::to_vec(funds_to_send).unwrap());

    sha2::Sha256::digest(callback_sig_bytes.as_slice()).to_vec()
}

pub fn format_generic_error_message(encrypted_err: Value) -> Value {
    json!({"generic_err":{"msg":encrypted_err}})
}
