use log::*;
use std::ffi::c_void;

use enclave_ffi_types::{
    Ctx, EnclaveBuffer, EnclaveError, HandleResult, HealthCheckResult, InitResult, QueryResult,
};
use std::panic;

use crate::results::{
    result_handle_success_to_handleresult, result_init_success_to_initresult,
    result_query_success_to_queryresult,
};
use crate::{
    oom_handler,
    utils::{validate_const_ptr, validate_mut_ptr},
};

// todo: add description
/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_allocate(buffer: *const u8, length: usize) -> EnclaveBuffer {
    oom_handler::register_oom_handler();

    if let Err(_e) = validate_const_ptr(buffer, length as usize) {
        error!("Tried to access data outside enclave memory space!");
        return EnclaveBuffer::default();
    }

    let slice = std::slice::from_raw_parts(buffer, length);
    let result = panic::catch_unwind(|| {
        let vector_copy = slice.to_vec();
        let boxed_vector = Box::new(vector_copy);
        let heap_pointer = Box::into_raw(boxed_vector);
        EnclaveBuffer {
            ptr: heap_pointer as *mut c_void,
        }
    });
    oom_handler::restore_safety_buffer();

    result.unwrap_or_else(|err| {
        // We can get here only by failing to allocate memory,
        // so there's no real need here to test if oom happened
        error!("Enclave ran out of memory: {:?}", err);
        oom_handler::get_then_clear_oom_happened();
        EnclaveBuffer::default()
    })
}

/// Take a pointer as returned by `ecall_allocate` and recover the Vec<u8> inside of it.
/// # Safety
///  This is a text
pub unsafe fn recover_buffer(ptr: EnclaveBuffer) -> Option<Vec<u8>> {
    if ptr.ptr.is_null() {
        return None;
    }
    let boxed_vector = Box::from_raw(ptr.ptr as *mut Vec<u8>);
    Some(*boxed_vector)
}

/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_init(
    context: Ctx,
    gas_limit: u64,
    used_gas: *mut u64,
    contract: *const u8,
    contract_len: usize,
    env: *const u8,
    env_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> InitResult {
    oom_handler::register_oom_handler();

    if let Err(_e) = validate_mut_ptr(used_gas as _, std::mem::size_of::<u64>()) {
        error!("Tried to access data outside enclave memory!");
        return result_init_success_to_initresult(Err(EnclaveError::FailedFunctionCall));
    }
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

    let contract = std::slice::from_raw_parts(contract, contract_len);
    let env = std::slice::from_raw_parts(env, env_len);
    let msg = std::slice::from_raw_parts(msg, msg_len);
    let result = panic::catch_unwind(|| {
        let mut local_used_gas = *used_gas;
        let result = crate::wasm::init(context, gas_limit, &mut local_used_gas, contract, env, msg);
        *used_gas = local_used_gas;
        result_init_success_to_initresult(result)
    });
    oom_handler::restore_safety_buffer();

    if let Ok(res) = result {
        res
    } else {
        *used_gas = gas_limit / 2;

        if oom_handler::get_then_clear_oom_happened() {
            error!("Call ecall_init failed because the enclave ran out of memory!");
            InitResult::Failure {
                err: EnclaveError::OutOfMemory,
            }
        } else {
            error!("Call ecall_init panic'd unexpectedly!");
            InitResult::Failure {
                err: EnclaveError::Panic,
            }
        }
    }
}

/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_handle(
    context: Ctx,
    gas_limit: u64,
    used_gas: *mut u64,
    contract: *const u8,
    contract_len: usize,
    env: *const u8,
    env_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> HandleResult {
    oom_handler::register_oom_handler();

    if let Err(_e) = validate_mut_ptr(used_gas as _, std::mem::size_of::<u64>()) {
        error!("Tried to access data outside enclave memory!");
        return result_handle_success_to_handleresult(Err(EnclaveError::FailedFunctionCall));
    }
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

    let contract = std::slice::from_raw_parts(contract, contract_len);
    let env = std::slice::from_raw_parts(env, env_len);
    let msg = std::slice::from_raw_parts(msg, msg_len);
    let result = panic::catch_unwind(|| {
        let mut local_used_gas = *used_gas;
        let result =
            crate::wasm::handle(context, gas_limit, &mut local_used_gas, contract, env, msg);
        *used_gas = local_used_gas;
        result_handle_success_to_handleresult(result)
    });
    oom_handler::restore_safety_buffer();

    if let Ok(res) = result {
        res
    } else {
        *used_gas = gas_limit / 2;

        if oom_handler::get_then_clear_oom_happened() {
            error!("Call ecall_handle failed because the enclave ran out of memory!");
            HandleResult::Failure {
                err: EnclaveError::OutOfMemory,
            }
        } else {
            error!("Call ecall_handle panic'd unexpectedly!");
            HandleResult::Failure {
                err: EnclaveError::Panic,
            }
        }
    }
}

/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_query(
    context: Ctx,
    gas_limit: u64,
    used_gas: *mut u64,
    contract: *const u8,
    contract_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> QueryResult {
    oom_handler::register_oom_handler();

    if let Err(_e) = validate_mut_ptr(used_gas as _, std::mem::size_of::<u64>()) {
        error!("Tried to access data outside enclave memory!");
        return result_query_success_to_queryresult(Err(EnclaveError::FailedFunctionCall));
    }
    if let Err(_e) = validate_const_ptr(msg, msg_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_query_success_to_queryresult(Err(EnclaveError::FailedFunctionCall));
    }
    if let Err(_e) = validate_const_ptr(contract, contract_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_query_success_to_queryresult(Err(EnclaveError::FailedFunctionCall));
    }

    let contract = std::slice::from_raw_parts(contract, contract_len);
    let msg = std::slice::from_raw_parts(msg, msg_len);
    let result = panic::catch_unwind(|| {
        let mut local_used_gas = *used_gas;
        let result = crate::wasm::query(context, gas_limit, &mut local_used_gas, contract, msg);
        *used_gas = local_used_gas;
        result_query_success_to_queryresult(result)
    });
    oom_handler::restore_safety_buffer();

    if let Ok(res) = result {
        res
    } else {
        *used_gas = gas_limit / 2;

        if oom_handler::get_then_clear_oom_happened() {
            error!("Call ecall_query failed because the enclave ran out of memory!");
            QueryResult::Failure {
                err: EnclaveError::OutOfMemory,
            }
        } else {
            error!("Call ecall_query panic'd unexpectedly!");
            QueryResult::Failure {
                err: EnclaveError::Panic,
            }
        }
    }
}

/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_health_check() -> HealthCheckResult {
    HealthCheckResult::Success
}
