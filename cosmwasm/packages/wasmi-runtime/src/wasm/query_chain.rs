use super::errors::WasmEngineError;
use crate::crypto::Ed25519PublicKey;
use crate::wasm::types::{IoNonce, SecretMessage};
use crate::{exports, imports};

use crate::cosmwasm::encoding::Binary;
use crate::cosmwasm::query::{QueryRequest, WasmQuery};
use crate::cosmwasm::std_error::{StdError, StdResult};

use enclave_ffi_types::{Ctx, EnclaveBuffer, OcallReturn, UntrustedVmError};
use log::*;
use sgx_types::sgx_status_t;

pub fn encrypt_and_query_chain(
    query: &[u8],
    context: &Ctx,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<(Option<Vec<u8>>, u64), WasmEngineError> {
    let mut query_struct: QueryRequest = serde_json::from_slice(query).map_err(|err| {
        error!(
            "encrypt_and_query_chain() cannot build struct from json {:?}: {:?}",
            String::from_utf8_lossy(query),
            err
        );
        WasmEngineError::BadQueryChainRequest
    })?;

    query_struct = match query_struct {
        QueryRequest::Wasm(WasmQuery::Smart { contract_addr, msg }) => {
            let mut encrypted_msg = SecretMessage {
                msg: msg.0,
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
            let msg = Binary(encrypted_msg.to_slice());
            QueryRequest::Wasm(WasmQuery::Smart { contract_addr, msg })
        }
        QueryRequest::Wasm(_) => query_struct, // WasmQuery::Raw - pass it to x/compute to fail because the state is encrypted on-chain
        _ => query_struct,
    };

    let encrypted_query = serde_json::to_vec(&query_struct).map_err(|err| {
        // this should never happen
        error!(
            "encrypt_and_query_chain() cannot build json from struct {:?}: {:?}",
            query_struct, err
        );
        WasmEngineError::BadQueryChainRequest
    })?;

    // Call query_chain (this bubbles up to x/compute via ocalls and FFI to Go code)
    // This returns the value from x/compute
    match query_chain(context, &encrypted_query) {
        Ok((answer, gas_used)) => match answer {
            Some(answer_as_vec) => {
                // answer is QueryResult (Result<Binary,StdError>) encoded be serde to bytes
                // we need to:
                //  (1) deserialize it from bytes
                //  (2) decrypt the result/stderror
                //  (3) turn in back to QueryResult as bytes

                let answer: StdResult<Binary> = match serde_json::from_slice(&answer_as_vec) {
                    Err(err) => {
                        error!("encrypt_and_query_chain() got an error while trying to deserialize the answer as StdResult<Binary>: {:?}", err);
                        return Err(WasmEngineError::DeserializationError);
                    }
                    Ok(x) => x,
                };

                let decrypted_answer: StdResult<Binary> = match answer {
                    Ok(result) => {
                        // query response returns without nonce and user_public_key appended to it
                        // because the sender is supposed to have them already
                        let as_secret_msg = SecretMessage {
                            nonce,
                            user_public_key,
                            msg: result.0,
                        };

                        match as_secret_msg.decrypt() {
                            Ok(decrypted) => Ok(Binary(decrypted)),
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
                    Err(err) => {
                        let std_err: StdError = match err {
                            StdError::GenericErr { msg } => {
                                let inner_error_bytes = match base64::decode(&msg) {
                                    Ok(x) => x,
                                    Err(err) => {
                                        error!(
                                            "encrypt_and_query_chain() got an StdError as an answer, tried to decode the inner msg as bytes because it's encrypted, but got an error while trying to decode from base64: {:?}",
                                            err
                                        );
                                        return Err(WasmEngineError::DecryptionError);
                                    }
                                };

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
                                        let inner_error: StdError = match serde_json::from_slice(
                                            &decrypted,
                                        ) {
                                            Err(err) => {
                                                error!("encrypt_and_query_chain() got an error while trying to deserialize the inner error as StdError: {:?}", err);
                                                return Err(WasmEngineError::DeserializationError);
                                            }
                                            Ok(x) => x,
                                        };

                                        inner_error
                                    }
                                }
                            }
                            _ => {
                                error!(
                                    "encrypt_and_query_chain() got an StdError as an answer, but it should be of type GenericErr and encrypted inside. Got instead: {:?}",
                                    err
                                );
                                err
                            }
                        };

                        Err(std_err)
                    }
                };

                let decrypted_answer_as_vec = match serde_json::to_vec(&decrypted_answer) {
                    Err(err) => {
                        error!("encrypt_and_query_chain() got an error while trying to serialize the decrypted answer to bytes: {:?}", err);
                        return Err(WasmEngineError::SerializationError);
                    }
                    Ok(x) => x,
                };

                Ok((Some(decrypted_answer_as_vec), gas_used))
            }
            None => Ok((None, gas_used)),
        },
        Err(err) => Err(err),
    }
}

/// Safe wrapper around quering other contracts and modules
fn query_chain(context: &Ctx, query: &[u8]) -> Result<(Option<Vec<u8>>, u64), WasmEngineError> {
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
                exports::recover_buffer(enclave_buffer)
            }
            OcallReturn::Failure => {
                return Err(WasmEngineError::FailedOcall(vm_err));
            }
            OcallReturn::Panic => return Err(WasmEngineError::Panic),
        }
    };

    Ok((value, gas_used))
}
