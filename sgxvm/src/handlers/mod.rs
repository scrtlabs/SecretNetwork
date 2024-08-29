use std::vec::Vec;
use std::slice;
use sgx_types::sgx_status_t;
use protobuf::Message;

use crate::protobuf_generated::ffi::{FFIRequest, FFIRequest_oneof_req};
use crate::{AllocationWithResult, Allocation};
use crate::ocall;
use crate::key_manager::KeyManager;
use crate::protobuf_generated::ffi::{
    NodePublicKeyResponse,
    SGXVMCallRequest, 
    SGXVMCreateRequest,
};
use crate::GoQuerier;

pub mod tx;

/// Handles incoming protobuf-encoded request
pub fn handle_protobuf_request_inner(
    querier: *mut GoQuerier,
    request_data: *const u8,
    len: usize,
) -> AllocationWithResult {
    let request_slice = unsafe { slice::from_raw_parts(request_data, len) };

    let ffi_request = match protobuf::parse_from_bytes::<FFIRequest>(request_slice) {
        Ok(ffi_request) => ffi_request,
        Err(err) => {
            println!("Got error during protobuf decoding: {:?}", err);
            return AllocationWithResult::default();
        }
    };

    match ffi_request.req {
        Some(req) => {
            match req {
                FFIRequest_oneof_req::callRequest(data) => {
                    handle_evm_call_request(querier, data)
                },
                FFIRequest_oneof_req::createRequest(data) => {
                    handle_evm_create_request(querier, data)
                },
                FFIRequest_oneof_req::publicKeyRequest(data) => {
                    handle_public_key_request(data.blockNumber)
                }
            }
        }
        None => {
            println!("Got empty request during protobuf decoding");
            AllocationWithResult::default()
        }
    }
}

/// Allocates provided data outside of enclave
/// * data - bytes to allocate outside of enclave
/// 
/// Returns allocation result with pointer to allocated memory, length of allocated data and status of allocation
pub fn allocate_inner(data: Vec<u8>) -> AllocationWithResult {
    let mut ocall_result = std::mem::MaybeUninit::<Allocation>::uninit();
    let sgx_result = unsafe { 
        ocall::ocall_allocate(
            ocall_result.as_mut_ptr(),
            data.as_ptr(),
            data.len()
        ) 
    };
    match sgx_result {
        sgx_status_t::SGX_SUCCESS => {
            let ocall_result = unsafe { ocall_result.assume_init() };
            AllocationWithResult {
                result_ptr: ocall_result.result_ptr,
                result_len: data.len(),
                status: sgx_status_t::SGX_SUCCESS
            }
        },
        _ => {
            println!("ocall_allocate failed: {:?}", sgx_result.as_str());
            AllocationWithResult::default()
        }
    }
}

/// Handles incoming request for node public key, which can be used
/// to derive shared encryption key to encrypt transaction data or 
/// decrypt node response
pub fn handle_public_key_request(block_number: u64) -> AllocationWithResult {
    let key_manager = match KeyManager::unseal() {
        Ok(manager) => manager,
        Err(_) => {
            println!("Cannot unseal key manager");
            return AllocationWithResult::default()
        }
    };

    let public_key = match key_manager.get_public_key(block_number) {
        Ok(public_key) => public_key,
        Err(_) => {
            println!("Cannot find key in Epoch Manager");
            return AllocationWithResult::default()
        }
    };

    let mut response = NodePublicKeyResponse::new();
    response.set_publicKey(public_key);

    let encoded_response = match response.write_to_bytes() {
        Ok(res) => res,
        Err(err) => {
            println!("Cannot encode protobuf result. Reason: {:?}", err);
            return AllocationWithResult::default();
        }
    };
    
    allocate_inner(encoded_response)
}

/// Handles incoming request for calling contract or transferring value
/// * querier - GoQuerier which is used to interact with Go (Cosmos) from SGX Enclave
/// * data - EVM call data (destination, value, etc.)
pub fn handle_evm_call_request(querier: *mut GoQuerier, data: SGXVMCallRequest) -> AllocationWithResult {
    let res = tx::handle_call_request_inner(querier, data);
    tx::convert_and_allocate_transaction_result(res)
}

/// Handles incoming request for creation of a new contract
/// * querier - GoQuerier which is used to interact with Go (Cosmos) from SGX Enclave
/// * data - EVM call data (value, tx.data, etc.)
pub fn handle_evm_create_request(querier: *mut GoQuerier, data: SGXVMCreateRequest) -> AllocationWithResult {
    let res = tx::handle_create_request_inner(querier, data);
    tx::convert_and_allocate_transaction_result(res)
}