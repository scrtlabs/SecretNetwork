use std::prelude::v1::*;

use enclave_ffi_types::{Ctx, EnclaveError, UserSpaceBuffer};

use super::imports;
use super::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::exports;

/// This is a safe wrapper for allocating buffers outside the enclave.
pub(super) fn allocate_user_space_buffer(buffer: &[u8]) -> UserSpaceBuffer {
    let ptr = buffer.as_ptr();
    let len = buffer.len();
    unsafe { imports::ocall_allocate(ptr, len) }
}

/// Safe wrapper around reads from the contract storage
fn read_db(context: Ctx, key: &[u8]) -> Option<Vec<u8>> {
    unsafe { exports::recover_buffer(imports::ocall_read_db(context, key.as_ptr(), key.len())) }
}

/// Safe wrapper around writes to the contract storage
fn write_db(context: Ctx, key: &[u8], value: &[u8]) {
    unsafe {
        imports::ocall_write_db(
            context,
            key.as_ptr(),
            key.len(),
            value.as_ptr(),
            value.len(),
        )
    }
}

pub fn init(
    context: Ctx,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
) -> Result<InitSuccess, EnclaveError> {
    todo!()
}

pub fn handle(
    context: Ctx,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
) -> Result<HandleSuccess, EnclaveError> {
    todo!()
}

pub fn query(context: Ctx, contract: &[u8], msg: &[u8]) -> Result<QuerySuccess, EnclaveError> {
    todo!()
}
