use sgx_types::{sgx_report_t, sgx_status_t, sgx_target_info_t};

extern "C" {
    pub fn ocall_sgx_get_target_info(
        ret_val: *mut sgx_status_t,
        ret_ti: *mut sgx_target_info_t,
    ) -> sgx_status_t;

    pub fn ocall_get_quote_dcap(
        ret_val: *mut sgx_status_t,
        report: *const sgx_report_t,
        maxlen: u32,
        ret_quote: *mut u8,
        ret_quote_len: *mut u32,
    ) -> sgx_status_t;
}
