use super::exports;
use enclave_ffi_types::{EnclaveError, InitResult, HandleResult, QueryResult};

/// This struct is returned from module initialization.
pub struct InitSuccess {
    /// A pointer to the output of the execution using `ocall_save_to_memory`
    output: Vec<u8>,
    /// The gas used by the execution.
    used_gas: u64,
    /// A signature by the enclave on all of the results.
    signature: [u8; 65],
}

impl InitSuccess {
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

impl std::convert::From<InitResult> for Result<InitSuccess, EnclaveError> {
    fn from(other: InitResult) -> Self {
        match other {
            InitResult::Success {
                output,
                used_gas,
                signature,
            } => Ok(InitSuccess {
                output: unsafe { exports::recover_buffer(output) },
                used_gas,
                signature,
            }),
            InitResult::Failure { err } => Err(err),
        }
    }
}

/// This struct is returned from a handle method.
pub struct HandleSuccess {
    /// A pointer to the output of the execution using `ocall_save_to_memory`
    output: Vec<u8>,
    /// The gas used by the execution.
    used_gas: u64,
    /// A signature by the enclave on all of the results.
    signature: [u8; 65],
}

impl HandleSuccess {
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

impl std::convert::From<HandleResult> for Result<HandleSuccess, EnclaveError> {
    fn from(other: HandleResult) -> Self {
        match other {
            HandleResult::Success {
                output,
                used_gas,
                signature,
            } => Ok(HandleSuccess {
                output: unsafe { exports::recover_buffer(output) },
                used_gas,
                signature,
            }),
            HandleResult::Failure { err } => Err(err),
        }
    }
}

/// This struct is returned from a query method.
pub struct QuerySuccess {
    /// A pointer to the output of the execution using `ocall_save_to_memory`
    output: Vec<u8>,
    /// The gas used by the execution.
    used_gas: u64,
    /// A signature by the enclave on all of the results.
    signature: [u8; 65],
}

impl QuerySuccess {
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

impl std::convert::From<QueryResult> for Result<QuerySuccess, EnclaveError> {
    fn from(other: QueryResult) -> Self {
        match other {
            QueryResult::Success {
                output,
                used_gas,
                signature,
            } => Ok(QuerySuccess {
                output: unsafe { exports::recover_buffer(output) },
                used_gas,
                signature,
            }),
            QueryResult::Failure { err } => Err(err),
        }
    }
}
