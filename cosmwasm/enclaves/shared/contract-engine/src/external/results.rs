use sgx_types::sgx_status_t;

use enclave_ffi_types::{
    EnclaveError, HandleResult, InitResult, MigrateResult, QueryResult, UntrustedVmError,
    UpdateAdminResult, UserSpaceBuffer,
};

use crate::external::ocalls::ocall_allocate;

/// This struct is returned from module initialization.
pub struct InitSuccess {
    /// The output of the calculation
    pub output: Vec<u8>,
    /// The contract_key of this contract.
    pub contract_key: [u8; 64],
    pub admin_proof: [u8; 32],
}

pub fn result_init_success_to_initresult(result: Result<InitSuccess, EnclaveError>) -> InitResult {
    match result {
        Ok(InitSuccess {
            output,
            contract_key,
            admin_proof,
        }) => {
            let user_buffer = unsafe {
                let mut user_buffer = std::mem::MaybeUninit::<UserSpaceBuffer>::uninit();
                match ocall_allocate(user_buffer.as_mut_ptr(), output.as_ptr(), output.len()) {
                    sgx_status_t::SGX_SUCCESS => { /* continue */ }
                    _ => {
                        return InitResult::Failure {
                            err: EnclaveError::FailedOcall {
                                vm_error: UntrustedVmError::default(),
                            },
                        }
                    }
                }
                user_buffer.assume_init()
            };
            InitResult::Success {
                output: user_buffer,
                contract_key,
                admin_proof,
            }
        }
        Err(err) => InitResult::Failure { err },
    }
}

/// This struct is returned from a handle method.
pub struct HandleSuccess {
    /// The output of the calculation
    pub output: Vec<u8>,
}

pub fn result_handle_success_to_handleresult(
    result: Result<HandleSuccess, EnclaveError>,
) -> HandleResult {
    match result {
        Ok(HandleSuccess { output }) => {
            let user_buffer = unsafe {
                let mut user_buffer = std::mem::MaybeUninit::<UserSpaceBuffer>::uninit();
                match ocall_allocate(user_buffer.as_mut_ptr(), output.as_ptr(), output.len()) {
                    sgx_status_t::SGX_SUCCESS => { /* continue */ }
                    _ => {
                        return HandleResult::Failure {
                            err: EnclaveError::FailedOcall {
                                vm_error: UntrustedVmError::default(),
                            },
                        }
                    }
                }
                user_buffer.assume_init()
            };
            HandleResult::Success {
                output: user_buffer,
            }
        }
        Err(err) => HandleResult::Failure { err },
    }
}

/// This struct is returned from a migrate method.
pub struct MigrateSuccess {
    /// The output of the calculation
    pub output: Vec<u8>,
    pub new_contract_key: [u8; 64],
    pub new_contract_key_proof: [u8; 32],
}

pub fn result_migrate_success_to_result(
    result: Result<MigrateSuccess, EnclaveError>,
) -> MigrateResult {
    match result {
        Ok(MigrateSuccess {
            output,
            new_contract_key,
            new_contract_key_proof,
        }) => {
            let user_buffer = unsafe {
                let mut user_buffer = std::mem::MaybeUninit::<UserSpaceBuffer>::uninit();
                match ocall_allocate(user_buffer.as_mut_ptr(), output.as_ptr(), output.len()) {
                    sgx_status_t::SGX_SUCCESS => { /* continue */ }
                    _ => {
                        return MigrateResult::Failure {
                            err: EnclaveError::FailedOcall {
                                vm_error: UntrustedVmError::default(),
                            },
                        }
                    }
                }
                user_buffer.assume_init()
            };
            MigrateResult::Success {
                output: user_buffer,
                new_contract_key,
                new_contract_key_proof,
            }
        }
        Err(err) => MigrateResult::Failure { err },
    }
}

/// This struct is returned from a migrate method.
pub struct UpdateAdminSuccess {
    pub new_admin_proof: [u8; 32],
}

pub fn result_update_admin_success_to_result(
    result: Result<UpdateAdminSuccess, EnclaveError>,
) -> UpdateAdminResult {
    match result {
        Ok(UpdateAdminSuccess { new_admin_proof }) => {
            UpdateAdminResult::UpdateAdminSuccess { new_admin_proof }
        }
        Err(err) => UpdateAdminResult::UpdateAdminFailure { err },
    }
}

/// This struct is returned from a query method.
pub struct QuerySuccess {
    /// The output of the calculation
    pub output: Vec<u8>,
}

pub fn result_query_success_to_queryresult(
    result: Result<QuerySuccess, EnclaveError>,
) -> QueryResult {
    match result {
        Ok(QuerySuccess { output }) => {
            let user_buffer = unsafe {
                let mut user_buffer = std::mem::MaybeUninit::<UserSpaceBuffer>::uninit();
                match ocall_allocate(user_buffer.as_mut_ptr(), output.as_ptr(), output.len()) {
                    sgx_status_t::SGX_SUCCESS => { /* continue */ }
                    _ => {
                        return QueryResult::Failure {
                            err: EnclaveError::FailedOcall {
                                vm_error: UntrustedVmError::default(),
                            },
                        }
                    }
                }
                user_buffer.assume_init()
            };
            QueryResult::Success {
                output: user_buffer,
            }
        }
        Err(err) => QueryResult::Failure { err },
    }
}
