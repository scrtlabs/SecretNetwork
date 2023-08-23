use std::ffi::c_void;
use std::panic;
use std::sync::SgxMutex;

use lazy_static::lazy_static;
use log::*;

use sgx_types::sgx_status_t;

use enclave_ffi_types::{
    Ctx, EnclaveBuffer, EnclaveError, HandleResult, HealthCheckResult, InitResult, MigrateResult,
    QueryResult, RuntimeConfiguration, UpdateAdminResult,
};

use enclave_utils::{oom_handler, validate_const_ptr, validate_input_length, validate_mut_ptr};

use crate::external::results::{
    result_handle_success_to_handleresult, result_init_success_to_initresult,
    result_migrate_success_to_result, result_query_success_to_queryresult,
    result_update_admin_success_to_result,
};

lazy_static! {
    static ref ECALL_ALLOCATE_STACK: SgxMutex<Vec<EnclaveBuffer>> = SgxMutex::new(Vec::new());
}

const MAX_ENV_LENGTH: usize = 10_240; // 10 KiB
const MAX_SIG_INFO_LENGTH: usize = 5_120_000; // 5 MiB, includes tx_bytes and sign_bytes
const MAX_MSG_LENGTH: usize = 2_048_000; // 2 MiB
const MAX_ADDRESS_LENGTH: usize = 65; // canonical can be 20 or 32 bytes, humanized can be 45 or 65
const MAX_PROOF_LENGTH: usize = 32; // output of sha256
const MAX_WASM_LENGHT: usize = 3_145_728; // 3 MiB, larger Wasm ATM is 1,990,361 bytes (1.6 MiB)

/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_allocate(buffer: *const u8, length: usize) -> EnclaveBuffer {
    ecall_allocate_impl(buffer, length)
}

/// Allocate a buffer in the enclave and return a pointer to it. This is useful for ocalls that
/// want to return a response of unknown length to the enclave. Instead of pre-allocating it on the
/// ecall side, the ocall can call this ecall and return the EnclaveBuffer to the ecall that called
/// it.
///
/// host -> ecall_x -> ocall_x -> ecall_allocate
/// # Safety
/// Always use protection
unsafe fn ecall_allocate_impl(buffer: *const u8, length: usize) -> EnclaveBuffer {
    if let Err(_err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return EnclaveBuffer::default();
    }

    validate_const_ptr!(buffer, length, EnclaveBuffer::default());

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

/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_configure_runtime(config: RuntimeConfiguration) -> sgx_status_t {
    ecall_configure_runtime_impl(config)
}

/// This function sets up any components of the contract runtime
/// that should be set up once when the node starts.
///
/// # Safety
/// Always use protection
#[no_mangle]
fn ecall_configure_runtime_impl(config: RuntimeConfiguration) -> sgx_status_t {
    debug!(
        "inside ecall_configure_runtime: {}",
        config.module_cache_size
    );
    crate::wasm3::module_cache::configure_module_cache(config.module_cache_size as usize);
    sgx_status_t::SGX_SUCCESS
}

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
    admin: *const u8,
    admin_len: usize,
) -> InitResult {
    if let Err(err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return InitResult::Failure { err };
    }

    let failed_call = || result_init_success_to_initresult(Err(EnclaveError::FailedFunctionCall));
    validate_mut_ptr!(used_gas as _, std::mem::size_of::<u64>(), failed_call());
    validate_const_ptr!(env, env_len, failed_call());
    validate_const_ptr!(msg, msg_len, failed_call());
    validate_const_ptr!(contract, contract_len, failed_call());
    validate_const_ptr!(sig_info, sig_info_len, failed_call());
    // admin can be null (checked later), so admin_len is allowed to be 0

    validate_input_length!(env_len, "env", MAX_ENV_LENGTH, failed_call());
    validate_input_length!(msg_len, "msg", MAX_MSG_LENGTH, failed_call());
    validate_input_length!(contract_len, "contract", MAX_WASM_LENGHT, failed_call());
    validate_input_length!(sig_info_len, "sig_info", MAX_SIG_INFO_LENGTH, failed_call());
    validate_input_length!(admin_len, "admin", MAX_ADDRESS_LENGTH, failed_call());

    let contract = std::slice::from_raw_parts(contract, contract_len);
    let env = std::slice::from_raw_parts(env, env_len);
    let msg = std::slice::from_raw_parts(msg, msg_len);
    let sig_info = std::slice::from_raw_parts(sig_info, sig_info_len);
    let admin = std::slice::from_raw_parts(admin, admin_len);
    let result = panic::catch_unwind(|| {
        let mut local_used_gas = *used_gas;
        let result = crate::contract_operations::init(
            context,
            gas_limit,
            &mut local_used_gas,
            contract,
            env,
            msg,
            sig_info,
            admin,
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
    handle_type: u8,
) -> HandleResult {
    if let Err(err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return HandleResult::Failure { err };
    }

    let failed_call =
        || result_handle_success_to_handleresult(Err(EnclaveError::FailedFunctionCall));
    validate_mut_ptr!(used_gas as _, std::mem::size_of::<u64>(), failed_call());
    validate_const_ptr!(env, env_len, failed_call());
    validate_const_ptr!(msg, msg_len, failed_call());
    validate_const_ptr!(contract, contract_len, failed_call());
    validate_const_ptr!(sig_info, sig_info_len, failed_call());

    validate_input_length!(env_len, "env", MAX_ENV_LENGTH, failed_call());
    validate_input_length!(msg_len, "msg", MAX_MSG_LENGTH, failed_call());
    validate_input_length!(contract_len, "contract", MAX_WASM_LENGHT, failed_call());
    validate_input_length!(sig_info_len, "sig_info", MAX_SIG_INFO_LENGTH, failed_call());

    let contract = std::slice::from_raw_parts(contract, contract_len);
    let env = std::slice::from_raw_parts(env, env_len);
    let msg = std::slice::from_raw_parts(msg, msg_len);
    let sig_info = std::slice::from_raw_parts(sig_info, sig_info_len);
    let result = panic::catch_unwind(|| {
        let mut local_used_gas = *used_gas;
        let result = crate::contract_operations::handle(
            context,
            gas_limit,
            &mut local_used_gas,
            contract,
            env,
            msg,
            sig_info,
            handle_type,
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
    env: *const u8,
    env_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> QueryResult {
    ecall_query_impl(
        context,
        gas_limit,
        used_gas,
        contract,
        contract_len,
        env,
        env_len,
        msg,
        msg_len,
    )
}

/// # Safety
/// Always use protection
#[allow(clippy::too_many_arguments)]
unsafe fn ecall_query_impl(
    context: Ctx,
    gas_limit: u64,
    used_gas: *mut u64,
    contract: *const u8,
    contract_len: usize,
    env: *const u8,
    env_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> QueryResult {
    if let Err(err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return QueryResult::Failure { err };
    }

    let failed_call = || result_query_success_to_queryresult(Err(EnclaveError::FailedFunctionCall));
    validate_mut_ptr!(used_gas as _, std::mem::size_of::<u64>(), failed_call());
    validate_const_ptr!(env, env_len, failed_call());
    validate_const_ptr!(msg, msg_len, failed_call());
    validate_const_ptr!(contract, contract_len, failed_call());

    validate_input_length!(env_len, "env", MAX_ENV_LENGTH, failed_call());
    validate_input_length!(msg_len, "msg", MAX_MSG_LENGTH, failed_call());
    validate_input_length!(contract_len, "contract", MAX_WASM_LENGHT, failed_call());

    let contract = std::slice::from_raw_parts(contract, contract_len);
    let env = std::slice::from_raw_parts(env, env_len);
    let msg = std::slice::from_raw_parts(msg, msg_len);
    let result = panic::catch_unwind(|| {
        let mut local_used_gas = *used_gas;
        let result = crate::contract_operations::query(
            context,
            gas_limit,
            &mut local_used_gas,
            contract,
            env,
            msg,
        );
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
pub unsafe extern "C" fn ecall_migrate(
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
    admin: *const u8,
    admin_len: usize,
    admin_proof: *const u8,
    admin_proof_len: usize,
) -> MigrateResult {
    if let Err(err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return MigrateResult::Failure { err };
    }

    let failed_call = || result_migrate_success_to_result(Err(EnclaveError::FailedFunctionCall));
    validate_mut_ptr!(used_gas as _, std::mem::size_of::<u64>(), failed_call());

    validate_const_ptr!(env, env_len, failed_call());
    validate_const_ptr!(msg, msg_len, failed_call());
    validate_const_ptr!(contract, contract_len, failed_call());
    validate_const_ptr!(sig_info, sig_info_len, failed_call());
    validate_const_ptr!(admin, admin_len, failed_call());
    validate_const_ptr!(admin_proof, admin_proof_len, failed_call());

    validate_input_length!(env_len, "env", MAX_ENV_LENGTH, failed_call());
    validate_input_length!(msg_len, "msg", MAX_MSG_LENGTH, failed_call());
    validate_input_length!(contract_len, "contract", MAX_WASM_LENGHT, failed_call());
    validate_input_length!(sig_info_len, "sig_info", MAX_SIG_INFO_LENGTH, failed_call());
    validate_input_length!(admin_len, "admin", MAX_ADDRESS_LENGTH, failed_call());
    validate_input_length!(
        admin_proof_len,
        "admin_proof",
        MAX_ENV_LENGTH,
        failed_call()
    );

    let contract = std::slice::from_raw_parts(contract, contract_len);
    let env = std::slice::from_raw_parts(env, env_len);
    let msg = std::slice::from_raw_parts(msg, msg_len);
    let sig_info = std::slice::from_raw_parts(sig_info, sig_info_len);
    let admin = std::slice::from_raw_parts(admin, admin_len);
    let admin_proof = std::slice::from_raw_parts(admin_proof, admin_proof_len);

    let result = panic::catch_unwind(|| {
        let mut local_used_gas = *used_gas;
        let result = crate::contract_operations::migrate(
            context,
            gas_limit,
            &mut local_used_gas,
            contract,
            env,
            msg,
            sig_info,
            admin,
            admin_proof,
        );
        *used_gas = local_used_gas;
        result_migrate_success_to_result(result)
    });

    if let Err(err) = oom_handler::restore_safety_buffer() {
        error!("Could not restore OOM safety buffer!");
        return MigrateResult::Failure { err };
    }

    if let Ok(res) = result {
        res
    } else {
        *used_gas = gas_limit / 2;

        if oom_handler::get_then_clear_oom_happened() {
            error!("Call ecall_migrate failed because the enclave ran out of memory!");
            MigrateResult::Failure {
                err: EnclaveError::OutOfMemory,
            }
        } else {
            error!("Call ecall_migrate panicked unexpectedly!");
            MigrateResult::Failure {
                err: EnclaveError::Panic,
            }
        }
    }
}

/// # Safety
/// Always use protection
#[no_mangle]
pub unsafe extern "C" fn ecall_update_admin(
    env: *const u8,
    env_len: usize,
    sig_info: *const u8,
    sig_info_len: usize,
    current_admin: *const u8,
    current_admin_len: usize,
    current_admin_proof: *const u8,
    current_admin_proof_len: usize,
    new_admin: *const u8,
    new_admin_len: usize,
) -> UpdateAdminResult {
    if let Err(err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return UpdateAdminResult::UpdateAdminFailure { err };
    }

    let failed_call =
        || result_update_admin_success_to_result(Err(EnclaveError::FailedFunctionCall));
    validate_const_ptr!(env, env_len, failed_call());
    validate_const_ptr!(sig_info, sig_info_len, failed_call());
    validate_const_ptr!(current_admin, current_admin_len, failed_call());
    validate_const_ptr!(
        current_admin_proof,
        current_admin_proof_len,
        failed_call()
    );
    // new_admin can be null (checked later), so new_admin_len is allowed to be 0

    validate_input_length!(env_len, "env", MAX_ENV_LENGTH, failed_call());
    validate_input_length!(sig_info_len, "sig_info", MAX_SIG_INFO_LENGTH, failed_call());
    validate_input_length!(
        current_admin_len,
        "current_admin",
        MAX_ADDRESS_LENGTH,
        failed_call()
    );
    validate_input_length!(
        current_admin_proof_len,
        "current_admin_proof",
        MAX_PROOF_LENGTH,
        failed_call()
    );

    let env = std::slice::from_raw_parts(env, env_len);
    let sig_info = std::slice::from_raw_parts(sig_info, sig_info_len);
    let current_admin = std::slice::from_raw_parts(current_admin, current_admin_len);
    let current_admin_proof =
        std::slice::from_raw_parts(current_admin_proof, current_admin_proof_len);
    let new_admin = std::slice::from_raw_parts(new_admin, new_admin_len);

    let result = panic::catch_unwind(|| {
        let result = crate::contract_operations::update_admin(
            env,
            sig_info,
            current_admin,
            current_admin_proof,
            new_admin,
        );
        result_update_admin_success_to_result(result)
    });

    if let Err(err) = oom_handler::restore_safety_buffer() {
        error!("Could not restore OOM safety buffer!");
        return UpdateAdminResult::UpdateAdminFailure { err };
    }

    if let Ok(res) = result {
        res
    } else if oom_handler::get_then_clear_oom_happened() {
        error!("Call ecall_update_admin failed because the enclave ran out of memory!");
        UpdateAdminResult::UpdateAdminFailure {
            err: EnclaveError::OutOfMemory,
        }
    } else {
        error!("Call ecall_update_admin panicked unexpectedly!");
        UpdateAdminResult::UpdateAdminFailure {
            err: EnclaveError::Panic,
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

    fn ecall_stack_length() -> usize {
        ECALL_ALLOCATE_STACK.lock().unwrap().len()
    }

    fn test_recover_enclave_buffer_valid() {
        let message = b"some example text";
        assert_eq!(ecall_stack_length(), 0);
        let enclave_buffer = unsafe { ecall_allocate(message.as_ptr(), message.len()) };
        assert_eq!(ecall_stack_length(), 1);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ecall_stack_length(), 0);
        assert_eq!(recovered.unwrap().unwrap(), message);
    }

    fn test_recover_enclave_buffer_invalid() {
        let enclave_buffer = EnclaveBuffer {
            ptr: 0x12345678_usize as _,
        };
        assert_eq!(ecall_stack_length(), 0);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ecall_stack_length(), 0);
        assert_eq!(recovered.unwrap_err(), BufferRecoveryError);
    }

    fn test_recover_enclave_buffer_invalid_but_similar() {
        let message = Box::new(Vec::<u8>::from(&b"some example text"[..]));
        let enclave_buffer = EnclaveBuffer {
            ptr: message.as_ptr() as _,
        };
        assert_eq!(ecall_stack_length(), 0);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ecall_stack_length(), 0);
        assert_eq!(recovered.unwrap_err(), BufferRecoveryError);
    }

    fn test_recover_enclave_buffer_invalid_null() {
        let enclave_buffer = EnclaveBuffer {
            ptr: std::ptr::null_mut(),
        };
        assert_eq!(ecall_stack_length(), 0);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ecall_stack_length(), 0);
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
                assert_eq!(ecall_stack_length(), index);
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
            assert_eq!(ecall_stack_length(), index + 1);
            let recovered = unsafe { recover_buffer(enclave_buffer) };
            assert_eq!(recovered.unwrap().unwrap(), message.as_bytes())
        }
        assert_eq!(ecall_stack_length(), 0)
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
                assert_eq!(ecall_stack_length(), index);
                unsafe { ecall_allocate(message.as_ptr(), message.len()) }
            })
            .collect();

        let message = Box::new(Vec::<u8>::from(&b"some example text"[..]));
        let enclave_buffer = EnclaveBuffer {
            ptr: message.as_ptr() as _,
        };
        assert_eq!(ecall_stack_length(), recursion_depth);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ecall_stack_length(), recursion_depth);
        assert_eq!(recovered.unwrap_err(), BufferRecoveryError);

        // simulate clearing the stack recursively
        for (index, (message, enclave_buffer)) in messages
            .into_iter()
            .zip(enclave_buffers.into_iter())
            .enumerate()
            .rev()
        {
            assert_eq!(ecall_stack_length(), index + 1);
            let recovered = unsafe { recover_buffer(enclave_buffer) };
            assert_eq!(recovered.unwrap().unwrap(), message.as_bytes())
        }
        assert_eq!(ecall_stack_length(), 0)
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
                assert_eq!(ecall_stack_length(), index);
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
            assert_eq!(ecall_stack_length(), index + 1);
            let recovered = unsafe { recover_buffer(enclave_buffer) };
            assert_eq!(recovered.unwrap().unwrap(), message.as_bytes())
        }
        assert_eq!(ecall_stack_length(), 0)
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
                assert_eq!(ecall_stack_length(), index);
                unsafe { ecall_allocate(message.as_ptr(), message.len()) }
            })
            .collect();

        let message = Box::new(Vec::<u8>::from(&b"some example text"[..]));
        let enclave_buffer = EnclaveBuffer {
            ptr: message.as_ptr() as _,
        };
        assert_eq!(ecall_stack_length(), recursion_depth);
        let recovered = unsafe { recover_buffer(enclave_buffer) };
        assert_eq!(ecall_stack_length(), recursion_depth);
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
            assert_eq!(ecall_stack_length(), index + 1);
            let recovered = unsafe { recover_buffer(enclave_buffer) };
            assert_eq!(recovered.unwrap().unwrap(), message.as_bytes())
        }
        assert_eq!(ecall_stack_length(), 0)
    }
}
