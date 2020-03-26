//! This module provides safe wrappers for the calls into the enclave running WASMI.

use std::ffi::c_void;

use enclave_ffi_types::{ EnclaveBuffer, Ctx};

use crate::errors::Result;

use super::imports;
use super::exports;
use super::results::{InitResult, HandleResult, QueryResult};


/// This is a safe wrapper for allocating buffers inside the enclave.
pub(super) fn allocate_enclave_buffer(buffer: &[u8]) -> EnclaveBuffer {
    let ptr = buffer.as_ptr();
    let len = buffer.len();
    unsafe { imports::ecall_allocate(ptr, len) }
}

pub struct Module {
    bytecode: Vec<u8>,
    context: Ctx,
    gas_limit: u64,
}

impl Module {
    pub fn new(bytecode: Vec<u8>, storage: *mut c_void, gas_limit: u64) -> Self {
        // TODO add validation of this bytecode?
        let context = Ctx { data: storage };
        Self { bytecode, context, gas_limit }
    }

    pub fn init(&self, env: &[u8], msg: &[u8]) -> InitResult {
        let init_result = unsafe {
            imports::ecall_init(
                self.context,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                env.as_ptr(),
                env.len(),
                msg.as_ptr(),
                msg.len(),
            )
        };
        init_result.into()
    }

    pub fn handle(&self, env: &[u8], msg: &[u8]) -> HandleResult {
        let handle_result = unsafe {
            imports::ecall_handle(
                self.context,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                env.as_ptr(),
                env.len(),
                msg.as_ptr(),
                msg.len(),
            )
        };
        handle_result.into()
    }

    pub fn query(&self, msg: &[u8]) -> QueryResult {
        let query_result = unsafe {
            imports::ecall_query(
                self.context,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                msg.as_ptr(),
                msg.len(),
            )
        };
        query_result.into()
    }
}
