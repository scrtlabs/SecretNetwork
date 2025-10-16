use sgx_types::{
    sgx_report_t, sgx_status_t, sgx_target_info_t,
};

extern "C" {
    pub fn ocall_get_quote_ecdsa_params(
        ret_val: *mut sgx_status_t,
        p_qe_info: *mut sgx_target_info_t,
        p_quote_size: *mut u32,
    ) -> sgx_status_t;
    pub fn ocall_get_quote_ecdsa(
        ret_val: *mut sgx_status_t,
        p_report: *const sgx_report_t,
        p_quote: *mut u8,
        n_quote: u32,
    ) -> sgx_status_t;
    pub fn ocall_get_quote_ecdsa_collateral(
        ret_val: *mut sgx_status_t,
        p_quote: *const u8,
        n_quote: u32,
        p_col: *mut u8,
        n_col: u32,
        p_col_out: *mut u32,
    ) -> sgx_status_t;
}
