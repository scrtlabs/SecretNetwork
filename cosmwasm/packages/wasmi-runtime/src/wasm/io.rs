/// This contains all the user-facing functions. In these functions we will be using
/// the consensus_io_exchange_keypair and a user-generated key to create a symmetric key
/// that is unique to the user and the enclave
///
use super::types::{IoNonce, SecretMessage};

use crate::cosmwasm::types::WasmOutput;
use crate::cosmwasm::types::WasmOutput::ErrString;
use crate::crypto::{AESKey, Ed25519PublicKey, Kdf, SIVEncryptable, KEY_MANAGER};
use enclave_ffi_types::EnclaveError;
use log::*;
use serde::Serialize;
use serde_json::{json, Value};

pub fn calc_encryption_key(nonce: &IoNonce, user_public_key: &Ed25519PublicKey) -> AESKey {
    let enclave_io_key = KEY_MANAGER.get_consensus_io_exchange_keypair().unwrap();

    let tx_encryption_ikm = enclave_io_key.diffie_hellman(user_public_key);

    let tx_encryption_key = AESKey::new_from_slice(&tx_encryption_ikm).derive_key_from_this(nonce);

    debug!("rust tx_encryption_key {:?}", tx_encryption_key.get());

    tx_encryption_key
}

fn encrypt_serializeable<T>(key: &AESKey, val: &T) -> Result<Value, EnclaveError>
where
    T: ?Sized + Serialize,
{
    let serialized: String = serde_json::to_string(val).map_err(|err| {
        error!(
            "got an error while trying to encrypt output error {:?}: {}",
            err, err
        );
        EnclaveError::EncryptionError
    })?;

    // todo: think about if we should just move this function to handle only serde_json::Value::Strings
    // instead of removing the extra quotes like this
    let trimmed = serialized.trim_start_matches('"').trim_end_matches('"');

    let encrypted_data = key.encrypt_siv(trimmed.as_bytes(), None).map_err(|err| {
        error!(
            "got an error while trying to encrypt output error {:?}: {}",
            err, err
        );
        EnclaveError::EncryptionError
    })?;

    Ok(encode(encrypted_data.as_slice()))
}

fn encode(data: &[u8]) -> Value {
    Value::String(base64::encode(data))
}

pub fn encrypt_output(
    output: Vec<u8>,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<Vec<u8>, EnclaveError> {
    let key = calc_encryption_key(&nonce, &user_public_key);

    debug!(
        "Output before encryption: {:?}",
        String::from_utf8_lossy(&output)
    );

    let output: WasmOutput = serde_json::from_slice(&output).map_err(|err| {
        error!(
            "got an error while trying to deserialize output bytes into json {:?}: {}",
            output, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let mut new_output: Value;

    match output.clone() {
        WasmOutput::ErrString { err } => {}
        WasmOutput::OkString { ok } => {
            let encrypted = encrypt_serializeable(&key, &ok)?;

            new_output = serde_json::to_value(output).unwrap();
            new_output["Ok"] = encrypted;
        }
        WasmOutput::OkNested { ok } => {}
    };

    debug!("WasmOutput: {:?}", new_output);

    // Because output is conditionally in totally different structures without useful methods
    // I'm not sure there's a better way to parse this (I mean, there probably is, but whatever)
    // let mut v: Value = serde_json::from_slice(&output).map_err(|err| {
    //     error!(
    //         "got an error while trying to deserialize output bytes into json {:?}: {}",
    //         output, err
    //     );
    //     EnclaveError::FailedToDeserialize
    // })?;

    // if v["Err"].is_object() {
    //     if let Value::Object(err) = &mut v["Err"] {
    //         let mut new_value: Value = json!({"generic_err":{"msg":""}});
    //         new_value["generic_err"]["msg"] = encrypt_serializeable(&key, &err)?;
    //         v["Err"] = new_value;
    //     }
    // } else if v["Ok"].is_string() {
    //     // query
    //     if let Value::String(ok) = &v["Ok"] {
    //         v["Ok"] = encrypt_serializeable(&key, &ok)?;
    //     }
    // } else if v["Ok"].is_object() {
    //     // init or handle or migrate
    //     if let Value::Object(ok) = &mut v["Ok"] {
    //         if ok["messages"].is_array() {
    //             if let Value::Array(msgs) = &mut ok["messages"] {
    //                 for msg in msgs {
    //                     if msg["wasm"]["execute"]["msg"].is_string() {
    //                         if let Value::String(msg_b64) = &mut msg["wasm"]["execute"]["msg"] {
    //                             let mut msg_to_pass = SecretMessage::from_base64(
    //                                 (*msg_b64).to_string(),
    //                                 nonce,
    //                                 user_public_key,
    //                             )?;
    //
    //                             msg_to_pass.encrypt_in_place()?;
    //
    //                             msg["wasm"]["execute"]["msg"] = encode(&msg_to_pass.to_slice());
    //                         }
    //                     } else if msg["wasm"]["instantiate"]["msg"].is_string() {
    //                         if let Value::String(msg_b64) = &mut msg["wasm"]["instantiate"]["msg"] {
    //                             let mut msg_to_pass = SecretMessage::from_base64(
    //                                 (*msg_b64).to_string(),
    //                                 nonce,
    //                                 user_public_key,
    //                             )?;
    //
    //                             msg_to_pass.encrypt_in_place()?;
    //
    //                             msg["wasm"]["instantiate"]["msg"] = encode(&msg_to_pass.to_slice());
    //                         }
    //                     }
    //                 }
    //             }
    //         }
    //
    //         if ok["log"].is_array() {
    //             if let Value::Array(events) = &mut ok["log"] {
    //                 for e in events {
    //                     if e["key"].is_string() {
    //                         if let Value::String(k) = &mut e["key"] {
    //                             e["key"] = encrypt_serializeable(&key, k)?;
    //                         }
    //                     }
    //                     if e["value"].is_string() {
    //                         if let Value::String(v) = &mut e["value"] {
    //                             e["value"] = encrypt_serializeable(&key, v)?;
    //                         }
    //                     }
    //                 }
    //             }
    //         }
    //
    //         if v["Ok"]["data"].is_string() {
    //             if let Value::String(data) = &mut v["Ok"]["data"] {
    //                 v["Ok"]["data"] = encrypt_serializeable(&key, data)?;
    //             }
    //         }
    //     }
    // }
    //
    // let output = serde_json::ser::to_vec(&v).map_err(|err| {
    //     error!(
    //         "got an error while trying to serialize output json into bytes {:?}: {}",
    //         v, err
    //     );
    //     EnclaveError::FailedToSerialize
    // })?;
    //
    // debug!(
    //     "Output after encryption: {:?}",
    //     String::from_utf8_lossy(&output)
    // );
    //
    // Ok(output)

    unimplemented!()
}
