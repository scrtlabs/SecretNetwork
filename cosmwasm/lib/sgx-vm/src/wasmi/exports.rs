use enclave_ffi_types::{Ctx, EnclaveBuffer, UserSpaceBuffer};
use log::info;
use std::ffi::c_void;

use crate::context::with_storage_from_context;

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
pub extern "C" fn ocall_read_db(mut context: Ctx, key: *const u8, key_len: usize) -> EnclaveBuffer {
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

    // Returning `EnclaveBuffer { ptr: std::ptr::null_mut() }` is basically returning a null pointer,
    // which in the enclave is interpreted as signaling that the key does not exist.
    // We also interpret this potential panic here as a missing key because we have no way of handling
    // it at the moment.
    // In the future, if we see that panics do occur here, we should add a way to report this to the enclave.
    std::panic::catch_unwind(move || {
        let mut value: Option<Vec<u8>> = None;
        with_storage_from_context(&mut context, |storage| value = storage.get(key));
        value
    })
    .map(|value| {
        value
            .map(|vec| {
                super::allocate_enclave_buffer(&vec).unwrap_or(unsafe { null_buffer.clone() })
            })
            .unwrap_or(unsafe { null_buffer.clone() })
    })
    // TODO add logging if we fail to write
    .unwrap_or(unsafe { null_buffer.clone() })
}

/// Write a value to the contracts key-value store.
/// instance_id should be the sha256 of the wasm blob.
#[no_mangle]
pub extern "C" fn ocall_write_db(
    mut context: Ctx,
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

    // We explicitly ignore this potential panic here because we have no way of handling it at the moment.
    // In the future, if we see that panics do occur here, we should add a way to report this to the enclave.
    let _ = std::panic::catch_unwind(move || {
        with_storage_from_context(&mut context, |storage| storage.set(key, value))
    }); // TODO add logging if we fail to write
}
