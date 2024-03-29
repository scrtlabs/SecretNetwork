//! This file should be autogenerated based on the headers created from the .edl file.

use log::*;

use sgx_types::{sgx_enclave_id_t, sgx_status_t, SgxResult};

use enclave_ffi_types::{
    Ctx, EnclaveBuffer, HandleResult, InitResult, MigrateResult, QueryResult, UpdateAdminResult,
};

use crate::enclave::ENCLAVE_DOORBELL;

extern "C" {
    /// Copy a buffer into the enclave memory space, and receive an opaque pointer to it.
    pub fn ecall_allocate(
        eid: sgx_enclave_id_t,
        retval: *mut EnclaveBuffer,
        buffer: *const u8,
        length: usize,
    ) -> sgx_status_t;

    pub fn ecall_migrate(
        eid: sgx_enclave_id_t,
        retval: *mut MigrateResult,
        context: Ctx,
        gas_limit: u64,
        used_gas: *mut u64,
        contract: *const u8,
        contract_len: usize,
        env: *const u8,
        env_len: usize,
        msg: *const u8,
        msg_len: usize,
        sig_info: *const u8,
        sig_info_len: usize,
        admin: *const u8,
        admin_len: usize,
        admin_proof: *const u8,
        admin_proof_len: usize,
    ) -> sgx_status_t;

    pub fn ecall_update_admin(
        eid: sgx_enclave_id_t,
        retval: *mut UpdateAdminResult,
        env: *const u8,
        env_len: usize,
        sig_info: *const u8,
        sig_info_len: usize,
        current_admin: *const u8,
        current_admin_len: usize,
        current_admin_proof: *const u8,
        current_admin_proof_len: usize,
        new_admin: *const u8,
        new_admin_len: usize,
    ) -> sgx_status_t;

    /// Trigger the init method in a wasm contract
    pub fn ecall_init(
        eid: sgx_enclave_id_t,
        retval: *mut InitResult,
        context: Ctx,
        gas_limit: u64,
        used_gas: *mut u64,
        contract: *const u8,
        contract_len: usize,
        env: *const u8,
        env_len: usize,
        msg: *const u8,
        msg_len: usize,
        sig_info: *const u8,
        sig_info_len: usize,
        admin: *const u8,
        admin_len: usize,
    ) -> sgx_status_t;

    /// Trigger a handle method in a wasm contract
    pub fn ecall_handle(
        eid: sgx_enclave_id_t,
        retval: *mut HandleResult,
        context: Ctx,
        gas_limit: u64,
        used_gas: *mut u64,
        contract: *const u8,
        contract_len: usize,
        env: *const u8,
        env_len: usize,
        msg: *const u8,
        msg_len: usize,
        sig_info: *const u8,
        sig_info_len: usize,
        handle_type: u8,
    ) -> sgx_status_t;
}

extern "C" {
    /// Trigger a query method in a wasm contract
    pub fn ecall_query(
        eid: sgx_enclave_id_t,
        retval: *mut QueryResult,
        context: Ctx,
        gas_limit: u64,
        used_gas: *mut u64,
        contract: *const u8,
        contract_len: usize,
        env: *const u8,
        env_len: usize,
        msg: *const u8,
        msg_len: usize,
    ) -> sgx_status_t;
}

/// This is a safe wrapper for allocating buffers inside the enclave.
pub(super) fn allocate_enclave_buffer(buffer: &[u8]) -> SgxResult<EnclaveBuffer> {
    let ptr = buffer.as_ptr();
    let len = buffer.len();
    let mut enclave_buffer = EnclaveBuffer::default();

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        // This is always called from an ocall contxt, so we don't want to wait for
        // an new TCS. To do that, we say that our query depth is >1, e.g. 2
        .get_access(2)
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;

    let enclave_id = enclave_access_token
        .expect("If we got here, surely the enclave has been loaded")
        .geteid();

    trace!(
        target: module_path!(),
        "allocate_enclave_buffer() called with len: {:?} enclave_id: {:?}",
        len,
        enclave_id,
    );

    match unsafe { ecall_allocate(enclave_id, &mut enclave_buffer, ptr, len) } {
        sgx_status_t::SGX_SUCCESS => Ok(enclave_buffer),
        failure_status => Err(failure_status),
    }
}
