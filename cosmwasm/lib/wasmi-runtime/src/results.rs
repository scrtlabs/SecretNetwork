use std::prelude::v1::*;

use enclave_ffi_types::{EnclaveError, HandleResult, InitResult, QueryResult};

use super::imports;

/// This struct is returned from module initialization.
pub struct InitSuccess {
    /// The output of the calculation
    pub output: Vec<u8>,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 65],
}

pub fn result_init_success_to_initresult(result: Result<InitSuccess, EnclaveError>) -> InitResult {
    match result {
        Ok(InitSuccess {
            output,
            used_gas,
            signature,
        }) => InitResult::Success {
            output: unsafe { imports::ocall_allocate(output.as_ptr(), output.len()) },
            used_gas,
            signature,
        },
        Err(err) => InitResult::Failure { err },
    }
}

/// This struct is returned from a handle method.
pub struct HandleSuccess {
    /// The output of the calculation
    pub output: Vec<u8>,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 65],
}

pub fn result_handle_success_to_handleresult(
    result: Result<HandleSuccess, EnclaveError>,
) -> HandleResult {
    match result {
        Ok(HandleSuccess {
            output,
            used_gas,
            signature,
        }) => HandleResult::Success {
            output: unsafe { imports::ocall_allocate(output.as_ptr(), output.len()) },
            used_gas,
            signature,
        },
        Err(err) => HandleResult::Failure { err },
    }
}

/// This struct is returned from a query method.
pub struct QuerySuccess {
    /// The output of the calculation
    pub output: Vec<u8>,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 65],
}

pub fn result_query_success_to_queryresult(
    result: Result<QuerySuccess, EnclaveError>,
) -> QueryResult {
    match result {
        Ok(QuerySuccess {
            output,
            used_gas,
            signature,
        }) => QueryResult::Success {
            output: unsafe { imports::ocall_allocate(output.as_ptr(), output.len()) },
            used_gas,
            signature,
        },
        Err(err) => QueryResult::Failure { err },
    }
}
