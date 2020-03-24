//! This module provides safe wrappers for the calls into the enclave running WASMI.

use std::ffi::c_void;

use enclave_ffi_types::EnclaveBuffer;

use crate::errors::Result;

use super::imports;
use super::exports;


/// This is a safe wrapper for allocating buffers inside the enclave.
pub(super) fn allocate_enclave_buffer(buffer: &[u8]) -> EnclaveBuffer {
    let ptr = buffer.as_ptr();
    let len = buffer.len();
    unsafe { imports::ecall_allocate(ptr, len) }
}


/// This struct is returned from module initialization.
pub struct InitResult {
    /// A pointer to the output of the execution using `ocall_save_to_memory`
    output: Vec<u8>,
    /// The gas used by the execution.
    used_gas: u64,
    /// A signature by the enclave on all of the results.
    signature: [u8; 65],
}

impl InitResult {
    pub fn output(&self) -> &[u8] {
        &self.output
    }

    pub fn used_gas(&self) -> u64 {
        self.used_gas
    }

    pub fn signature(&self) -> &[u8; 65] {
        &self.signature
    }
}

impl std::convert::From<enclave_ffi_types::InitResult> for InitResult {
    fn from(other: enclave_ffi_types::InitResult) -> Self {
        let enclave_ffi_types::InitResult {
            output,
            used_gas,
            signature,
        } = other;
        Self {
            output: unsafe { exports::recover_buffer(output) },
            used_gas,
            signature,
        }
    }
}

/// This struct is returned from a handle method.
pub struct HandleResult {
    /// A pointer to the output of the execution using `ocall_save_to_memory`
    output: Vec<u8>,
    /// The gas used by the execution.
    used_gas: u64,
    /// A signature by the enclave on all of the results.
    signature: [u8; 65],
}

impl HandleResult {
    pub fn output(&self) -> &[u8] {
        &self.output
    }

    pub fn used_gas(&self) -> u64 {
        self.used_gas
    }

    pub fn signature(&self) -> &[u8; 65] {
        &self.signature
    }
}

impl std::convert::From<enclave_ffi_types::HandleResult> for HandleResult {
    fn from(other: enclave_ffi_types::HandleResult) -> Self {
        let enclave_ffi_types::HandleResult {
            output,
            used_gas,
            signature,
        } = other;
        Self {
            output: unsafe { exports::recover_buffer(output) },
            used_gas,
            signature,
        }
    }
}

/// This struct is returned from a query method.
pub struct QueryResult {
    /// A pointer to the output of the execution using `ocall_save_to_memory`
    output: Vec<u8>,
    /// The gas used by the execution.
    used_gas: u64,
    /// A signature by the enclave on all of the results.
    signature: [u8; 65],
}

impl QueryResult {
    pub fn output(&self) -> &[u8] {
        &self.output
    }

    pub fn used_gas(&self) -> u64 {
        self.used_gas
    }

    pub fn signature(&self) -> &[u8; 65] {
        &self.signature
    }
}

impl std::convert::From<enclave_ffi_types::QueryResult> for QueryResult {
    fn from(other: enclave_ffi_types::QueryResult) -> Self {
        let enclave_ffi_types::QueryResult {
            output,
            used_gas,
            signature,
        } = other;
        Self {
            output: unsafe { exports::recover_buffer(output) },
            used_gas,
            signature,
        }
    }
}

pub struct Module {
    bytecode: Vec<u8>,
}

impl Module {
    pub fn new(bytecode: Vec<u8>) -> Self {
        // TODO add validation of this bytecode?
        Self { bytecode }
    }

    pub fn init(&self, env: &[u8], msg: &[u8]) -> InitResult {
        let init_result = unsafe {
            imports::ecall_init(
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
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                msg.as_ptr(),
                msg.len(),
            )
        };
        query_result.into()
    }
}
