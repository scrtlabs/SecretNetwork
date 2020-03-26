use std::prelude::v1::*;

use enclave_ffi_types::{EnclaveBuffer, UserSpaceBuffer};

use super::imports;

/// This struct is returned from module initialization.
pub struct InitResult {
    /// The output of the calculation
    pub output: Vec<u8>,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 65],
}

impl std::convert::From<InitResult> for enclave_ffi_types::InitResult {
    fn from(other: InitResult) -> Self {
        let InitResult {
            output,
            used_gas,
            signature,
        } = other;
        Self {
            output: unsafe { imports::ocall_allocate(output.as_ptr(), output.len()) },
            used_gas,
            signature,
        }
    }
}

/// This struct is returned from a handle method.
pub struct HandleResult {
    /// The output of the calculation
    pub output: Vec<u8>,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 65],
}

impl std::convert::From<HandleResult> for enclave_ffi_types::HandleResult {
    fn from(other: HandleResult) -> Self {
        let HandleResult {
            output,
            used_gas,
            signature,
        } = other;
        Self {
            output: unsafe { imports::ocall_allocate(output.as_ptr(), output.len()) },
            used_gas,
            signature,
        }
    }
}

/// This struct is returned from a query method.
pub struct QueryResult {
    /// The output of the calculation
    pub output: Vec<u8>,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 65],
}

impl std::convert::From<QueryResult> for enclave_ffi_types::QueryResult {
    fn from(other: QueryResult) -> Self {
        let QueryResult {
            output,
            used_gas,
            signature,
        } = other;
        Self {
            output: unsafe { imports::ocall_allocate(output.as_ptr(), output.len()) },
            used_gas,
            signature,
        }
    }
}
