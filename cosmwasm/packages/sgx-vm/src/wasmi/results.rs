use super::exports;
use crate::VmResult;
use enclave_ffi_types::{HandleResult, InitResult, MigrateResult, QueryResult, UpdateAdminResult};

/// This struct is returned from module initialization.
pub struct InitSuccess {
    /// A pointer to the output of the execution
    output: Vec<u8>,
    /// The contract_key for this contract.
    contract_key: [u8; 64],
    admin_proof: [u8; 32],
}

impl InitSuccess {
    pub fn into_output(self) -> Vec<u8> {
        let mut out_vec = self.contract_key.to_vec();
        out_vec.extend_from_slice(&self.admin_proof);
        out_vec.extend_from_slice(&self.output);
        out_vec
    }
}

pub fn init_result_to_vm_result(other: InitResult) -> VmResult<InitSuccess> {
    match other {
        InitResult::Success {
            output,
            contract_key,
            admin_proof,
        } => Ok(InitSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(Vec::new),
            contract_key,
            admin_proof,
        }),
        InitResult::Failure { err } => Err(err.into()),
    }
}

pub fn migrate_result_to_vm_result(other: MigrateResult) -> VmResult<MigrateSuccess> {
    match other {
        MigrateResult::Success {
            output,
            new_contract_key,
            contract_key_proof,
        } => Ok(MigrateSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(Vec::new),
            new_contract_key,
            contract_key_proof,
        }),
        MigrateResult::Failure { err } => Err(err.into()),
    }
}

pub fn update_admin_result_to_vm_result(other: UpdateAdminResult) -> VmResult<UpdateAdminSuccess> {
    match other {
        UpdateAdminResult::Success { admin_proof } => Ok(UpdateAdminSuccess { admin_proof }),
        UpdateAdminResult::Failure { err } => Err(err.into()),
    }
}

/// This struct is returned from a migrate method.
pub struct MigrateSuccess {
    /// A pointer to the output of the execution
    output: Vec<u8>,
    new_contract_key: [u8; 64],
    contract_key_proof: [u8; 32],
}

impl MigrateSuccess {
    pub fn into_output(self) -> Vec<u8> {
        let mut out_vec = self.new_contract_key.to_vec();
        out_vec.extend_from_slice(&self.contract_key_proof);
        out_vec.extend_from_slice(&self.output);
        out_vec
    }
}

/// This struct is returned from a migrate method.
pub struct UpdateAdminSuccess {
    admin_proof: [u8; 32],
}

impl UpdateAdminSuccess {
    pub fn into_output(self) -> Vec<u8> {
        self.admin_proof.to_vec()
    }
}

/// This struct is returned from a handle method.
pub struct HandleSuccess {
    /// A pointer to the output of the execution
    output: Vec<u8>,
}

impl HandleSuccess {
    pub fn into_output(self) -> Vec<u8> {
        self.output
    }
}

pub fn handle_result_to_vm_result(other: HandleResult) -> VmResult<HandleSuccess> {
    match other {
        HandleResult::Success { output } => Ok(HandleSuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(Vec::new),
        }),
        HandleResult::Failure { err } => Err(err.into()),
    }
}

/// This struct is returned from a query method.
pub struct QuerySuccess {
    /// A pointer to the output of the execution
    output: Vec<u8>,
}

impl QuerySuccess {
    pub fn into_output(self) -> Vec<u8> {
        self.output
    }
}

pub fn query_result_to_vm_result(other: QueryResult) -> VmResult<QuerySuccess> {
    match other {
        QueryResult::Success { output } => Ok(QuerySuccess {
            output: unsafe { exports::recover_buffer(output) }.unwrap_or_else(Vec::new),
        }),
        QueryResult::Failure { err } => Err(err.into()),
    }
}
