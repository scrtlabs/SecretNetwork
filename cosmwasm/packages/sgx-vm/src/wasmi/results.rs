use super::exports;
use enclave_ffi_types::{EnclaveError, HandleResult, InitResult, QueryResult};

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

pub fn init_result_to_result_initsuccess(other: InitResult) -> Result<InitSuccess, EnclaveError> {
    match other {
        InitResult::Success {
            output,
            used_gas: _,
            signature,
        } => Ok(InitSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(|| Vec::new()),
            signature,
        }),
        InitResult::Failure { err, used_gas: _ } => Err(err),
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

pub fn handle_result_to_result_handlesuccess(
    other: HandleResult,
) -> Result<HandleSuccess, EnclaveError> {
    match other {
        HandleResult::Success {
            output,
            used_gas: _,
            signature,
        } => Ok(HandleSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(|| Vec::new()),
            signature,
        }),
        HandleResult::Failure { err, used_gas: _ } => Err(err),
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

pub fn query_result_to_result_querysuccess(
    other: QueryResult,
) -> Result<QuerySuccess, EnclaveError> {
    match other {
        QueryResult::Success {
            output,
            used_gas: _,
            signature,
        } => Ok(QuerySuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(|| Vec::new()),
            signature,
        }),
        QueryResult::Failure { err, used_gas: _ } => Err(err),
    }
}
