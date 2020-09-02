use lazy_static::lazy_static;
use log::*;
use std::ffi::c_void;

use enclave_ffi_types::{
    Ctx, EnclaveBuffer, EnclaveError, HandleResult, HealthCheckResult, InitResult, QueryResult,
};
use std::panic;
use std::sync::SgxMutex;

use crate::results::{
    result_handle_success_to_handleresult, result_init_success_to_initresult,
    result_query_success_to_queryresult,
};
use crate::{
    oom_handler, recursion_depth,
    utils::{validate_const_ptr, validate_mut_ptr},
};

lazy_static! {
    static ref ECALL_ALLOCATE_STACK: SgxMutex<Vec<EnclaveBuffer>> = SgxMutex::new(Vec::new());
}

/// Allocate a buffer in the enclave and return a pointer to it. This is useful for ocalls that
/// want to return a response of unknown length to the enclave. Instead of pre-allocating it on the
/// ecall side, the ocall can call this ecall and return the EnclaveBuffer to the ecall that called
/// it.
///
/// host -> ecall_x -> ocall_x -> ecall_allocate
/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_allocate(buffer: *const u8, length: usize) -> EnclaveBuffer {
    if let Err(_err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return EnclaveBuffer::default();
    }

    if let Err(_e) = validate_const_ptr(buffer, length as usize) {
        error!("Tried to access data outside enclave memory space!");
        return EnclaveBuffer::default();
    }

    let slice = std::slice::from_raw_parts(buffer, length);
    let result = panic::catch_unwind(|| {
        let vector_copy = slice.to_vec();
        let boxed_vector = Box::new(vector_copy);
        let heap_pointer = Box::into_raw(boxed_vector);
        let enclave_buffer = EnclaveBuffer {
            ptr: heap_pointer as *mut c_void,
        };
        ECALL_ALLOCATE_STACK
            .lock()
            .unwrap()
            .push(enclave_buffer.unsafe_clone());
        enclave_buffer
    });

    if let Err(_err) = oom_handler::restore_safety_buffer() {
        error!("Could not restore OOM safety buffer!");
        return EnclaveBuffer::default();
    }

    result.unwrap_or_else(|err| {
        // We can get here only by failing to allocate memory,
        // so there's no real need here to test if oom happened
        error!("Enclave ran out of memory: {:?}", err);
        oom_handler::get_then_clear_oom_happened();
        EnclaveBuffer::default()
    })
}

#[derive(Debug, PartialEq)]
pub struct BufferRecoveryError;

/// Take a pointer as returned by `ecall_allocate` and recover the Vec<u8> inside of it.
/// # Safety
///  This is a text
pub unsafe fn recover_buffer(ptr: EnclaveBuffer) -> Result<Option<Vec<u8>>, BufferRecoveryError> {
    if ptr.ptr.is_null() {
        return Ok(None);
    }

    let mut alloc_stack = ECALL_ALLOCATE_STACK.lock().unwrap();

    // search the stack from the end for this pointer
    let maybe_index = alloc_stack
        .iter()
        .rev()
        .position(|buffer| buffer.ptr as usize == ptr.ptr as usize);
    if let Some(index_from_the_end) = maybe_index {
        // This index is probably at the end of the stack, but we give it a little more flexibility
        // in case access patterns change in the future
        let index = alloc_stack.len() - index_from_the_end - 1;
        alloc_stack.swap_remove(index);
    } else {
        return Err(BufferRecoveryError);
    }
    let boxed_vector = Box::from_raw(ptr.ptr as *mut Vec<u8>);
    Ok(Some(*boxed_vector))
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
    sig_info: *const u8,
    sig_info_len: usize,
) -> InitResult {
    let _recursion_guard = match recursion_depth::guard() {
        Ok(rg) => rg,
        Err(err) => {
            // https://github.com/enigmampc/SecretNetwork/pull/517#discussion_r481924571
            // I believe that this error condition is currently unreachable.
            // I think we can safely remove it completely right now, and have
            // recursion_depth::increment() simply increment the counter with no further checks,
            // but i wanted to stay on the safe side here, in case something changes in the
            // future, and we can easily spot that we forgot to add a limit somewhere.
            error!("recursion limit exceeded, can not perform init!");
            return InitResult::Failure { err };
        }
    };
    if let Err(err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return InitResult::Failure { err };
    }
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
    if let Err(_e) = validate_const_ptr(sig_info, sig_info_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_init_success_to_initresult(Err(EnclaveError::FailedFunctionCall));
    }

    let contract = std::slice::from_raw_parts(contract, contract_len);
    let env = std::slice::from_raw_parts(env, env_len);
    let msg = std::slice::from_raw_parts(msg, msg_len);
    let sig_info = std::slice::from_raw_parts(sig_info, sig_info_len);
    let result = panic::catch_unwind(|| {
        let mut local_used_gas = *used_gas;
        let result = crate::wasm::init(
            context,
            gas_limit,
            &mut local_used_gas,
            contract,
            env,
            msg,
            sig_info,
        );
        *used_gas = local_used_gas;
        result_init_success_to_initresult(result)
    });

    if let Err(err) = oom_handler::restore_safety_buffer() {
        error!("Could not restore OOM safety buffer!");
        return InitResult::Failure { err };
    }

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
            error!("Call ecall_init panicked unexpectedly!");
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
    sig_info: *const u8,
    sig_info_len: usize,
) -> HandleResult {
    let _recursion_guard = match recursion_depth::guard() {
        Ok(rg) => rg,
        Err(err) => {
            // https://github.com/enigmampc/SecretNetwork/pull/517#discussion_r481924571
            // I believe that this error condition is currently unreachable.
            // I think we can safely remove it completely right now, and have
            // recursion_depth::increment() simply increment the counter with no further checks,
            // but i wanted to stay on the safe side here, in case something changes in the
            // future, and we can easily spot that we forgot to add a limit somewhere.
            error!("recursion limit exceeded, can not perform handle!");
            return HandleResult::Failure { err };
        }
    };
    if let Err(err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return HandleResult::Failure { err };
    }
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
    if let Err(_e) = validate_const_ptr(sig_info, sig_info_len as usize) {
        error!("Tried to access data outside enclave memory!");
        return result_handle_success_to_handleresult(Err(EnclaveError::FailedFunctionCall));
    }

    let contract = std::slice::from_raw_parts(contract, contract_len);
    let env = std::slice::from_raw_parts(env, env_len);
    let msg = std::slice::from_raw_parts(msg, msg_len);
    let sig_info = std::slice::from_raw_parts(sig_info, sig_info_len);
    let result = panic::catch_unwind(|| {
        let mut local_used_gas = *used_gas;
        let result = crate::wasm::handle(
            context,
            gas_limit,
            &mut local_used_gas,
            contract,
            env,
            msg,
            sig_info,
        );
        *used_gas = local_used_gas;
        result_handle_success_to_handleresult(result)
    });

    if let Err(err) = oom_handler::restore_safety_buffer() {
        error!("Could not restore OOM safety buffer!");
        return HandleResult::Failure { err };
    }

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
            error!("Call ecall_handle panicked unexpectedly!");
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
    let _recursion_guard = match recursion_depth::guard() {
        Ok(rg) => rg,
        Err(err) => {
            // https://github.com/enigmampc/SecretNetwork/pull/517#discussion_r481924571
            // I believe that this error condition is currently unreachable.
            // I think we can safely remove it completely right now, and have
            // recursion_depth::increment() simply increment the counter with no further checks,
            // but i wanted to stay on the safe side here, in case something changes in the
            // future, and we can easily spot that we forgot to add a limit somewhere.
            error!("recursion limit exceeded, can not perform query!");
            return QueryResult::Failure { err };
        }
    };
    if let Err(err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return QueryResult::Failure { err };
    }
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

    if let Err(err) = oom_handler::restore_safety_buffer() {
        error!("Could not restore OOM safety buffer!");
        return QueryResult::Failure { err };
    }

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
            error!("Call ecall_query panicked unexpectedly!");
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

#[cfg(feature = "test")]
pub mod tests {
    use super::*;
    use crate::count_failures;

    pub fn run_tests() {
        println!();
        let mut failures = 0;

        count_failures!(failures, {
            test_recover_enclave_buffer_valid();
            test_recover_enclave_buffer_invalid();
            test_recover_enclave_buffer_invalid_but_similar();
            test_recover_enclave_buffer_invalid_null();
            test_recover_enclave_buffer_in_recursion_valid();
            test_recover_enclave_buffer_in_recursion_invalid();
            test_recover_enclave_buffer_multiple_out_of_order_valid();
            test_recover_enclave_buffer_multiple_out_of_order_invalid();
        });

        if failures != 0 {
            println!("{}: {} tests failed", file!(), failures);
            panic!()
        }
    }

    fn test_recover_enclave_buffer_valid() {
        let message = b"some example text";
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0);
        let enclave_buffer = unsafe { ecall_allocate(message.as_ptr(), message.len()) };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 1);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0);
        assert_eq!(recovered.unwrap().unwrap(), message);
    }

    fn test_recover_enclave_buffer_invalid() {
        let enclave_buffer = EnclaveBuffer {
            ptr: 0x12345678_usize as _,
        };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0);
        assert_eq!(recovered.unwrap_err(), BufferRecoveryError);
    }

    fn test_recover_enclave_buffer_invalid_but_similar() {
        let message = Box::new(Vec::<u8>::from(&b"some example text"[..]));
        let enclave_buffer = EnclaveBuffer {
            ptr: message.as_ptr() as _,
        };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0);
        assert_eq!(recovered.unwrap_err(), BufferRecoveryError);
    }

    fn test_recover_enclave_buffer_invalid_null() {
        let enclave_buffer = EnclaveBuffer {
            ptr: std::ptr::null_mut(),
        };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0);
        assert_eq!(recovered.unwrap(), None);
    }

    fn test_recover_enclave_buffer_in_recursion_valid() {
        let recursion_depth = 10;
        let messages: Vec<String> = (0..recursion_depth)
            .map(|num| format!("message-{}", num))
            .collect();

        // simulate building up the stack recursively
        let enclave_buffers: Vec<EnclaveBuffer> = messages
            .iter()
            .enumerate()
            .map(|(index, message)| {
                assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), index);
                unsafe { ecall_allocate(message.as_ptr(), message.len()) }
            })
            .collect();

        // simulate clearing the stack recursively
        for (index, (message, enclave_buffer)) in messages
            .into_iter()
            .zip(enclave_buffers.into_iter())
            .enumerate()
            .rev()
        {
            assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), index + 1);
            let recovered = unsafe { recover_buffer(enclave_buffer) };
            assert_eq!(recovered.unwrap().unwrap(), message.as_bytes())
        }
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0)
    }

    // This test is very similar to the test above, except it tries to give incorrect
    // inputs in the deepest part of the recursion
    fn test_recover_enclave_buffer_in_recursion_invalid() {
        let recursion_depth = 10;
        let messages: Vec<String> = (0..recursion_depth)
            .map(|num| format!("message-{}", num))
            .collect();

        // simulate building up the stack recursively
        let enclave_buffers: Vec<EnclaveBuffer> = messages
            .iter()
            .enumerate()
            .map(|(index, message)| {
                assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), index);
                unsafe { ecall_allocate(message.as_ptr(), message.len()) }
            })
            .collect();

        let message = Box::new(Vec::<u8>::from(&b"some example text"[..]));
        let enclave_buffer = EnclaveBuffer {
            ptr: message.as_ptr() as _,
        };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), recursion_depth);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), recursion_depth);
        assert_eq!(recovered.unwrap_err(), BufferRecoveryError);

        // simulate clearing the stack recursively
        for (index, (message, enclave_buffer)) in messages
            .into_iter()
            .zip(enclave_buffers.into_iter())
            .enumerate()
            .rev()
        {
            assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), index + 1);
            let recovered = unsafe { recover_buffer(enclave_buffer) };
            assert_eq!(recovered.unwrap().unwrap(), message.as_bytes())
        }
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0)
    }

    // These tests are vry similar to the recursion tests,
    // except that they don't release the buffers in a FIFO order.
    // In this case, I just release them in a FILO order, which should be worst-case.

    fn test_recover_enclave_buffer_multiple_out_of_order_valid() {
        let recursion_depth = 10;
        let messages: Vec<String> = (0..recursion_depth)
            .map(|num| format!("message-{}", num))
            .collect();

        // simulate building up the stack recursively
        let enclave_buffers: Vec<EnclaveBuffer> = messages
            .iter()
            .enumerate()
            .map(|(index, message)| {
                assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), index);
                unsafe { ecall_allocate(message.as_ptr(), message.len()) }
            })
            .collect();

        // simulate clearing the stack recursively
        // `.rev().enumerate().rev()` means that we'll be iterating over the lists in order, with reversed indexes.
        for (index, (message, enclave_buffer)) in messages
            .into_iter()
            .zip(enclave_buffers.into_iter())
            .rev()
            .enumerate()
            .rev()
        {
            assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), index + 1);
            let recovered = unsafe { recover_buffer(enclave_buffer) };
            assert_eq!(recovered.unwrap().unwrap(), message.as_bytes())
        }
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0)
    }

    fn test_recover_enclave_buffer_multiple_out_of_order_invalid() {
        let recursion_depth = 10;
        let messages: Vec<String> = (0..recursion_depth)
            .map(|num| format!("message-{}", num))
            .collect();

        // simulate building up the stack recursively
        let enclave_buffers: Vec<EnclaveBuffer> = messages
            .iter()
            .enumerate()
            .map(|(index, message)| {
                assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), index);
                unsafe { ecall_allocate(message.as_ptr(), message.len()) }
            })
            .collect();

        let message = Box::new(Vec::<u8>::from(&b"some example text"[..]));
        let enclave_buffer = EnclaveBuffer {
            ptr: message.as_ptr() as _,
        };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), recursion_depth);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), recursion_depth);
        assert_eq!(recovered.unwrap_err(), BufferRecoveryError);

        // simulate clearing the stack recursively
        // `.rev().enumerate().rev()` means that we'll be iterating over the lists in order, with reversed indexes.
        for (index, (message, enclave_buffer)) in messages
            .into_iter()
            .zip(enclave_buffers.into_iter())
            .rev()
            .enumerate()
            .rev()
        {
            assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), index + 1);
            let recovered = unsafe { recover_buffer(enclave_buffer) };
            assert_eq!(recovered.unwrap().unwrap(), message.as_bytes())
        }
        assert_eq!(ECALL_ALLOCATE_STACK.lock().unwrap().len(), 0)
    }
}
