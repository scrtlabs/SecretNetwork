use super::exports;
use enclave_ffi_types::{
    CryptoError, EnclaveError, HandleResult, InitResult, KeyGenResult, QueryResult,
};

/// This struct is returned from module initialization.
pub struct InitSuccess {
    /// A pointer to the output of the execution
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

    pub fn into_output(self) -> Vec<u8> {
        self.output
    }

    pub fn used_gas(&self) -> u64 {
        self.used_gas
    }

    pub fn signature(&self) -> &[u8; 65] {
        &self.signature
    }
}

pub fn init_result_to_result_initsuccess(other: InitResult) -> Result<InitSuccess, EnclaveError> {
    match other {
        InitResult::Success {
            output,
            used_gas,
            signature,
        } => Ok(InitSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(|| Vec::new()),
            used_gas,
            signature,
        }),
        InitResult::Failure { err } => Err(err),
    }
}

/// This struct is returned from a handle method.
pub struct HandleSuccess {
    /// A pointer to the output of the execution
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

    pub fn into_output(self) -> Vec<u8> {
        self.output
    }

    pub fn used_gas(&self) -> u64 {
        self.used_gas
    }

    pub fn signature(&self) -> &[u8; 65] {
        &self.signature
    }
}

pub fn handle_result_to_result_handlesuccess(
    other: HandleResult,
) -> Result<HandleSuccess, EnclaveError> {
    match other {
        HandleResult::Success {
            output,
            used_gas,
            signature,
        } => Ok(HandleSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(|| Vec::new()),
            used_gas,
            signature,
        }),
        HandleResult::Failure { err } => Err(err),
    }
}

/// This struct is returned from a query method.
pub struct QuerySuccess {
    /// A pointer to the output of the execution
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

    pub fn into_output(self) -> Vec<u8> {
        self.output
    }

    pub fn used_gas(&self) -> u64 {
        self.used_gas
    }

    pub fn signature(&self) -> &[u8; 65] {
        &self.signature
    }
}

pub fn query_result_to_result_querysuccess(
    other: QueryResult,
) -> Result<QuerySuccess, EnclaveError> {
    match other {
        QueryResult::Success {
            output,
            used_gas,
            signature,
        } => Ok(QuerySuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(|| Vec::new()),
            used_gas,
            signature,
        }),
        QueryResult::Failure { err } => Err(err),
    }
}

/// This struct is returned from key-pair generation.
pub struct KeyGenSuccess {
    /// A pointer to the output of the execution
    output: Vec<u8>,
    /// A signature by the enclave on all of the results.
    signature: [u8; 65],
}

impl KeyGenSuccess {
    pub fn output(&self) -> &[u8] {
        &self.output
    }

    pub fn into_output(self) -> Vec<u8> {
        self.output
    }

    pub fn signature(&self) -> &[u8; 65] {
        &self.signature
    }
}

// TODO not sure if we need key_gen_result_to_result_key_gensuccess
pub fn key_gen_result_to_result_key_gensuccess(
    other: KeyGenResult,
) -> Result<KeyGenSuccess, CryptoError> {
    match other {
        KeyGenResult::Success { output, signature } => Ok(KeyGenSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(|| Vec::new()),
            signature,
        }),
        KeyGenResult::Failure { err } => Err(err),
    }
}
