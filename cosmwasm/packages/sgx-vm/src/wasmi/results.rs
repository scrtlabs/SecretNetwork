use super::exports;
use crate::VmResult;
use enclave_ffi_types::{HandleResult, InitResult, QueryResult};

/// This struct is returned from module initialization.
pub struct InitSuccess {
    /// A pointer to the output of the execution
    output: Vec<u8>,
    /// A signature by the enclave on all of the results.
    signature: [u8; 64],
}

impl InitSuccess {
    pub fn into_output(self) -> Vec<u8> {
        let mut out_vec = self.signature.to_vec();
        out_vec.extend_from_slice(&self.output);
        out_vec
    }

    pub fn signature(&self) -> &[u8; 64] {
        &self.signature
    }
}

pub fn init_result_to_vm_result(other: InitResult) -> VmResult<InitSuccess> {
    match other {
        InitResult::Success { output, signature } => Ok(InitSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(Vec::new),
            signature,
        }),
        InitResult::Failure { err } => Err(err.into()),
    }
}

/// This struct is returned from a handle method.
pub struct HandleSuccess {
    /// A pointer to the output of the execution
    output: Vec<u8>,
    /// A signature by the enclave on all of the results.
    signature: [u8; 64],
}

impl HandleSuccess {
    pub fn into_output(self) -> Vec<u8> {
        self.output
    }

    pub fn signature(&self) -> &[u8; 64] {
        &self.signature
    }
}

pub fn handle_result_to_vm_result(other: HandleResult) -> VmResult<HandleSuccess> {
    match other {
        HandleResult::Success { output, signature } => Ok(HandleSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(Vec::new),
            signature,
        }),
        HandleResult::Failure { err } => Err(err.into()),
    }
}

/// This struct is returned from a query method.
pub struct QuerySuccess {
    /// A pointer to the output of the execution
    output: Vec<u8>,
    /// A signature by the enclave on all of the results.
    signature: [u8; 64],
}

impl QuerySuccess {
    pub fn into_output(self) -> Vec<u8> {
        self.output
    }

    pub fn signature(&self) -> &[u8; 64] {
        &self.signature
    }
}

pub fn query_result_to_vm_result(other: QueryResult) -> VmResult<QuerySuccess> {
    match other {
        QueryResult::Success { output, signature } => Ok(QuerySuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(Vec::new),
            signature,
        }),
        QueryResult::Failure { err } => Err(err.into()),
    }
}
