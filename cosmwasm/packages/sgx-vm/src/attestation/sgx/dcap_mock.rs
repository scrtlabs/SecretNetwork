/// this file exists to be able to link go-cosmwasm without dcap support (since the enclave will expect these calls to exist)
use sgx_types::{sgx_report_t, sgx_status_t, sgx_target_info_t};

#[no_mangle]
#[allow(unused_variables)]
pub extern "C" fn ocall_get_target_info(ret_target_info: *mut sgx_target_info_t) -> sgx_status_t {
    panic!("Should never get here if dcap feature is not enabled")
}

#[no_mangle]
#[allow(unused_variables)]
pub extern "C" fn ocall_get_quote_dcap(
    p_qe_report: *const sgx_report_t,
    ret_quote: *mut u8,
    maxlen: u32,
    ret_quote_len: *mut u32,
) -> sgx_status_t {
    panic!("Should never get here if dcap feature is not enabled")
}
