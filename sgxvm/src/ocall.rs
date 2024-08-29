/// This file contains signatures of `OCALL` functions
use crate::{querier::GoQuerier, Allocation, AllocationWithResult};
use sgx_types::sgx_status_t;
use sgx_types::*;

extern "C" {
    pub fn ocall_query_raw(
        ret_val: *mut AllocationWithResult,
        querier: *mut GoQuerier,
        request: *const u8,
        len: usize,
    ) -> sgx_status_t;

    pub fn ocall_allocate(ret_val: *mut Allocation, data: *const u8, len: usize) -> sgx_status_t;

    pub fn ocall_sgx_init_quote(
        ret_val: *mut sgx_status_t,
        ret_ti: *mut sgx_target_info_t,
        ret_gid: *mut sgx_epid_group_id_t,
    ) -> sgx_status_t;

    pub fn ocall_get_ias_socket(ret_val: *mut sgx_status_t, ret_fd: *mut i32) -> sgx_status_t;

    pub fn ocall_get_quote(
        ret_val: *mut sgx_status_t,
        p_sigrl: *const u8,
        sigrl_len: u32,
        p_report: *const sgx_report_t,
        quote_type: sgx_quote_sign_type_t,
        p_spid: *const sgx_spid_t,
        p_nonce: *const sgx_quote_nonce_t,
        p_qe_report: *mut sgx_report_t,
        p_quote: *mut u8,
        maxlen: u32,
        p_quote_len: *mut u32,
    ) -> sgx_status_t;

    pub fn ocall_get_ecdsa_quote(
		ret_val: *mut sgx_status_t,
		p_report: *const sgx_report_t,
		p_quote: *mut u8,
		quote_size: u32,
	) -> sgx_status_t;

    pub fn ocall_get_qve_report(
		ret_val: *mut sgx_status_t,
		p_quote: *const u8,
		quote_len: u32,
		timestamp: i64,
		p_collateral_expiration_status: *mut u32,
		p_quote_verification_result: *mut sgx_ql_qv_result_t,
		p_qve_report_info: *mut sgx_ql_qe_report_info_t,
		p_supplemental_data: *mut u8,
		supplemental_data_size: u32,
        p_collateral: *const u8,
        collateral_len: u32,
    ) -> sgx_status_t;

    pub fn ocall_get_supplemental_data_size(
        ret_val: *mut sgx_status_t,
        data_size: *mut u32,
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
