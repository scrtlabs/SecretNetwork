/// This contains all the user-facing functions. In these functions we will be using
/// the consensus_io_exchange_keypair and a user-generated key to create a symmetric key
/// that is unique to the user and the enclave
///
use super::types::{IoNonce, SecretMessage};

use crate::crypto::{AESKey, Ed25519PublicKey, Kdf, SIVEncryptable, KEY_MANAGER};
use enclave_ffi_types::EnclaveError;
use log::*;
use serde::Serialize;
// use serde_json;
use serde_json::Value;

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

    debug!("Before encryption: {:?}", String::from_utf8_lossy(&output));

    // Because output is conditionally in totally different structures without useful methods
    // I'm not sure there's a better way to parse this (I mean, there probably is, but whatever)
    let mut v: Value = serde_json::from_slice(&output).map_err(|err| {
        error!(
            "got an error while trying to deserialize output bytes into json {:?}: {}",
            output, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    if let Value::String(err) = &v["err"] {
        v["err"] = encrypt_serializeable(&key, &err)?;
    } else if let Value::String(ok) = &v["ok"] {
        // query
        v["ok"] = encrypt_serializeable(&key, &ok)?;
    } else if let Value::Object(ok) = &mut v["ok"] {
        // init of handle
        if let Value::Array(msgs) = &mut ok["messages"] {
            for msg in msgs {
                if let Value::String(msg_b64) = &mut msg["contract"]["msg"] {
                    let mut msg_to_pass =
                        SecretMessage::from_base64((*msg_b64).to_string(), nonce, user_public_key)?;

                    msg_to_pass.encrypt_in_place()?;

                    msg["contract"]["msg"] = encode(&msg_to_pass.to_slice());
                }
            }
        }

        if let Value::Array(events) = &mut v["ok"]["log"] {
            for e in events {
                if let Value::String(k) = &mut e["key"] {
                    e["key"] = encrypt_serializeable(&key, k)?;
                }
                if let Value::String(v) = &mut e["value"] {
                    e["value"] = encrypt_serializeable(&key, v)?;
                }
            }
        }

        if let Value::String(data) = &mut v["ok"]["data"] {
            v["ok"]["data"] = encrypt_serializeable(&key, data)?;
        }
    }

    let output = serde_json::ser::to_vec(&v).map_err(|err| {
        error!(
            "got an error while trying to serialize output json into bytes {:?}: {}",
            v, err
        );
        EnclaveError::FailedToSerialize
    })?;

    debug!("after encryption: {:?}", String::from_utf8_lossy(&output));

    Ok(output)
}
