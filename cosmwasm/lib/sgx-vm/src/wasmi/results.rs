use super::exports;

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
