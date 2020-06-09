use log::*;
use std::ffi::c_void;

use enclave_ffi_types::{Ctx, EnclaveBuffer, EnclaveError, HandleResult, InitResult, QueryResult};

use crate::results::{
    result_handle_success_to_handleresult, result_init_success_to_initresult,
    result_query_success_to_queryresult,
};
use crate::utils::validate_const_ptr;

#[no_mangle]
pub extern "C" fn ecall_allocate(buffer: *const u8, length: usize) -> EnclaveBuffer {
    if let Err(_e) = validate_const_ptr(buffer, length as usize) {
        panic!("Tried to access data outside enclave memory!");
    }

    let slice = unsafe { std::slice::from_raw_parts(buffer, length) };
    let vector_copy = slice.to_vec();
    let boxed_vector = Box::new(vector_copy);
    let heap_pointer = Box::into_raw(boxed_vector);
    EnclaveBuffer {
        ptr: heap_pointer as *mut c_void,
    }
}

/// Take a pointer as returned by `ecall_allocate` and recover the Vec<u8> inside of it.
pub unsafe fn recover_buffer(ptr: EnclaveBuffer) -> Option<Vec<u8>> {
    if ptr.ptr.is_null() {
        return None;
    }
    let boxed_vector = Box::from_raw(ptr.ptr as *mut Vec<u8>);
    Some(*boxed_vector)
}

#[no_mangle]
pub extern "C" fn ecall_init(
    context: Ctx,
    gas_limit: u64,
    contract: *const u8,
    contract_len: usize,
    env: *const u8,
    env_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> InitResult {
    if let Err(_e) = validate_const_ptr(env, env_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_init_success_to_initresult(Err(EnclaveError::FailedFunctionCall));
    }
    if let Err(_e) = validate_const_ptr(msg, msg_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_init_success_to_initresult(Err(EnclaveError::FailedFunctionCall));
    }
    if let Err(_e) = validate_const_ptr(contract, contract_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_init_success_to_initresult(Err(EnclaveError::FailedFunctionCall));
    }

    let contract = unsafe { std::slice::from_raw_parts(contract, contract_len) };
    let env = unsafe { std::slice::from_raw_parts(env, env_len) };
    let msg = unsafe { std::slice::from_raw_parts(msg, msg_len) };

    let result = crate::wasm::init(context, gas_limit, contract, env, msg);
    result_init_success_to_initresult(result)
}

#[no_mangle]
pub extern "C" fn ecall_handle(
    context: Ctx,
    gas_limit: u64,
    contract: *const u8,
    contract_len: usize,
    env: *const u8,
    env_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> HandleResult {
    if let Err(_e) = validate_const_ptr(env, env_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_handle_success_to_handleresult(Err(EnclaveError::FailedFunctionCall));
    }
    if let Err(_e) = validate_const_ptr(msg, msg_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_handle_success_to_handleresult(Err(EnclaveError::FailedFunctionCall));
    }
    if let Err(_e) = validate_const_ptr(contract, contract_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_handle_success_to_handleresult(Err(EnclaveError::FailedFunctionCall));
    }

    let contract = unsafe { std::slice::from_raw_parts(contract, contract_len) };
    let env = unsafe { std::slice::from_raw_parts(env, env_len) };
    let msg = unsafe { std::slice::from_raw_parts(msg, msg_len) };

    let result = crate::wasm::handle(context, gas_limit, contract, env, msg);
    result_handle_success_to_handleresult(result)
}

#[no_mangle]
pub extern "C" fn ecall_query(
    context: Ctx,
    gas_limit: u64,
    contract: *const u8,
    contract_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> QueryResult {
    if let Err(_e) = validate_const_ptr(msg, msg_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_query_success_to_queryresult(Err(EnclaveError::FailedFunctionCall));
    }
    if let Err(_e) = validate_const_ptr(contract, contract_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_query_success_to_queryresult(Err(EnclaveError::FailedFunctionCall));
    }

    let contract = unsafe { std::slice::from_raw_parts(contract, contract_len) };
    let msg = unsafe { std::slice::from_raw_parts(msg, msg_len) };

    let result = crate::wasm::query(context, gas_limit, contract, msg);
    result_query_success_to_queryresult(result)
}
