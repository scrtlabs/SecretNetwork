/// This contains all the user-facing functions. In these functions we will be using
/// the consensus_io_exchange_keypair and a user-generated key to create a symmetric key
/// that is unique to the user and the enclave
use crate::crypto::{AESKey, Kdf, SIVEncryptable, KEY_MANAGER};
use enclave_ffi_types::EnclaveError;
use log::*;
use serde_json::Value;

fn derive_dh_io_key(user_pubkey: &[u8; 32], nonce: &[u8; 32]) -> Result<AESKey, EnclaveError> {
    let enclave_io_key = KEY_MANAGER.get_consensus_io_exchange_keypair().unwrap();

    let tx_encryption_ikm = enclave_io_key.diffie_hellman(user_pubkey);

    let tx_encryption_key = AESKey::new_from_slice(&tx_encryption_ikm).derive_key_from_this(nonce);

    debug!("rust tx_encryption_key {:?}", tx_encryption_key);

    Ok(tx_encryption_key)
}

pub fn decrypt_msg(msg: &[u8]) -> Result<(Vec<u8>, [u8; 32], [u8; 32]), EnclaveError> {
    // 32 bytes of AD
    // 33 bytes of secp256k1 compressed public key
    // 16+ bytes of encrypted data
    if msg.len() < 82 {
        error!(
            "Encrypted message length {:?} is too short. Cannot parse",
            msg.len()
        );
        return Err(EnclaveError::DecryptionError);
    };

    let mut nonce = [0u8; 32];
    nonce.copy_from_slice(&msg[0..32]);

    let mut user_pubkey = [0u8; 32];
    user_pubkey.copy_from_slice(&msg[32..64]);

    let encrypted_msg = &msg[64..];

    let key = derive_dh_io_key(&user_pubkey, &nonce)?;

    // pass
    let msg = key.decrypt_siv(encrypted_msg, &vec![&[]]).map_err(|err| {
        error!(
            "handle() got an error while trying to decrypt the msg: {}",
            err
        );
        EnclaveError::DecryptionError
    })?;

    Ok((msg, user_pubkey, nonce))
}

pub fn encrypt_output(
    plaintext: &Vec<u8>,
    user_pubkey: &[u8; 32],
    nonce: &[u8; 32],
) -> Result<Vec<u8>, EnclaveError> {
    let key = derive_dh_io_key(user_pubkey, nonce)?;

    debug!(
        "Before encryption: {:?}",
        String::from_utf8_lossy(&plaintext)
    );

    let mut v: Value = serde_json::from_slice(plaintext).map_err(|err| {
        error!(
            "got an error while trying to deserialize output bytes into json {:?}: {}",
            plaintext, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    if let Value::String(err) = &v["err"] {
        v["err"] = Value::String(base64::encode(
            &key.encrypt_siv(&err.to_owned().into_bytes(), &vec![&[]])
                .map_err(|err| {
                    error!(
                        "got an error while trying to encrypt output error {:?}: {}",
                        err, err
                    );
                    EnclaveError::EncryptionError
                })?,
        ));
    } else if let Value::String(ok) = &v["ok"] {
        // query
        v["ok"] = Value::String(base64::encode(
            &key.encrypt_siv(&ok.to_owned().into_bytes(), &vec![&[]])
                .map_err(|err| {
                    error!(
                        "got an error while trying to encrypt query output {:?}: {}",
                        ok, err
                    );
                    EnclaveError::EncryptionError
                })?,
        ));
    } else if let Value::Object(ok) = &mut v["ok"] {
        // init of handle
        if let Value::Array(msgs) = &mut ok["messages"] {
            for msg in msgs {
                if let Value::String(msg_to_next_call) = &mut msg["contract"]["msg"] {
                    msg["contract"]["msg"] = Value::String(base64::encode(
                        &key.encrypt_siv(&msg_to_next_call.to_owned().into_bytes(), &vec![&[]])
                            .map_err(|err| {
                                error!(
                                    "got an error while trying to encrypt the msg to next call {:?}: {}",
                                    msg["contract"], err
                                );
                                EnclaveError::EncryptionError
                            })?,
                    ));
                }
            }
        }

        if let Value::Array(events) = &mut v["ok"]["log"] {
            for e in events {
                if let Value::String(k) = &mut e["key"] {
                    e["key"] = Value::String(base64::encode(
                        &key.encrypt_siv(&k.to_owned().into_bytes(), &vec![&[]])
                            .map_err(|err| {
                                error!(
                                    "got an error while trying to encrypt the event key {}: {}",
                                    k, err
                                );
                                EnclaveError::EncryptionError
                            })?,
                    ));
                }
                if let Value::String(v) = &mut e["value"] {
                    e["value"] = Value::String(base64::encode(
                        &key.encrypt_siv(&v.to_owned().into_bytes(), &vec![&[]])
                            .map_err(|err| {
                                error!(
                                    "got an error while trying to encrypt the event value {}: {}",
                                    v, err
                                );
                                EnclaveError::EncryptionError
                            })?,
                    ));
                }
            }
        }

        if let Value::String(data) = &mut v["ok"]["data"] {
            v["ok"]["data"] = Value::String(base64::encode(
                &key.encrypt_siv(&data.to_owned().into_bytes(), &vec![&[]])
                    .map_err(|err| {
                        error!(
                            "got an error while trying to encrypt the data section {}: {}",
                            data, err
                        );
                        EnclaveError::EncryptionError
                    })?,
            ));
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
