use std::ffi::c_void;

use sgx_types::SgxResult;

use enclave_ffi_types::{Ctx, EnclaveBuffer, OcallReturn, UntrustedVmError, UserSpaceBuffer};

use cosmwasm_std::{Binary, StdResult, SystemResult};

use crate::context::{with_querier_from_context, with_storage_from_context};
use crate::{Querier, Storage, VmError, VmResult};

#[no_mangle]
pub extern "C" fn ocall_allocate(buffer: *const u8, length: usize) -> UserSpaceBuffer {
    ocall_allocate_impl(buffer, length)
}

#[cfg(feature = "query-node")]
#[no_mangle]
pub extern "C" fn ocall_allocate_qe(buffer: *const u8, length: usize) -> UserSpaceBuffer {
    ocall_allocate_impl(buffer, length)
}

/// Copy a buffer from the enclave memory space, and return an opaque pointer to it.
fn ocall_allocate_impl(buffer: *const u8, length: usize) -> UserSpaceBuffer {
    let slice = unsafe { std::slice::from_raw_parts(buffer, length) };
    let vector_copy = slice.to_vec();
    let boxed_vector = Box::new(vector_copy);
    let heap_pointer = Box::into_raw(boxed_vector);
    UserSpaceBuffer {
        ptr: heap_pointer as *mut c_void,
    }
}

/// Take a pointer as returned by `ocall_allocate` and recover the Vec<u8> inside of it.
pub unsafe fn recover_buffer(ptr: UserSpaceBuffer) -> Option<Vec<u8>> {
    if ptr.ptr.is_null() {
        return None;
    }
    let boxed_vector = Box::from_raw(ptr.ptr as *mut Vec<u8>);
    Some(*boxed_vector)
}

#[no_mangle]
pub extern "C" fn ocall_read_db(
    context: Ctx,
    vm_error: *mut UntrustedVmError,
    gas_used: *mut u64,
    value: *mut EnclaveBuffer,
    key: *const u8,
    key_len: usize,
) -> OcallReturn {
    ocall_read_db_concrete(
        super::allocate_enclave_buffer,
        context,
        vm_error,
        gas_used,
        value,
        key,
        key_len,
    )
}

#[cfg(feature = "query-node")]
#[no_mangle]
pub extern "C" fn ocall_read_db_qe(
    context: Ctx,
    vm_error: *mut UntrustedVmError,
    gas_used: *mut u64,
    value: *mut EnclaveBuffer,
    key: *const u8,
    key_len: usize,
) -> OcallReturn {
    ocall_read_db_concrete(
        super::allocate_enclave_buffer_qe,
        context,
        vm_error,
        gas_used,
        value,
        key,
        key_len,
    )
}

/// Read a key from the contracts key-value store.
fn ocall_read_db_concrete(
    alloc_impl: fn(&[u8]) -> SgxResult<EnclaveBuffer>,
    context: Ctx,
    vm_error: *mut UntrustedVmError,
    gas_used: *mut u64,
    value: *mut EnclaveBuffer,
    key: *const u8,
    key_len: usize,
) -> OcallReturn {
    let key = unsafe { std::slice::from_raw_parts(key, key_len) };

    let implementation = unsafe { get_implementations_from_context(&context).read_db };

    std::panic::catch_unwind(|| implementation(context, key))
        // Get either an error(`OcallReturn`), or a response(`EnclaveBuffer`)
        // which will be converted to a success status.
        .map(|result| -> Result<EnclaveBuffer, OcallReturn> {
            match result {
                Ok((value, gas_cost)) => {
                    unsafe { *gas_used = gas_cost };
                    value
                        .map(|val| alloc_impl(&val).map_err(|_| OcallReturn::Failure))
                        .unwrap_or_else(|| Ok(EnclaveBuffer::default()))
                }
                Err(err) => {
                    unsafe { store_vm_error(err, vm_error) };
                    Err(OcallReturn::Failure)
                }
            }
        })
        // Return the result or report the error
        .map(|result| match result {
            Ok(enclave_buffer) => {
                unsafe { *value = enclave_buffer };
                OcallReturn::Success
            }
            Err(err) => err,
        })
        // This will happen only when `catch_unwind` returns `Err`, which indicates a caught panic
        .unwrap_or(OcallReturn::Panic)
}

#[no_mangle]
pub extern "C" fn ocall_query_chain(
    context: Ctx,
    vm_error: *mut UntrustedVmError,
    gas_used: *mut u64,
    gas_limit: u64,
    value: *mut EnclaveBuffer,
    query: *const u8,
    query_len: usize,
) -> OcallReturn {
    ocall_query_chain_concrete(
        super::allocate_enclave_buffer,
        context,
        vm_error,
        gas_used,
        gas_limit,
        value,
        query,
        query_len,
    )
}

#[cfg(feature = "query-node")]
#[no_mangle]
pub extern "C" fn ocall_query_chain_qe(
    context: Ctx,
    vm_error: *mut UntrustedVmError,
    gas_used: *mut u64,
    gas_limit: u64,
    value: *mut EnclaveBuffer,
    query: *const u8,
    query_len: usize,
) -> OcallReturn {
    ocall_query_chain_concrete(
        super::allocate_enclave_buffer_qe,
        context,
        vm_error,
        gas_used,
        gas_limit,
        value,
        query,
        query_len,
    )
}

/// Read a key from the contracts key-value store.
#[allow(clippy::too_many_arguments)]
fn ocall_query_chain_concrete(
    alloc_impl: fn(&[u8]) -> SgxResult<EnclaveBuffer>,
    context: Ctx,
    vm_error: *mut UntrustedVmError,
    gas_used: *mut u64,
    gas_limit: u64,
    value: *mut EnclaveBuffer,
    query: *const u8,
    query_len: usize,
) -> OcallReturn {
    let query = unsafe { std::slice::from_raw_parts(query, query_len) };

    let implementation = unsafe { get_implementations_from_context(&context).query_chain };

    std::panic::catch_unwind(|| implementation(context, query, gas_limit))
        // Get either an error(`OcallReturn`), or a response(`EnclaveBuffer`)
        // which will be converted to a success status.
        .map(|answer| -> Result<EnclaveBuffer, OcallReturn> {
            match answer {
                Ok((system_result, gas_cost)) => {
                    unsafe { *gas_used = gas_cost };

                    // wasm code expects to get this as Result<Result<Binary, StdError>, SystemError> which is called SystemResult
                    // see CosmWasm's implementation https://github.com/enigmampc/SecretNetwork/blob/508e99c990dd656eb61f456584dab054487ba178/cosmwasm/packages/sgx-vm/src/imports.rs#L124

                    crate::serde::to_vec(&system_result)
                        .map(|val| alloc_impl(&val).map_err(|_| OcallReturn::Failure))
                        .unwrap_or_else(|_| Ok(EnclaveBuffer::default()))
                }
                Err(err) => {
                    unsafe { store_vm_error(err, vm_error) };
                    Err(OcallReturn::Failure)
                }
            }
        })
        // Return the result or report the error
        .map(|result| match result {
            Ok(enclave_buffer) => {
                unsafe { *value = enclave_buffer };
                OcallReturn::Success
            }
            Err(err) => err,
        })
        // This will happen only when `catch_unwind` returns `Err`, which indicates a caught panic
        .unwrap_or(OcallReturn::Panic)
}

/// Remove a key from the contracts key-value store.
#[no_mangle]
pub extern "C" fn ocall_remove_db(
    context: Ctx,
    vm_error: *mut UntrustedVmError,
    gas_used: *mut u64,
    key: *const u8,
    key_len: usize,
) -> OcallReturn {
    let key = unsafe { std::slice::from_raw_parts(key, key_len) };

    let implementation = unsafe { get_implementations_from_context(&context).remove_db };

    // We explicitly ignore this potential panic here because we have no way of handling it at the moment.
    // In the future, if we see that panics do occur here, we should add a way to report this to the enclave.
    // TODO add logging if we fail to write
    std::panic::catch_unwind(|| match implementation(context, key) {
        Ok(gas_cost) => {
            unsafe { *gas_used = gas_cost };
            OcallReturn::Success
        }
        Err(err) => {
            unsafe { store_vm_error(err, vm_error) };
            OcallReturn::Failure
        }
    })
    // This will happen only when `catch_unwind` returns `Err`, which indicates a caught panic
    .unwrap_or(OcallReturn::Panic)
}

/// Write a value to the contracts key-value store.
#[no_mangle]
pub extern "C" fn ocall_write_db(
    context: Ctx,
    vm_error: *mut UntrustedVmError,
    gas_used: *mut u64,
    key: *const u8,
    key_len: usize,
    value: *const u8,
    value_len: usize,
) -> OcallReturn {
    let key = unsafe { std::slice::from_raw_parts(key, key_len) };
    let value = unsafe { std::slice::from_raw_parts(value, value_len) };

    let implementation = unsafe { get_implementations_from_context(&context).write_db };

    // We explicitly ignore this potential panic here because we have no way of handling it at the moment.
    // In the future, if we see that panics do occur here, we should add a way to report this to the enclave.
    // TODO add logging if we fail to write
    std::panic::catch_unwind(|| match implementation(context, key, value) {
        Ok(gas_cost) => {
            unsafe { *gas_used = gas_cost };
            OcallReturn::Success
        }
        Err(err) => {
            unsafe { store_vm_error(err, vm_error) };
            OcallReturn::Failure
        }
    })
    // This will happen only when `catch_unwind` returns `Err`, which indicates a caught panic
    .unwrap_or(OcallReturn::Panic)
}

/// Box the error and return a pointer to it.
/// This box will be recovered on the side that called the enclave.
///
/// # Safety
/// Make sure that the pointer is valid
unsafe fn store_vm_error(vm_err: VmError, location: *mut UntrustedVmError) {
    let boxed_err = Box::new(vm_err);
    let err_ptr = Box::leak(boxed_err) as *mut _ as *mut c_void;
    *location = UntrustedVmError::new(err_ptr);
}

/// This type allows us to dynamically dispatch on the ocall side based on the generic implementation that the
/// original caller requested, without any downcasting magic.
/// The side that calls into the enclave will call the `new()` method with the Generic arguments that are
/// appropriate for it.
#[allow(clippy::type_complexity)]
struct ExportImplementations {
    read_db: fn(context: Ctx, key: &[u8]) -> VmResult<(Option<Vec<u8>>, u64)>,
    query_chain: fn(
        context: Ctx,
        query: &[u8],
        gas_limit: u64,
    ) -> VmResult<(SystemResult<StdResult<Binary>>, u64)>,
    remove_db: fn(context: Ctx, key: &[u8]) -> VmResult<u64>,
    write_db: fn(context: Ctx, key: &[u8], value: &[u8]) -> VmResult<u64>,
}

impl ExportImplementations {
    fn new<S, Q>() -> Self
    where
        S: Storage,
        Q: Querier,
    {
        Self {
            read_db: ocall_read_db_impl::<S, Q>,
            query_chain: ocall_query_chain_impl::<S, Q>,
            remove_db: ocall_remove_db_impl::<S, Q>,
            write_db: ocall_write_db_impl::<S, Q>,
        }
    }
}

/// This type is a wrapper for the `*mut c_void` that the original `cosmwasm_vm` implementation used to provide to
/// the wasmer context object. It's a pointer to the `ContextData` type.
/// We also add pointers to the concrete monomorphization of the generic implementation of the imports.
/// This allows us to keep a minimal diff from the original codebase, by using most of their infrastructure,
/// and allowing us to pull in future changes.
pub(crate) struct FullContext {
    pub(crate) context_data: *mut c_void,
    implementation: ExportImplementations,
}

impl FullContext {
    pub(crate) fn new<S, Q>(context_data: *mut c_void) -> Self
    where
        S: Storage,
        Q: Querier,
    {
        Self {
            context_data,
            implementation: ExportImplementations::new::<S, Q>(),
        }
    }
}

/// This function assumes all pointers in the `Ctx` are valid
unsafe fn get_implementations_from_context(context: &Ctx) -> &ExportImplementations {
    &(*(context.data as *mut FullContext)).implementation
}

fn ocall_read_db_impl<S, Q>(mut context: Ctx, key: &[u8]) -> VmResult<(Option<Vec<u8>>, u64)>
where
    S: Storage,
    Q: Querier,
{
    with_storage_from_context::<S, Q, _, _>(&mut context, |storage: &mut S| {
        let (ffi_result, gas_info) = storage.get(key);
        ffi_result
            .map(|value| (value, gas_info.externally_used))
            .map_err(Into::into)
    })
}

fn ocall_query_chain_impl<S, Q>(
    mut context: Ctx,
    query: &[u8],
    gas_limit: u64,
) -> VmResult<(SystemResult<StdResult<Binary>>, u64)>
where
    S: Storage,
    Q: Querier,
{
    with_querier_from_context::<S, Q, _, _>(&mut context, |querier: &mut Q| {
        let (ffi_result, gas_info) = querier.query_raw(query, gas_limit);
        ffi_result
            .map(|system_result| (system_result, gas_info.externally_used))
            .map_err(Into::into)
    })
}

fn ocall_remove_db_impl<S, Q>(mut context: Ctx, key: &[u8]) -> VmResult<u64>
where
    S: Storage,
    Q: Querier,
{
    with_storage_from_context::<S, Q, _, _>(&mut context, |storage: &mut S| {
        let (ffi_result, gas_info) = storage.remove(key);
        ffi_result
            .and(Ok(gas_info.externally_used))
            .map_err(Into::into)
    })
}

fn ocall_write_db_impl<S, Q>(mut context: Ctx, key: &[u8], value: &[u8]) -> VmResult<u64>
where
    S: Storage,
    Q: Querier,
{
    with_storage_from_context::<S, Q, _, _>(&mut context, |storage: &mut S| {
        let (ffi_result, gas_info) = storage.set(key, value);
        ffi_result
            .and(Ok(gas_info.externally_used))
            .map_err(Into::into)
    })
}
