use log::*;

use sgx_types::sgx_status_t;

use enclave_ffi_types::{Ctx, EnclaveBuffer, OcallReturn, UntrustedVmError};

use enclave_crypto::Ed25519PublicKey;
use enclave_utils::recursion_depth;

use super::errors::WasmEngineError;
use crate::external::{ecalls, ocalls};
use crate::types::{IoNonce, SecretMessage};

use enclave_cosmwasm_types::{
    encoding::Binary,
    query::{QueryRequest, WasmQuery},
    std_error::{StdError, StdResult},
    system_error::{SystemError, SystemResult},
};

pub fn encrypt_and_query_chain(
    query: &[u8],
    context: &Ctx,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
    gas_used: &mut u64,
    gas_limit: u64,
) -> Result<Vec<u8>, WasmEngineError> {
    if let Some(answer) = check_recursion_limit() {
        return serialize_error_response(&answer);
    }

    let mut query_struct: QueryRequest = match serde_json::from_slice(query) {
        Ok(query_struct) => query_struct,
        Err(err) => {
            *gas_used = 500; // Should we charge gas for this to prevent spam?
            return system_error_invalid_request(query, err);
        }
    };

    let is_encrypted = encrypt_query_request(&mut query_struct, nonce, user_public_key)?;

    let encrypted_query = serde_json::to_vec(&query_struct).map_err(|err| {
        // this should never happen
        debug!(
            "encrypt_and_query_chain() got an error while trying to serialize the query {:?} to pass to x/compute: {:?}",
            query_struct,
            err
        );
        WasmEngineError::SerializationError
    })?;

    // Call query_chain (this bubbles up to x/compute via ocalls and FFI to Go code)
    // This returns the answer from x/compute
    let (result, query_used_gas) = query_chain(context, &encrypted_query, gas_limit);
    *gas_used = query_used_gas;
    let encrypted_answer_as_vec = result?;

    if !is_encrypted {
        return Ok(encrypted_answer_as_vec);
    }

    // answer is QueryResult (Result<Result<Binary,StdError>,SystemError>) encoded by serde to bytes.
    // we need to:
    //  (1) deserialize it from bytes
    //  (2) decrypt the Result/StdError
    //  (3) turn in back to QueryResult as bytes
    let parse_result = serde_json::from_slice(&encrypted_answer_as_vec);
    let encrypted_answer: SystemResult<StdResult<Binary>> = match parse_result {
        Ok(encrypted_answer) => encrypted_answer,
        Err(err) => {
            return system_error_invalid_response(encrypted_answer_as_vec, err);
        }
    };

    debug!(
        "encrypt_and_query_chain() got encrypted answer with gas {}: {:?}",
        gas_used, encrypted_answer
    );

    // decrypt query response
    let answer: SystemResult<StdResult<Binary>> = match encrypted_answer {
        Err(_) => encrypted_answer,
        // normal response from contract
        Ok(Ok(result)) => {
            let decrypted = decrypt_query_response(query, result.0, nonce, user_public_key)?;
            Ok(Ok(Binary(decrypted)))
        }
        // error response from contract, or critical error in called VM
        Ok(Err(StdError::GenericErr { msg })) => {
            let error_prefix = "encrypted: ";
            let error_suffix = ": query contract failed";
            if !(msg.starts_with(error_prefix) && msg.ends_with(error_suffix)) {
                Ok(Err(StdError::GenericErr { msg }))
            } else {
                let msg = &msg[error_prefix.len()..(msg.len() - error_suffix.len())];
                match base64::decode(msg) {
                    Err(err) => {
                        debug!(
                            "encrypt_and_query_chain() got an StdError as an answer {:?}, tried to decode \
                            the inner msg as bytes because it's encrypted, but got an error while trying to \
                            decode from base64: {}",
                            msg, err
                        );
                        return Err(WasmEngineError::DeserializationError);
                    }
                    Ok(error) => {
                        let decrypted =
                            decrypt_query_response_error(query, error, nonce, user_public_key)?;
                        match serde_json::from_slice::<StdError>(&decrypted) {
                            Ok(answer) => Ok(Err(answer)),
                            Err(err) => {
                                debug!("encrypt_and_query_chain() got an error while trying to deserialize the inner error as StdError: {:?}", err);
                                return system_error_invalid_response(decrypted, err);
                            }
                        }
                    }
                }
            }
        }
        Ok(Err(std_error)) => {
            // TODO when removing this you can replace this clause with: `other => other` instead of the `Ok(Err(...))`
            debug!(
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
        debug!("encrypt_and_query_chain() got an error while trying to serialize the decrypted answer to bytes: {:?}", err);
        WasmEngineError::SerializationError
    })?;

    Ok(answer_as_vec)
}

/// Safe wrapper around quering other contracts and modules
fn query_chain(
    context: &Ctx,
    query: &[u8],
    gas_limit: u64,
) -> (Result<Vec<u8>, WasmEngineError>, u64) {
    let mut ocall_return = OcallReturn::Success;
    let mut enclave_buffer = std::mem::MaybeUninit::<EnclaveBuffer>::uninit();
    let mut vm_err = UntrustedVmError::default();
    let mut gas_used = 0_u64;
    let value = unsafe {
        let status = ocalls::ocall_query_chain(
            &mut ocall_return,
            context.unsafe_clone(),
            &mut vm_err,
            &mut gas_used,
            gas_limit,
            enclave_buffer.as_mut_ptr(),
            query.as_ptr(),
            query.len(),
        );

        trace!("ocall_query_chain returned with gas {}", gas_used);

        match status {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            error_status => {
                warn!(
                    "query_chain() got an error from ocall_query_chain, stopping wasm: {:?}",
                    error_status
                );
                return (Err(WasmEngineError::FailedOcall(vm_err)), gas_used);
            }
        }

        match ocall_return {
            OcallReturn::Success => {
                let enclave_buffer = enclave_buffer.assume_init();
                match ecalls::recover_buffer(enclave_buffer) {
                    Ok(buff) => buff.unwrap_or_default(),
                    Err(err) => return (Err(err.into()), gas_used),
                }
            }
            OcallReturn::Failure => return (Err(WasmEngineError::FailedOcall(vm_err)), gas_used),
            OcallReturn::Panic => return (Err(WasmEngineError::Panic), gas_used),
        }
    };

    (Ok(value), gas_used)
}

/// Check whether the query is allowed to run.
///
/// We make sure that a recursion limit is in place in order to
/// mitigate cases where the enclave runs out of memory.
fn check_recursion_limit() -> Option<SystemResult<StdResult<Binary>>> {
    if recursion_depth::limit_reached() {
        debug!(
            "Recursion limit reached while performing nested queries. Returning error to contract."
        );
        Some(Err(SystemError::ExceededRecursionLimit {}))
    } else {
        None
    }
}

fn system_error_invalid_request<T>(request: &[u8], err: T) -> Result<Vec<u8>, WasmEngineError>
where
    T: std::fmt::Debug + ToString,
{
    debug!(
        "encrypt_and_query_chain() cannot build struct from json {:?}: {:?}",
        String::from_utf8_lossy(request),
        err
    );
    let answer: SystemResult<StdResult<Binary>> = Err(SystemError::InvalidRequest {
        request: Binary(request.into()),
        error: err.to_string(),
    });

    serialize_error_response(&answer)
}

fn system_error_invalid_response<T>(response: Vec<u8>, err: T) -> Result<Vec<u8>, WasmEngineError>
where
    T: std::fmt::Debug + ToString,
{
    let answer: SystemResult<StdResult<Binary>> = Err(SystemError::InvalidResponse {
        response: Binary(response),
        error: err.to_string(),
    });

    serialize_error_response(&answer)
}

fn serialize_error_response(
    answer: &SystemResult<StdResult<Binary>>,
) -> Result<Vec<u8>, WasmEngineError> {
    serde_json::to_vec(answer).map_err(|err| {
        // this should never happen
        debug!(
            "encrypt_and_query_chain() got an error while trying to serialize the error {:?} returned to WASM: {:?}",
            answer,
            err
        );

        WasmEngineError::SerializationError
    })
}

fn encrypt_query_request(
    query_struct: &mut QueryRequest,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<bool, WasmEngineError> {
    let mut is_encrypted = false;

    // encrypt message
    if let QueryRequest::Wasm(WasmQuery::Smart {
        msg,
        callback_code_hash,
        ..
    }) = query_struct
    {
        is_encrypted = true;

        let mut hash_appended_msg = callback_code_hash.clone().into_bytes();
        hash_appended_msg.extend_from_slice(&msg.0);

        let mut encrypted_msg = SecretMessage {
            msg: hash_appended_msg,
            user_public_key,
            nonce,
        };
        encrypted_msg.encrypt_in_place().map_err(|err| {
            debug!(
                "encrypt_and_query_chain() got an error while trying to encrypt the request for query {:?}, stopping wasm: {:?}",
                String::from_utf8_lossy(&msg.0),
                err
            );

            WasmEngineError::EncryptionError
        })?;

        *msg = Binary(encrypted_msg.to_vec());
    };

    Ok(is_encrypted)
}

fn decrypt_query_response(
    query: &[u8],
    response: Vec<u8>,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<Vec<u8>, WasmEngineError> {
    // query response returns without nonce and user_public_key appended to it
    // because the sender is supposed to have them already
    let as_secret_msg = SecretMessage {
        nonce,
        user_public_key,
        msg: response,
    };

    let b64_decrypted = as_secret_msg.decrypt().map_err(|err| {
        debug!(
            "encrypt_and_query_chain() got an error while trying to decrypt the result for query {:?}, stopping wasm: {:?}",
            String::from_utf8_lossy(query),
            err
        );
        WasmEngineError::DecryptionError
    })?;

    base64::decode(&b64_decrypted).map_err(|err| {
        debug!(
            "encrypt_and_query_chain() got an answer, managed to decrypt it, then tried to decode the output from base64 to bytes and failed: {:?}",
            err
        );
        WasmEngineError::DeserializationError
    })
}

fn decrypt_query_response_error(
    query: &[u8],
    error: Vec<u8>,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<Vec<u8>, WasmEngineError> {
    let error_msg = SecretMessage {
        nonce,
        user_public_key,
        msg: error,
    };

    error_msg.decrypt().map_err(|err| {
        debug!(
            "encrypt_and_query_chain() got an error while trying to decrypt the inner error for query {:?}, stopping wasm: {:?}",
            String::from_utf8_lossy(&query),
            err
        );
        WasmEngineError::DecryptionError
    })
}
