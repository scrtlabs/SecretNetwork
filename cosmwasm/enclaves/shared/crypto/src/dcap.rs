#[cfg(feature = "SGX_MODE_HW")]
use log::*;

use sgx_types::{sgx_ql_qv_result_t, sgx_status_t};

#[cfg(feature = "SGX_MODE_HW")]
use sgx_types::{
    sgx_isv_svn_t, sgx_ql_qe_report_info_t, sgx_quote3_error_t, sgx_self_target, sgx_target_info_t,
    sgx_tvl_verify_qve_report_and_identity,
};

#[cfg(feature = "SGX_MODE_HW")]
extern "C" {
    pub fn ocall_verify_quote_ecdsa(
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
    ) -> sgx_status_t;
}

#[cfg(not(feature = "SGX_MODE_HW"))]
pub fn verify_quote_any(
    _vec_quote: &[u8],
    _vec_coll: &[u8],
    _time_s: i64,
) -> Result<sgx_ql_qv_result_t, sgx_status_t> {
    Err(sgx_status_t::SGX_ERROR_NO_DEVICE)
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn verify_quote_any(
    vec_quote: &[u8],
    vec_coll: &[u8],
    time_s: i64,
) -> Result<sgx_ql_qv_result_t, sgx_status_t> {
    let mut qe_report: sgx_ql_qe_report_info_t = sgx_ql_qe_report_info_t::default();
    let mut p_supp: [u8; 5000] = [0; 5000];
    let mut n_supp: u32 = 0;
    let mut exp_time_s: i64 = 0;
    let mut exp_status: u32 = 0;
    let mut qv_result: sgx_ql_qv_result_t = sgx_ql_qv_result_t::default();
    let mut rt: sgx_status_t = sgx_status_t::default();

    let mut ti: sgx_target_info_t = sgx_target_info_t::default();
    unsafe { sgx_self_target(&mut ti) };

    let res = unsafe {
        ocall_verify_quote_ecdsa(
            &mut rt as *mut sgx_status_t,
            vec_quote.as_ptr(),
            vec_quote.len() as u32,
            vec_coll.as_ptr(),
            vec_coll.len() as u32,
            &ti,
            time_s,
            &mut qe_report,
            p_supp.as_mut_ptr(),
            p_supp.len() as u32,
            &mut n_supp,
            &mut exp_time_s,
            &mut exp_status,
            &mut qv_result,
        )
    };

    if res != sgx_status_t::SGX_SUCCESS {
        return Err(res);
    }
    if rt != sgx_status_t::SGX_SUCCESS {
        return Err(rt);
    }

    match qv_result {
        sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK => {}
        sgx_ql_qv_result_t::SGX_QL_QV_RESULT_SW_HARDENING_NEEDED => {}
        _ => {
            trace!("Quote verification result: {}", qv_result);
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    // verify the qve report
    if time_s != 0 {
        exp_time_s = time_s; // insist on our time, if supplied
    }

    let qve_isvsvn_threshold: sgx_isv_svn_t = 3;
    let dcap_ret: sgx_quote3_error_t = unsafe {
        sgx_tvl_verify_qve_report_and_identity(
            vec_quote.as_ptr(),
            vec_quote.len() as u32,
            &qe_report,
            exp_time_s,
            exp_status,
            qv_result,
            p_supp.as_ptr(),
            n_supp,
            qve_isvsvn_threshold,
        )
    };

    if dcap_ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("QVE report verification result: {}", dcap_ret);
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    trace!("n_supp = {}", n_supp);
    trace!("exp_time_s = {}", exp_time_s);
    trace!("exp_status = {}", exp_status);
    trace!("qv_result = {}", qv_result);

    if exp_status != 0 {
        trace!("DCAP Collateral expired");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    Ok(qv_result)
}
