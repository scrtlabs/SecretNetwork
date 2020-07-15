use super::errors::WasmEngineError;
use crate::crypto::Ed25519PublicKey;
use crate::wasm::types::{IoNonce, SecretMessage};
use crate::{exports, imports};

use crate::cosmwasm::encoding::Binary;
use crate::cosmwasm::query::{QueryRequest, WasmQuery};
use crate::cosmwasm::{
    std_error::{StdError, StdResult},
    system_error::SystemResult,
};

use enclave_ffi_types::{Ctx, EnclaveBuffer, OcallReturn, UntrustedVmError};
use log::*;
use sgx_types::sgx_status_t;

pub fn encrypt_and_query_chain(
    query: &[u8],
    context: &Ctx,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<(Vec<u8>, u64), WasmEngineError> {
    let mut query_struct: QueryRequest = serde_json::from_slice(query).map_err(|err| {
        error!(
            "encrypt_and_query_chain() cannot build struct from json {:?}: {:?}",
            String::from_utf8_lossy(query),
            err
        );
        WasmEngineError::BadQueryChainRequest
    })?;

    let mut is_encrypted = false;

    if let QueryRequest::Wasm(WasmQuery::Smart {
        contract_addr: _,
        msg,
    }) = &mut query_struct
    {
        is_encrypted = true;

        let mut encrypted_msg = SecretMessage {
            msg: msg.0.clone(),
            user_public_key,
            nonce,
        };
        encrypted_msg.encrypt_in_place().map_err(|err|{
            error!(
                "encrypt_and_query_chain() got an error while trying to encrypt the request for query {:?}, stopping wasm: {:?}",
                String::from_utf8_lossy(&query),
                err
            );

            WasmEngineError::EncryptionError
        })?;

        *msg = Binary(encrypted_msg.to_slice());
    }

    let encrypted_query = serde_json::to_vec(&query_struct).map_err(|err| {
        // this should never happen
        error!(
            "encrypt_and_query_chain() cannot build json from struct {:?}: {:?}",
            query_struct, err
        );
        WasmEngineError::BadQueryChainRequest
    })?;

    let answer = query_chain(context, &encrypted_query);

    if !is_encrypted {
        return answer;
    }

    // Call query_chain (this bubbles up to x/compute via ocalls and FFI to Go code)
    // This returns the answer from x/compute
    match answer {
        Ok((encrypted_answer_as_vec, gas_used)) => {
            // answer is QueryResult (Result<Binary,StdError>) encoded be serde to bytes
            // we need to:
            //  (1) deserialize it from bytes
            //  (2) decrypt the result/stderror
            //  (3) turn in back to QueryResult as bytes

            let encrypted_answer: SystemResult<StdResult<Binary>> = serde_json::from_slice(
                &encrypted_answer_as_vec,
            ).map_err(|err|{
                  error!("encrypt_and_query_chain() got an error while trying to deserialize the answer as StdResult<Binary>: {:?}", err);
                       WasmEngineError::DeserializationError
            })?;

            trace!(
                "encrypt_and_query_chain() go encrypted answer: {:?}",
                encrypted_answer
            );

            let answer: SystemResult<StdResult<Binary>> = match encrypted_answer {
                Err(_) => encrypted_answer,
                Ok(Ok(result)) => {
                    // query response returns without nonce and user_public_key appended to it
                    // because the sender is supposed to have them already
                    let as_secret_msg = SecretMessage {
                        nonce,
                        user_public_key,
                        msg: result.0,
                    };

                    match as_secret_msg.decrypt() {
                        Ok(b64_decrypted) => {
                            let decrypted = base64::decode(&b64_decrypted).map_err(|err| {
                                error!(
                                    "encrypt_and_query_chain() got an answer, managed to decrypt it, then tried to decode the output from base64 to bytes and failed: {:?}",
                                    err
                                );
                                WasmEngineError::DeserializationError
                            })?;

                            Ok(Ok(Binary(decrypted)))
                        }
                        Err(err) => {
                            error!(
                                    "encrypt_and_query_chain() got an error while trying to decrypt the result for query {:?}, stopping wasm: {:?}",
                                    String::from_utf8_lossy(&query),
                                    err
                                );

                            return Err(WasmEngineError::DecryptionError);
                        }
                    }
                }
                Ok(Err(StdError::GenericErr { msg })) => match base64::decode(&msg) {
                    Err(err) => {
                        error!(
                            "encrypt_and_query_chain() got an StdError as an answer {:?}, tried to decode the inner msg as bytes because it's encrypted, but got an error while trying to decode from base64. This usually means that the called contract panicked and the error is plaintext: {:?}",
                            msg, err
                        );
                        Ok(Err(StdError::GenericErr { msg }))
                    }
                    Ok(inner_error_bytes) => {
                        // query response returns without nonce and user_public_key appended to it
                        // because the sender is supposed to have them already
                        let inner_error_as_secret_msg = SecretMessage {
                            nonce,
                            user_public_key,
                            msg: inner_error_bytes,
                        };

                        match inner_error_as_secret_msg.decrypt() {
                            Err(err) => {
                                error!(
                                    "encrypt_and_query_chain() got an error while trying to decrypt the inner error for query {:?}, stopping wasm: {:?}",
                                    String::from_utf8_lossy(&query),
                                    err
                                );

                                return Err(WasmEngineError::DecryptionError);
                            }
                            Ok(decrypted) => {
                                serde_json::from_slice(&decrypted).map_err(|err| {
                                    error!("encrypt_and_query_chain() got an error while trying to deserialize the inner error as StdError: {:?}", err);
                                    WasmEngineError::DeserializationError
                                })?
                            }
                        }
                    }
                },
                Ok(Err(std_error)) => {
                    error!(
                        "encrypt_and_query_chain() got an StdError as an answer, but it should be of type GenericErr and encrypted inside. Got instead: {:?}",
                        std_error
                    );
                    Ok(Err(std_error))
                }
            };

            trace!(
                "encrypt_and_query_chain() decrypted the answer to be: {:?}",
                answer
            );

            let answer_as_vec = serde_json::to_vec(&answer).map_err(|err| {
                error!("encrypt_and_query_chain() got an error while trying to serialize the decrypted answer to bytes: {:?}", err);
                WasmEngineError::SerializationError
            })?;

            Ok((answer_as_vec, gas_used))
        }
        Err(err) => Err(err),
    }
}

/// Safe wrapper around quering other contracts and modules
fn query_chain(context: &Ctx, query: &[u8]) -> Result<(Vec<u8>, u64), WasmEngineError> {
    let mut ocall_return = OcallReturn::Success;
    let mut enclave_buffer = std::mem::MaybeUninit::<EnclaveBuffer>::uninit();
    let mut vm_err = UntrustedVmError::default();
    let mut gas_used = 0_u64;
    let value = unsafe {
        let status = imports::ocall_query_chain(
            (&mut ocall_return) as *mut _,
            context.unsafe_clone(),
            (&mut vm_err) as *mut _,
            (&mut gas_used) as *mut _,
            enclave_buffer.as_mut_ptr(),
            query.as_ptr(),
            query.len(),
        );
        match status {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            error_status => {
                error!(
                    "query_chain() got an error from ocall_query_chain, stopping wasm: {:?}",
                    error_status
                );
                return Err(WasmEngineError::FailedOcall(vm_err));
            }
        }

        match ocall_return {
            OcallReturn::Success => {
                let enclave_buffer = enclave_buffer.assume_init();
                // TODO add validation of this pointer before returning its contents.
                exports::recover_buffer(enclave_buffer).unwrap_or_else(Vec::new)
            }
            OcallReturn::Failure => {
                return Err(WasmEngineError::FailedOcall(vm_err));
            }
            OcallReturn::Panic => return Err(WasmEngineError::Panic),
        }
    };

    Ok((value, gas_used))
}
