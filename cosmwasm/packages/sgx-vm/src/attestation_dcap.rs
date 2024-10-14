use core::mem;

use std::ptr::null_mut;
use std::time::{SystemTime, UNIX_EPOCH};
use std::{self, ptr};

use log::*;
use sgx_types::*;

#[cfg(not(test))]
#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa_params(
    p_qe_info: *mut sgx_target_info_t,
    p_quote_size: *mut u32,
) -> sgx_status_t {
    let mut ret = unsafe { sgx_qe_get_target_info(p_qe_info) };
    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("sgx_qe_get_target_info returned {}", ret);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    ret = unsafe { sgx_qe_get_quote_size(p_quote_size) };
    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("sgx_qe_get_quote_size returned {}", ret);
        return sgx_status_t::SGX_ERROR_BUSY;
    }

    unsafe {
        trace!("*QuoteSize = {}", *p_quote_size);
    }

    sgx_status_t::SGX_SUCCESS
}

#[cfg(not(test))]
#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa(
    p_report: *const sgx_report_t,
    p_quote: *mut u8,
    n_quote: u32,
) -> sgx_status_t {
    trace!("Entering ocall_get_quote_ecdsa");

    //let mut qe_target_info: sgx_target_info_t;
    //sgx_qe_get_target_info(&qe_target_info);

    let mut n_quote_act: u32 = 0;
    let mut ret = unsafe { sgx_qe_get_quote_size(&mut n_quote_act) };
    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("sgx_qe_get_quote_size returned {}", ret);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if n_quote_act > n_quote {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    ret = unsafe { sgx_qe_get_quote(p_report, n_quote, p_quote) };
    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("sgx_qe_get_quote returned {}", ret);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    sgx_status_t::SGX_SUCCESS
}

pub struct QlQveCollateral {
    pub tee_type: u32, // 0x00000000: SGX or 0x00000081: TDX
    pub pck_crl_issuer_chain_size: u32,
    pub root_ca_crl_size: u32,
    pub pck_crl_size: u32,
    pub tcb_info_issuer_chain_size: u32,
    pub tcb_info_size: u32,
    pub qe_identity_issuer_chain_size: u32,
    pub qe_identity_size: u32,
}

fn sgx_ql_qve_collateral_serialize(
    p_col: *const u8,
    n_col: u32,
    p_res: *mut u8,
    n_res: u32,
) -> u32 {
    if n_col < mem::size_of::<sgx_ql_qve_collateral_t>() as u32 {
        return 0;
    }

    unsafe {
        let p_ql_col = p_col as *const sgx_ql_qve_collateral_t;

        let size_extra = (*p_ql_col).pck_crl_issuer_chain_size
            + (*p_ql_col).root_ca_crl_size
            + (*p_ql_col).pck_crl_size
            + (*p_ql_col).tcb_info_issuer_chain_size
            + (*p_ql_col).tcb_info_size
            + (*p_ql_col).qe_identity_issuer_chain_size
            + (*p_ql_col).qe_identity_size;

        if n_col < mem::size_of::<sgx_ql_qve_collateral_t>() as u32 + size_extra {
            return 0;
        }

        let out_size: u32 = mem::size_of::<QlQveCollateral>() as u32 + size_extra;

        if n_res >= out_size {
            let x = QlQveCollateral {
                tee_type: (*p_ql_col).tee_type,
                pck_crl_issuer_chain_size: (*p_ql_col).pck_crl_issuer_chain_size,
                root_ca_crl_size: (*p_ql_col).root_ca_crl_size,
                pck_crl_size: (*p_ql_col).pck_crl_size,
                tcb_info_issuer_chain_size: (*p_ql_col).tcb_info_issuer_chain_size,
                tcb_info_size: (*p_ql_col).tcb_info_size,
                qe_identity_issuer_chain_size: (*p_ql_col).qe_identity_issuer_chain_size,
                qe_identity_size: (*p_ql_col).qe_identity_size,
            };

            ptr::copy_nonoverlapping(
                &x as *const QlQveCollateral as *const u8,
                p_res,
                mem::size_of::<QlQveCollateral>(),
            );
            let mut offs = mem::size_of::<QlQveCollateral>();

            ptr::copy_nonoverlapping(
                (*p_ql_col).pck_crl_issuer_chain as *const u8,
                p_res.add(offs),
                x.pck_crl_issuer_chain_size as usize,
            );
            offs += x.pck_crl_issuer_chain_size as usize;

            ptr::copy_nonoverlapping(
                (*p_ql_col).root_ca_crl as *const u8,
                p_res.add(offs),
                x.root_ca_crl_size as usize,
            );
            offs += x.root_ca_crl_size as usize;

            ptr::copy_nonoverlapping(
                (*p_ql_col).pck_crl as *const u8,
                p_res.add(offs),
                x.pck_crl_size as usize,
            );
            offs += x.pck_crl_size as usize;

            ptr::copy_nonoverlapping(
                (*p_ql_col).tcb_info_issuer_chain as *const u8,
                p_res.add(offs),
                x.tcb_info_issuer_chain_size as usize,
            );
            offs += x.tcb_info_issuer_chain_size as usize;

            ptr::copy_nonoverlapping(
                (*p_ql_col).tcb_info as *const u8,
                p_res.add(offs),
                x.tcb_info_size as usize,
            );
            offs += x.tcb_info_size as usize;

            ptr::copy_nonoverlapping(
                (*p_ql_col).qe_identity_issuer_chain as *const u8,
                p_res.add(offs),
                x.qe_identity_issuer_chain_size as usize,
            );
            offs += x.qe_identity_issuer_chain_size as usize;

            ptr::copy_nonoverlapping(
                (*p_ql_col).qe_identity as *const u8,
                p_res.add(offs),
                x.qe_identity_size as usize,
            );
        }

        out_size
    }
}

fn sgx_ql_qve_collateral_deserialize(p_ser: *const u8, n_ser: u32) -> sgx_ql_qve_collateral_t {
    let mut res = sgx_ql_qve_collateral_t {
        version: 0,
        tee_type: 0,
        pck_crl_issuer_chain: null_mut(),
        pck_crl_issuer_chain_size: 0,
        root_ca_crl: null_mut(),
        root_ca_crl_size: 0,
        pck_crl: null_mut(),
        pck_crl_size: 0,
        tcb_info_issuer_chain: null_mut(),
        tcb_info_issuer_chain_size: 0,
        tcb_info: null_mut(),
        tcb_info_size: 0,
        qe_identity_issuer_chain: null_mut(),
        qe_identity_issuer_chain_size: 0,
        qe_identity: null_mut(),
        qe_identity_size: 0,
    };

    if n_ser >= mem::size_of::<QlQveCollateral>() as u32 {
        unsafe {
            let p_ql_col = p_ser as *const QlQveCollateral;
            let size_extra = (*p_ql_col).pck_crl_issuer_chain_size
                + (*p_ql_col).root_ca_crl_size
                + (*p_ql_col).pck_crl_size
                + (*p_ql_col).tcb_info_issuer_chain_size
                + (*p_ql_col).tcb_info_size
                + (*p_ql_col).qe_identity_issuer_chain_size
                + (*p_ql_col).qe_identity_size;

            if n_ser >= mem::size_of::<QlQveCollateral>() as u32 + size_extra {
                res.version = 1; // PCK Cert chain is in the Quote.
                res.tee_type = (*p_ql_col).tee_type;
                res.pck_crl_issuer_chain_size = (*p_ql_col).pck_crl_issuer_chain_size;
                res.root_ca_crl_size = (*p_ql_col).root_ca_crl_size;
                res.pck_crl_size = (*p_ql_col).pck_crl_size;
                res.tcb_info_issuer_chain_size = (*p_ql_col).tcb_info_issuer_chain_size;
                res.tcb_info_size = (*p_ql_col).tcb_info_size;
                res.qe_identity_issuer_chain_size = (*p_ql_col).qe_identity_issuer_chain_size;
                res.qe_identity_size = (*p_ql_col).qe_identity_size;

                let mut offs = mem::size_of::<QlQveCollateral>();

                res.pck_crl_issuer_chain = p_ser.add(offs) as *mut i8;
                offs += res.pck_crl_issuer_chain_size as usize;

                res.root_ca_crl = p_ser.add(offs) as *mut i8;
                offs += res.root_ca_crl_size as usize;

                res.pck_crl = p_ser.add(offs) as *mut i8;
                offs += res.pck_crl_size as usize;

                res.tcb_info_issuer_chain = p_ser.add(offs) as *mut i8;
                offs += res.tcb_info_issuer_chain_size as usize;

                res.tcb_info = p_ser.add(offs) as *mut i8;
                offs += res.tcb_info_size as usize;

                res.qe_identity_issuer_chain = p_ser.add(offs) as *mut i8;
                offs += res.qe_identity_issuer_chain_size as usize;

                res.qe_identity = p_ser.add(offs) as *mut i8;
            }
        }
    };

    res // unreachable
}

#[cfg(not(test))]
#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa_collateral(
    p_quote: *const u8,
    n_quote: u32,
    p_col: *mut u8,
    n_col: u32,
    p_col_size: *mut u32,
) -> sgx_status_t {
    let mut p_col_my: *mut u8 = std::ptr::null_mut::<u8>();
    let mut n_col_my: u32 = 0;

    let ret = unsafe { tee_qv_get_collateral(p_quote, n_quote, &mut p_col_my, &mut n_col_my) };

    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("tee_qv_get_collateral returned {}", ret);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    unsafe {
        *p_col_size = sgx_ql_qve_collateral_serialize(p_col_my, n_col_my, p_col, n_col);

        tee_qv_free_collateral(p_col_my);
    };

    sgx_status_t::SGX_SUCCESS
}

#[cfg(not(test))]
#[no_mangle]
pub extern "C" fn ocall_verify_quote_ecdsa(
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
    let mut time_use_s: time_t = time_s;
    if time_s == 0 {
        time_use_s = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs() as time_t;
    }

    unsafe {
        let res0 = sgx_qv_set_enclave_load_policy(sgx_ql_request_policy_t::SGX_QL_PERSISTENT);
        if sgx_quote3_error_t::SGX_QL_SUCCESS != res0 {
            warn!("sgx_qv_set_enclave_load_policy: {}", res0);
        }

        let res1 = sgx_qv_get_quote_supplemental_data_size(p_supp_data_size);
        if sgx_quote3_error_t::SGX_QL_SUCCESS != res1 {
            warn!("sgx_qv_get_quote_supplemental_data_size: {}", res1);
        }

        if *p_supp_data_size > n_supp_data {
            warn!("supp data buf required: {}", *p_supp_data_size);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }

        (*p_qve_report_info).app_enclave_target_info = *p_target_info;

        let my_col = sgx_ql_qve_collateral_deserialize(p_col, n_col);

        let res2 = sgx_qv_verify_quote(
            p_quote,
            n_quote,
            &my_col,
            time_use_s,
            p_collateral_expiration_status,
            p_qv_result,
            p_qve_report_info,
            *p_supp_data_size,
            p_supp_data,
        );
        if sgx_quote3_error_t::SGX_QL_SUCCESS != res2 {
            warn!("sgx_qv_verify_quote: {}", res2);
        }

        *p_time_s = time_use_s;
    };

    sgx_status_t::SGX_SUCCESS
}

#[cfg(test)]
#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa_params(
    _p_qe_info: *mut sgx_target_info_t,
    _p_quote_size: *mut u32,
) -> sgx_status_t {
    sgx_status_t::SGX_ERROR_SERVICE_UNAVAILABLE
}

#[cfg(test)]
#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa(
    _p_report: *const sgx_report_t,
    _p_quote: *mut u8,
    _n_quote: u32,
) -> sgx_status_t {
    sgx_status_t::SGX_ERROR_SERVICE_UNAVAILABLE
}

#[cfg(test)]
#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa_collateral(
    _p_quote: *const u8,
    _n_quote: u32,
    _p_col: *mut u8,
    _n_col: u32,
    _p_col_size: *mut u32,
) -> sgx_status_t {
    sgx_status_t::SGX_ERROR_SERVICE_UNAVAILABLE
}

#[cfg(test)]
#[no_mangle]
pub extern "C" fn ocall_verify_quote_ecdsa(
    _p_quote: *const u8,
    _n_quote: u32,
    _p_col: *const u8,
    _n_col: u32,
    _p_target_info: *const sgx_target_info_t,
    _time_s: i64,
    _p_qve_report_info: *mut sgx_ql_qe_report_info_t,
    _p_supp_data: *mut u8,
    _n_supp_data: u32,
    _p_supp_data_size: *mut u32,
    _p_time_s: *mut i64,
    _p_collateral_expiration_status: *mut u32,
    _p_qv_result: *mut sgx_ql_qv_result_t,
) -> sgx_status_t {
    sgx_status_t::SGX_ERROR_SERVICE_UNAVAILABLE
}
