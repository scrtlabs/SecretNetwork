use std::{
    net::{SocketAddr, TcpStream},
    os::unix::prelude::IntoRawFd,
};

use enclave_ffi_types::{Ctx, EnclaveBuffer, OcallReturn, UntrustedVmError, UserSpaceBuffer};
use sgx_types::{
    c_int, sgx_calc_quote_size, sgx_enclave_id_t, sgx_epid_group_id_t, sgx_get_quote,
    sgx_init_quote, sgx_platform_info_t, sgx_quote_nonce_t, sgx_quote_sign_type_t, sgx_quote_t,
    sgx_report_attestation_status, sgx_report_t, sgx_spid_t, sgx_status_t, sgx_target_info_t,
    sgx_update_info_bit_t,
};

// ecalls

// extern "C" {
//     pub fn ecall_get_attestation_report(
//         eid: sgx_enclave_id_t,
//         retval: *mut sgx_status_t,
//         api_key: *const u8,
//         api_key_len: u32,
//         dry_run: u8,
//     ) -> sgx_status_t;
// }

// ocalls

#[no_mangle]
pub extern "C" fn ocall_get_update_info(
    platform_blob: *const sgx_platform_info_t,
    enclave_trusted: i32,
    update_info: *mut sgx_update_info_bit_t,
) -> sgx_status_t  {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_get_quote(
    p_sigrl: *const u8,
    sigrl_len: u32,
    p_report: *const sgx_report_t,
    quote_type: sgx_quote_sign_type_t,
    p_spid: *const sgx_spid_t,
    p_nonce: *const sgx_quote_nonce_t,
    p_qe_report: *mut sgx_report_t,
    p_quote: *mut u8,
    _maxlen: u32,
    p_quote_len: *mut u32,
) -> sgx_status_t  {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_get_sn_tss_socket(_ret_fd: *mut c_int) -> sgx_status_t {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_get_ias_socket(ret_fd: *mut c_int) -> sgx_status_t  {
    unimplemented!()
}

pub fn lookup_ipv4(host: &str, port: u16) -> SocketAddr  {
    unimplemented!()
}

#[no_mangle]
pub extern "C" fn ocall_sgx_init_quote(
    ret_ti: *mut sgx_target_info_t,
    ret_gid: *mut sgx_epid_group_id_t,
) -> sgx_status_t {
    unimplemented!()
}

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
