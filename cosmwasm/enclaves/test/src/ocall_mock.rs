use std::{
    net::{SocketAddr, TcpStream},
    os::unix::prelude::IntoRawFd,
};

use enclave_ffi_types::{Ctx, EnclaveBuffer, OcallReturn, UntrustedVmError, UserSpaceBuffer};
use sgx_types::{
    c_int, sgx_enclave_id_t, sgx_epid_group_id_t, sgx_platform_info_t, sgx_ql_qe_report_info_t,
    sgx_ql_qv_result_t, sgx_quote_nonce_t, sgx_quote_sign_type_t, sgx_quote_t, sgx_report_t,
    sgx_spid_t, sgx_status_t, sgx_target_info_t, sgx_update_info_bit_t,
};

// ecalls

// extern "C" {
//     pub fn ecall_get_attestation_report(
//         eid: sgx_enclave_id_t,
//         retval: *mut sgx_status_t,
//         dry_run: u8,
//     ) -> sgx_status_t;
// }

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

#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa_params(
    ret_val: *mut sgx_status_t,
    p_qe_info: *mut sgx_target_info_t,
    p_quote_size: *mut u32,
) -> sgx_status_t {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa(
    ret_val: *mut sgx_status_t,
    p_report: *const sgx_report_t,
    p_quote: *mut u8,
    n_quote: u32,
) -> sgx_status_t {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa_collateral(
    ret_val: *mut sgx_status_t,
    p_quote: *const u8,
    n_quote: u32,
    p_col: *mut u8,
    n_col: u32,
    p_col_out: *mut u32,
) -> sgx_status_t {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_verify_quote_ecdsa(
    ret_val: *mut sgx_status_t,
    p_quote: *const u8,
    n_quote: u32,
    p_col: *const u8,
    n_col: u32,
    p_target_info: *const sgx_target_info_t,
    time_s: i64,
    p_qve_report_info: *mut sgx_ql_qe_report_info_t,
    p_supp_data: *mut u8,
    n_supp_data: u32,
    p_supp_data_size: *mut u32,
    p_time_s: *mut i64,
    p_collateral_expiration_status: *mut u32,
    p_qv_result: *mut sgx_ql_qv_result_t,
) -> sgx_status_t {
    unimplemented!()
}
