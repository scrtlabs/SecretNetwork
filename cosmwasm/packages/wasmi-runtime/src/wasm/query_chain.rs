use super::errors::WasmEngineError;
use crate::crypto::Ed25519PublicKey;
use crate::wasm::types::{IoNonce, SecretMessage};
use crate::{exports, imports};

use crate::cosmwasm::encoding::Binary;
use crate::cosmwasm::query::{QueryRequest, WasmQuery};

use enclave_ffi_types::{Ctx, EnclaveBuffer, OcallReturn, UntrustedVmError};
use log::*;
use sgx_types::sgx_status_t;

pub fn encrypt_and_query_chain(
    query: &[u8],
    context: &Ctx,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<(Option<Vec<u8>>, u64), WasmEngineError> {
    // TODO encrypt query
    let mut query_struct: QueryRequest = serde_json::from_slice(query).map_err(|err| {
        error!(
            "encrypt_and_query_chain() cannot parse struct from json {:?}: {:?}",
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
                    "query_chain() got an error while trying to encrypt the request for query {:?}, stopping wasm: {:?}",
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
            "encrypt_and_query_chain() cannot parse json from struct {:?}: {:?}",
            query_struct, err
        );
        WasmEngineError::BadQueryChainRequest
    })?;

    // Call read_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
    // This returns the value from Tendermint
    match query_chain(context, &encrypted_query) {
        Ok((response, gas_used)) => match response {
            Some(response) => {
                // query response returns without nonce and user_public_key appended to it
                // because the sender is supposed to have them already
                let response_as_secret_msg = SecretMessage {
                    nonce,
                    user_public_key,
                    msg: response,
                };

                match response_as_secret_msg.decrypt() {
                    Ok(decrypted) => Ok((Some(decrypted), gas_used)),
                    // This error case is why we have all the matches here.
                    // If we successfully collected a value, but failed to decrypt it, then we propagate that error.
                    Err(err) => {
                        error!(
                            "query_chain() got an error while trying to decrypt the response for query {:?}, stopping wasm: {:?}",
                            String::from_utf8_lossy(&query),
                            err
                        );

                        Err(WasmEngineError::DecryptionError)
                    }
                }
            }
            None => Ok((None, gas_used)),
        },
        Err(err) => Err(err),
    }
}

/// Safe wrapper around reads from the contract storage
fn query_chain(context: &Ctx, query: &[u8]) -> Result<(Option<Vec<u8>>, u64), WasmEngineError> {
    let mut ocall_return = OcallReturn::Success;
    let mut enclave_buffer = std::mem::MaybeUninit::<EnclaveBuffer>::uninit();
    let mut vm_err = UntrustedVmError::default();
    let mut gas_used = 0_u64;
    let value = unsafe {
        let status = imports::ocall_read_db(
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
