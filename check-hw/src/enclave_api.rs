use enclave_ffi_types::{
    Ctx, EnclaveBuffer, NodeAuthResult, OcallReturn, UntrustedVmError, UserSpaceBuffer,
};
use sgx_types::{
    sgx_enclave_id_t,
    sgx_ql_qe_report_info_t, sgx_ql_qv_result_t,
    sgx_report_t, sgx_status_t, sgx_target_info_t,
};

include!("../../cosmwasm/packages/sgx-vm/src/attestation_dcap.rs");

// ecalls

extern "C" {
    pub fn ecall_check_patch_level(
        eid: sgx_enclave_id_t,
        retval: *mut NodeAuthResult,
        p_ppid: *mut u8,
        n_ppid: u32,
        p_ppid_size: *mut u32,
    ) -> sgx_status_t;

    pub fn ecall_migration_op(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        opcode: u32,
    ) -> sgx_status_t;
}

// ocalls

#[no_mangle]
pub extern "C" fn ocall_write_db(
    _context: Ctx,
    _vm_error: *mut UntrustedVmError,
    _gas_used: *mut u64,
    _key: *const u8,
    _key_len: usize,
    _value: *const u8,
    _value_len: usize,
) -> OcallReturn {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_multiple_write_db(
    _context: Ctx,
    _vm_error: *mut UntrustedVmError,
    _gas_used: *mut u64,
    _keys: *const u8,
    _keys_len: usize,
) -> OcallReturn {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_remove_db(
    _context: Ctx,
    _vm_error: *mut UntrustedVmError,
    _gas_used: *mut u64,
    _key: *const u8,
    _key_len: usize,
) -> OcallReturn {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_query_chain(
    _context: Ctx,
    _vm_error: *mut UntrustedVmError,
    _gas_used: *mut u64,
    _gas_limit: u64,
    _value: *mut EnclaveBuffer,
    _query: *const u8,
    _query_len: usize,
    _query_depth: u32,
) -> OcallReturn {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_read_db(
    _context: Ctx,
    _vm_error: *mut UntrustedVmError,
    _gas_used: *mut u64,
    _value: *mut EnclaveBuffer,
    _key: *const u8,
    _key_len: usize,
) -> OcallReturn {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_allocate(_buffer: *const u8, _length: usize) -> UserSpaceBuffer {
    unimplemented!()
}
