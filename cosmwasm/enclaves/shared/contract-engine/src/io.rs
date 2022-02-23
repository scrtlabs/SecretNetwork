/// This contains all the user-facing functions. In these functions we will be using
/// the consensus_io_exchange_keypair and a user-generated key to create a symmetric key
/// that is unique to the user and the enclave
///

use super::types::{IoNonce, SecretMessage};
use enclave_ffi_types::EnclaveError;
use enclave_cosmwasm_types as cosmwasm_v010_types;
use enclave_cosmwasm_types::encoding::Binary;
use enclave_cosmwasm_types::types::{CanonicalAddr, Coin};
use enclave_cosmwasm_v016_types as cosmwasm_v016_types;

use enclave_crypto::{AESKey, Ed25519PublicKey, Kdf, SIVEncryptable, KEY_MANAGER};

use log::*;
use serde::{Deserialize, Serialize};
use serde_json::json;
use serde_json::Value;
use sha2::Digest;

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(untagged)]
enum WasmOutput {
    ErrObject {
        #[serde(rename = "Err")]
        err: Value,
    },
    OkString {
        #[serde(rename = "Ok")]
        ok: String,
    },
    OkObjectV010 {
        #[serde(rename = "Ok")]
        ok: cosmwasm_v010_types::types::ContractResult,
    },
    OkObjectV016 {
        #[serde(rename = "Ok")]
        ok: cosmwasm_v016_types::results::Response,
    },
}

pub fn calc_encryption_key(nonce: &IoNonce, user_public_key: &Ed25519PublicKey) -> AESKey {
    let enclave_io_key = KEY_MANAGER.get_consensus_io_exchange_keypair().unwrap();

    let tx_encryption_ikm = enclave_io_key.diffie_hellman(user_public_key);

    let tx_encryption_key = AESKey::new_from_slice(&tx_encryption_ikm).derive_key_from_this(nonce);

    trace!("rust tx_encryption_key {:?}", tx_encryption_key.get());

    tx_encryption_key
}

fn encrypt_serializable<T>(key: &AESKey, val: &T) -> Result<String, EnclaveError>
where
    T: ?Sized + Serialize,
{
    let serialized: String = serde_json::to_string(val).map_err(|err| {
        debug!("got an error while trying to encrypt output error {}", err);
        EnclaveError::EncryptionError
    })?;

    let trimmed = serialized.trim_start_matches('"').trim_end_matches('"');

    let encrypted_data = key.encrypt_siv(trimmed.as_bytes(), None).map_err(|err| {
        debug!(
            "got an error while trying to encrypt output error {:?}: {}",
            err, err
        );
        EnclaveError::EncryptionError
    })?;

    Ok(b64_encode(encrypted_data.as_slice()))
}

// use this to encrypt a String that has already been serialized.  When that is the case, if
// encrypt_serializable is called instead, it will get double serialized, and any escaped
// characters will be double escaped
fn encrypt_preserialized_string(key: &AESKey, val: &str) -> Result<String, EnclaveError> {
    let encrypted_data = key.encrypt_siv(val.as_bytes(), None).map_err(|err| {
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

pub fn encrypt_output(
    output: Vec<u8>,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
    contract_addr: &CanonicalAddr,
) -> Result<Vec<u8>, EnclaveError> {
    let key = calc_encryption_key(&nonce, &user_public_key);

    trace!(
        "Output before encryption: {:?}",
        String::from_utf8_lossy(&output)
    );

    let mut output: WasmOutput = serde_json::from_slice(&output).map_err(|err| {
        warn!("got an error while trying to deserialize output bytes into json");
        trace!("output: {:?} error: {:?}", output, err);
        EnclaveError::FailedToDeserialize
    })?;

    match &mut output {
        WasmOutput::ErrObject { err } => {
            let encrypted_err = encrypt_serializable(&key, err)?;

            // Putting the error inside a 'generic_err' envelope, so we can encrypt the error itself
            *err = json!({"generic_err":{"msg":encrypted_err}});
        }

        WasmOutput::OkString { ok } => {
            *ok = encrypt_serializable(&key, ok)?;
        }

        // Encrypt all Wasm messages (keeps Bank, Staking, etc.. as is)
        WasmOutput::OkObjectV010 { ok } => {
            for msg in &mut ok.messages {
                if let cosmwasm_v010_types::types::CosmosMsg::Wasm(wasm_msg) = msg {
                    encrypt_v010_wasm_msg(wasm_msg, nonce, user_public_key, contract_addr)?;
                }
            }

            // v0.10: The logs that will be emitted as part of a "wasm" event.
            for log in ok.log.iter_mut().filter(|log| log.encrypted) {
                log.key = encrypt_preserialized_string(&key, &log.key)?;
                log.value = encrypt_preserialized_string(&key, &log.value)?;
            }

            if let Some(data) = &mut ok.data {
                *data = Binary::from_base64(&encrypt_serializable(&key, data)?)?;
            }
        }
        WasmOutput::OkObjectV016 { ok } => {
            for sub_msg in &mut ok.messages {
                if let cosmwasm_v016_types::results::CosmosMsg::Wasm(wasm_msg) = &mut sub_msg.msg {
                    encrypt_v016_wasm_msg(wasm_msg, nonce, user_public_key, contract_addr)?;
                }
            }

            // v0.16: The attributes that will be emitted as part of a "wasm" event.
            for attr in ok.attributes.iter_mut().filter(|attr| attr.encrypted) {
                attr.key = encrypt_preserialized_string(&key, &attr.key)?;
                attr.value = encrypt_preserialized_string(&key, &attr.value)?;
            }

            // v0.16: Extra, custom events separate from the main wasm one. These will have "wasm-"" prepended to the type.
            for event in ok.events.iter_mut() {
                for attr in event.attributes.iter_mut().filter(|attr| attr.encrypted) {
                    attr.key = encrypt_preserialized_string(&key, &attr.key)?;
                    attr.value = encrypt_preserialized_string(&key, &attr.value)?;
                }
            }

            if let Some(data) = &mut ok.data {
                *data = cosmwasm_v016_types::binary::Binary::from_base64(&encrypt_serializable(
                    &key, data,
                )?)?;
            }
        }
    };

    trace!("WasmOutput: {:?}", output);

    let encrypted_output = serde_json::to_vec(&output).map_err(|err| {
        debug!(
            "got an error while trying to serialize output json into bytes {:?}: {}",
            output, err
        );
        EnclaveError::FailedToSerialize
    })?;

    Ok(encrypted_output)
}

fn encrypt_v010_wasm_msg(
    wasm_msg: &mut cosmwasm_v010_types::types::WasmMsg,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
    contract_addr: &CanonicalAddr,
) -> Result<(), EnclaveError> {
    match wasm_msg {
        cosmwasm_v010_types::types::WasmMsg::Execute {
            msg,
            callback_code_hash,
            callback_sig,
            send,
            ..
        }
        | cosmwasm_v010_types::types::WasmMsg::Instantiate {
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

            *callback_sig = Some(create_callback_signature(contract_addr, &msg_to_pass, send));
        }
    }

    Ok(())
}

fn encrypt_v016_wasm_msg(
    wasm_msg: &mut cosmwasm_v016_types::results::WasmMsg,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
    contract_addr: &CanonicalAddr,
) -> Result<(), EnclaveError> {
    match wasm_msg {
        cosmwasm_v016_types::results::WasmMsg::Execute {
            msg,
            code_hash,
            callback_sig,
            funds,
            ..
        }
        | cosmwasm_v016_types::results::WasmMsg::Instantiate {
            msg,
            code_hash,
            callback_sig,
            funds,
            ..
        } => {
            let mut hash_appended_msg = code_hash.as_bytes().to_vec();
            hash_appended_msg.extend_from_slice(msg.as_slice());

            let mut msg_to_pass = SecretMessage::from_base64(
                cosmwasm_v016_types::binary::Binary(hash_appended_msg).to_base64(),
                nonce,
                user_public_key,
            )?;

            msg_to_pass.encrypt_in_place()?;
            *msg = cosmwasm_v016_types::binary::Binary::from(msg_to_pass.to_vec().as_slice());

            *callback_sig = Some(create_callback_signature(
                contract_addr,
                &msg_to_pass,
                &funds
                    .iter()
                    .map(|coin| cosmwasm_v010_types::types::Coin {
                        denom: coin.denom.clone(),
                        amount: cosmwasm_v010_types::math::Uint128(coin.amount.u128()),
                    })
                    .collect::<Vec<cosmwasm_v010_types::types::Coin>>()[..],
            ));
        }
    }

    Ok(())
}

pub fn create_callback_signature(
    contract_addr: &CanonicalAddr,
    msg_to_sign: &SecretMessage,
    funds_to_send: &[Coin],
) -> Vec<u8> {
    // Hash(Enclave_secret | sender(current contract) | msg_to_pass | sent_funds)
    let mut callback_sig_bytes = KEY_MANAGER
        .get_consensus_callback_secret()
        .unwrap()
        .get()
        .to_vec();

    callback_sig_bytes.extend(contract_addr.as_slice());
    callback_sig_bytes.extend(msg_to_sign.msg.as_slice());
    callback_sig_bytes.extend(serde_json::to_vec(funds_to_send).unwrap());

    sha2::Sha256::digest(callback_sig_bytes.as_slice()).to_vec()
}
