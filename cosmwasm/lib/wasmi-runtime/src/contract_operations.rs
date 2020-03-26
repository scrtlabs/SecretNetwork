use std::prelude::v1::*;

use enclave_ffi_types::{Ctx, UserSpaceBuffer};

use super::imports;
use super::results::{HandleResult, InitResult, QueryResult};

/// This is a safe wrapper for allocating buffers outside the enclave.
pub(super) fn allocate_user_space_buffer(buffer: &[u8]) -> UserSpaceBuffer {
    let ptr = buffer.as_ptr();
    let len = buffer.len();
    unsafe { imports::ocall_allocate(ptr, len) }
}

pub fn init(context: Ctx, contract: &[u8], env: &[u8], msg: &[u8]) -> InitResult {
    todo!()
}

pub fn handle(context: Ctx, contract: &[u8], env: &[u8], msg: &[u8]) -> HandleResult {
    todo!()
}

pub fn query(context: Ctx, contract: &[u8], msg: &[u8]) -> QueryResult {
    todo!()
}
