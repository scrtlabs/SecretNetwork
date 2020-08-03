/// This contains all the user-facing functions. In these functions we will be using
/// the consensus_io_exchange_keypair and a user-generated key to create a symmetric key
/// that is unique to the user and the enclave
///
use super::types::{IoNonce, SecretMessage};

use crate::cosmwasm::encoding::Binary;
use crate::cosmwasm::types::{CosmosMsg, WasmMsg, WasmOutput};
use crate::crypto::{AESKey, Ed25519PublicKey, Kdf, SIVEncryptable, KEY_MANAGER};
use enclave_ffi_types::EnclaveError;
use log::*;
use serde::Serialize;
use serde_json::json;

// {
// let receiver_human =
// msg_exec.get_mut("contract_addr").unwrap().as_str().unwrap();
//
// debug!("HERE 2?");
//
// // let test = msg_exec.as_object_mut().unwrap();
// // // *test.get_mut("cb_signature").unwrap() = json!("toml");
// // test.insert("cb_signature".to_string(), json!("toml"));
// //
// // debug!("LETS SEE {:?}", msg);
//
// let receiver_human = HumanAddr(receiver_human.to_string());
//
// let receiver_canonical_addr = CanonicalAddr::from_human(
// receiver_human.clone(),
// )
// .map_err(|err| {
// error!(
// "Couldn't translate human address: {:?} to canonical: {}",
// receiver_human, err
// );
// EnclaveError::FailedToDeserialize
// })?;
//
// // Hash(Enclave_secret | sender(current contract) | receiver (from json) | msg_to_pass)
// let callback_sig = create_callback_signature(
// &contract_addr,
// &receiver_canonical_addr,
// &msg_to_pass,
// );
//
// let new_msg_with_cb = msg_exec.as_object_mut().unwrap();
// new_msg_with_cb.insert(
// "cb_signature".to_string(),
// encode(&Sha256::digest(callback_sig.as_slice())),
// );
//
// debug!(
// "Callback sig is: {:?}",
// encode(&Sha256::digest(callback_sig.as_slice()))
// );
// // msg["wasm"]["execute"]["cb_signature"] =
// //     encode(&Sha256::digest(callback_sig.as_slice()));
// }

pub fn calc_encryption_key(nonce: &IoNonce, user_public_key: &Ed25519PublicKey) -> AESKey {
    let enclave_io_key = KEY_MANAGER.get_consensus_io_exchange_keypair().unwrap();

    let tx_encryption_ikm = enclave_io_key.diffie_hellman(user_public_key);

    let tx_encryption_key = AESKey::new_from_slice(&tx_encryption_ikm).derive_key_from_this(nonce);

    debug!("rust tx_encryption_key {:?}", tx_encryption_key.get());

    tx_encryption_key
}

fn encrypt_serializeable<T>(key: &AESKey, val: &T) -> Result<String, EnclaveError>
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

    Ok(b64_encode(encrypted_data.as_slice()))
}

fn b64_encode(data: &[u8]) -> String {
    base64::encode(data)
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

    let mut output: WasmOutput = serde_json::from_slice(&output).map_err(|err| {
        error!(
            "got an error while trying to deserialize output bytes into json {:?}: {}",
            output, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    match &mut output {
        // Output is error
        WasmOutput::ErrObject { err } => {
            // Encrypting the actual error
            let encrypted_err = encrypt_serializeable(&key, err)?;

            // Putting it inside a 'generic_err' envelope
            *err = json!({"generic_err":{"msg":encrypted_err}});
        }

        // Output is a simple string
        WasmOutput::OkString { ok } => {
            *ok = encrypt_serializeable(&key, ok)?;
        }

        // Output is an object
        // Encrypt all Wasm messages (keeps Bank, Staking, etc.. as is)
        WasmOutput::OkObject { ok } => {
            for msg in &mut ok.messages {
                if let CosmosMsg::Wasm(wasm_msg) = msg {
                    encrypt_wasm_msg(wasm_msg, nonce, user_public_key)?;
                }
            }

            // Encrypt all logs
            for log in &mut ok.log {
                log.key = encrypt_serializeable(&key, &log.key)?;
                log.value = encrypt_serializeable(&key, &log.value)?;
            }

            // If there's data at all
            if let Some(data) = &mut ok.data {
                *data = Binary::from_base64(&encrypt_serializeable(&key, data)?)?;
            }
        }
    };

    debug!("WasmOutput: {:?}", output);

    // Serialize back to json and return
    let encrypted_output = serde_json::to_vec(&output).map_err(|err| {
        error!(
            "got an error while trying to serialize output json into bytes {:?}: {}",
            output, err
        );
        EnclaveError::FailedToSerialize
    })?;

    Ok(encrypted_output)
}

fn encrypt_wasm_msg(
    wasm_msg: &mut WasmMsg,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<(), EnclaveError> {
    match wasm_msg {
        WasmMsg::Execute { msg, .. } | WasmMsg::Instantiate { msg, .. } => {
            let mut msg_to_pass =
                SecretMessage::from_base64((*msg).clone(), nonce, user_public_key)?;

            msg_to_pass.encrypt_in_place()?;
            *msg = b64_encode(&msg_to_pass.to_slice());
        }
    }

    Ok(())
}
