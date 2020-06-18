use enclave_ffi_types::{Ctx, EnclaveBuffer, UserSpaceBuffer};
use log::info;
use std::ffi::c_void;

use crate::context::with_storage_from_context;
use crate::{Querier, Storage, VmResult};

/// Copy a buffer from the enclave memory space, and return an opaque pointer to it.
#[no_mangle]
pub extern "C" fn ocall_allocate(buffer: *const u8, length: usize) -> UserSpaceBuffer {
    info!(
        target: module_path!(),
        "ocall_allocate() called with buffer length: {:?}", length
    );

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

/// Read a key from the contracts key-value store.
/// instance_id should be the sha256 of the wasm blob.
#[no_mangle]
pub extern "C" fn ocall_read_db(context: Ctx, key: *const u8, key_len: usize) -> EnclaveBuffer {
    let key = unsafe { std::slice::from_raw_parts(key, key_len) };

    info!(
        target: module_path!(),
        "ocall_read_db() called with len: {:?} key: {:?}",
        key_len,
        String::from_utf8_lossy(key)
    );
    let null_buffer = EnclaveBuffer {
        ptr: std::ptr::null_mut(),
    };

    let implementation = unsafe { get_implementations_from_context(&context).read_db };

    // Returning `EnclaveBuffer { ptr: std::ptr::null_mut() }` is basically returning a null pointer,
    // which in the enclave is interpreted as signaling that the key does not exist.
    // We also interpret this potential panic here as a missing key because we have no way of handling
    // it at the moment.
    // In the future, if we see that panics do occur here, we should add a way to report this to the enclave.
    // TODO handle errors and return the gas cost
    std::panic::catch_unwind(|| implementation(context, key).unwrap().0)
        .map(|value| {
            value
                .map(|vec| {
                    super::allocate_enclave_buffer(&vec)
                        .unwrap_or(unsafe { null_buffer.unsafe_clone() })
                })
                .unwrap_or(unsafe { null_buffer.unsafe_clone() })
        })
        // TODO add logging if we fail to write
        .unwrap_or(unsafe { null_buffer.unsafe_clone() })
}

/// Write a value to the contracts key-value store.
/// instance_id should be the sha256 of the wasm blob.
#[no_mangle]
pub extern "C" fn ocall_write_db(
    context: Ctx,
    key: *const u8,
    key_len: usize,
    value: *const u8,
    value_len: usize,
) {
    let key = unsafe { std::slice::from_raw_parts(key, key_len) };
    let value = unsafe { std::slice::from_raw_parts(value, value_len) };

    info!(
        target: module_path!(),
        "ocall_write_db() called with key_len: {:?} key: {:?} val_len: {:?} val: {:?}... (first 20 bytes)",
        key_len,
        String::from_utf8_lossy(key),
        value_len,
        String::from_utf8_lossy(value.get(0..std::cmp::min(20, value_len)).unwrap())
    );

    let implementation = unsafe { get_implementations_from_context(&context).write_db };

    // We explicitly ignore this potential panic here because we have no way of handling it at the moment.
    // In the future, if we see that panics do occur here, we should add a way to report this to the enclave.
    // TODO handle errors and return the gas cost
    let _ = std::panic::catch_unwind(|| implementation(context, key, value).unwrap());
    // TODO add logging if we fail to write
}

/// This type allows us to dynamically dispatch on the ocall side based on the generic implementation that the
/// original caller requested, without any downcasting magic.
/// The side that calls into the enclave will call the `new()` method with the Generic arguments that are
/// appropriate for it.
struct ExportImplementations {
    read_db: fn(context: Ctx, key: &[u8]) -> VmResult<(Option<Vec<u8>>, u64)>,
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
        storage.get(key).map_err(Into::into)
    })
}

fn ocall_write_db_impl<S, Q>(mut context: Ctx, key: &[u8], value: &[u8]) -> VmResult<u64>
where
    S: Storage,
    Q: Querier,
{
    with_storage_from_context::<S, Q, _, _>(&mut context, |storage: &mut S| {
        storage.set(key, value).map_err(Into::into)
    })
}
