use std::ffi::c_void;

use enclave_ffi_types::{Ctx, UserSpaceBuffer, EnclaveBuffer, HandleResult, InitResult, QueryResult};

/// Copy a buffer from the enclave memory space, and return an opaque pointer to it.
#[no_mangle]
pub extern "C" fn ocall_allocate(buffer: *const u8, length: usize) -> UserSpaceBuffer {
    let slice = unsafe { std::slice::from_raw_parts(buffer, length) };
    let vector_copy = slice.to_vec();
    let boxed_vector = Box::new(vector_copy);
    let heap_pointer = Box::into_raw(boxed_vector);
    UserSpaceBuffer { ptr: heap_pointer as *mut c_void }
}

/// Take a pointer as returned by `ocall_allocate` and recover the Vec<u8> inside of it.
pub unsafe fn recover_buffer(ptr: UserSpaceBuffer) -> Vec<u8> {
    let boxed_vector = Box::from_raw(ptr.ptr as *mut Vec<u8>);
    *boxed_vector
}

/// Read a key from the contracts key-value store.
/// instance_id should be the sha256 of the wasm blob.
#[no_mangle]
pub extern "C" fn ocall_read_db(
    context: Ctx,
    key: *const u8,
    key_len: usize,
) -> EnclaveBuffer {
    let _key = unsafe { std::slice::from_raw_parts(key, key_len) };
    // also add panic handlers here
    // how do we signal errors into the enclave? should we?
    let value = todo!();
    super::allocate_enclave_buffer(value)
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
    let _key = unsafe { std::slice::from_raw_parts(key, key_len) };
    let _value = unsafe { std::slice::from_raw_parts(value, value_len) };
    // also add panic handlers here
    todo!()
}
