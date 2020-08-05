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
        WasmOutput::ErrObject { err } => {
            let encrypted_err = encrypt_serializeable(&key, err)?;

            // Putting the error inside a 'generic_err' envelope, so we can encrypt the error itself
            *err = json!({"generic_err":{"msg":encrypted_err}});
        }
        WasmOutput::OkString { ok } => {
            *ok = encrypt_serializeable(&key, ok)?;
        }
        // Encrypt all Wasm messages (keeps Bank, Staking, etc.. as is)
        WasmOutput::OkObject { ok } => {
            for msg in &mut ok.messages {
                if let CosmosMsg::Wasm(wasm_msg) = msg {
                    encrypt_wasm_msg(wasm_msg, nonce, user_public_key)?;
                }
            }

            for log in &mut ok.log {
                log.key = encrypt_serializeable(&key, &log.key)?;
                log.value = encrypt_serializeable(&key, &log.value)?;
            }

            if let Some(data) = &mut ok.data {
                *data = Binary::from_base64(&encrypt_serializeable(&key, data)?)?;
            }
        }
    };

    debug!("WasmOutput: {:?}", output);

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
        WasmMsg::Execute {
            msg,
            callback_code_hash,
            ..
        }
        | WasmMsg::Instantiate {
            msg,
            callback_code_hash,
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
        }
    }

    Ok(())
}
