use enclave_ffi_types::{EnclaveError, HandleResult, InitResult, QueryResult, UserSpaceBuffer};
use sgx_types::sgx_status_t;

use crate::imports::ocall_allocate;

/// This struct is returned from module initialization.
pub struct InitSuccess {
    /// The output of the calculation
    pub output: Vec<u8>,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 64],
}

pub fn result_init_success_to_initresult(result: Result<InitSuccess, EnclaveError>) -> InitResult {
    match result {
        Ok(InitSuccess {
            output,
            used_gas,
            signature,
        }) => {
            let user_buffer = unsafe {
                let mut user_buffer = std::mem::MaybeUninit::<UserSpaceBuffer>::uninit();
                match ocall_allocate(user_buffer.as_mut_ptr(), output.as_ptr(), output.len()) {
                    sgx_status_t::SGX_SUCCESS => { /* continue */ }
                    _ => {
                        return InitResult::Failure {
                            err: EnclaveError::FailedOcall,
                        }
                    }
                }
                user_buffer.assume_init()
            };
            InitResult::Success {
                output: user_buffer,
                used_gas,
                signature,
            }
        }
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
    pub signature: [u8; 64],
}

pub fn result_handle_success_to_handleresult(
    result: Result<HandleSuccess, EnclaveError>,
) -> HandleResult {
    match result {
        Ok(HandleSuccess {
            output,
            used_gas,
            signature,
        }) => {
            let user_buffer = unsafe {
                let mut user_buffer = std::mem::MaybeUninit::<UserSpaceBuffer>::uninit();
                match ocall_allocate(user_buffer.as_mut_ptr(), output.as_ptr(), output.len()) {
                    sgx_status_t::SGX_SUCCESS => { /* continue */ }
                    _ => {
                        return HandleResult::Failure {
                            err: EnclaveError::FailedOcall,
                        }
                    }
                }
                user_buffer.assume_init()
            };
            HandleResult::Success {
                output: user_buffer,
                used_gas,
                signature,
            }
        }
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
    pub signature: [u8; 64],
}

pub fn result_query_success_to_queryresult(
    result: Result<QuerySuccess, EnclaveError>,
) -> QueryResult {
    match result {
        Ok(QuerySuccess {
            output,
            used_gas,
            signature,
        }) => {
            let user_buffer = unsafe {
                let mut user_buffer = std::mem::MaybeUninit::<UserSpaceBuffer>::uninit();
                match ocall_allocate(user_buffer.as_mut_ptr(), output.as_ptr(), output.len()) {
                    sgx_status_t::SGX_SUCCESS => { /* continue */ }
                    _ => {
                        return QueryResult::Failure {
                            err: EnclaveError::FailedOcall,
                        }
                    }
                }
                user_buffer.assume_init()
            };
            QueryResult::Success {
                output: user_buffer,
                used_gas,
                signature,
            }
        }
        Err(err) => QueryResult::Failure { err },
    }
}
