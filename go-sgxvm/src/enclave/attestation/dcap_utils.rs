use std::io::{Read, Write};
use std::{mem, ptr};

use crate::enclave;
use crate::errors::Error;
use crate::types;
use sgx_types::*;

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

/// Returns target info from Quoting Enclave (QE)
pub fn get_qe_target_info() -> Result<sgx_target_info_t, Error> {
    let mut qe_target_info = sgx_target_info_t::default();
    let qe3_ret = unsafe { sgx_qe_get_target_info(&mut qe_target_info) };
    if qe3_ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        println!(
            "[Enclave Wrapper] sgx_qe_get_target_info failed. Status code: {:?}",
            qe3_ret
        );
        return Err(Error::enclave_error("sgx_qe_get_target_info failed"));
    }

    Ok(qe_target_info)
}

/// Returns size of buffer to allocate for the quote
pub fn get_quote_size() -> Result<u32, Error> {
    let mut quote_size = 0u32;
    let qe3_ret = unsafe { sgx_qe_get_quote_size(&mut quote_size) };
    if qe3_ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        println!(
            "[Enclave Wrapper] sgx_qe_get_quote_size failed. Status code: {:?}",
            qe3_ret
        );
        return Err(Error::enclave_error("sgx_qe_get_quote_size failed"));
    }

    Ok(quote_size)
}

/// Returns DCAP quote from QE
pub fn get_qe_quote(report: sgx_report_t, quote_size: u32, p_quote: *mut u8) -> SgxResult<()> {
    println!("[Enclave Wrapper]: get_qe_quote");
    match unsafe { sgx_qe_get_quote(&report, quote_size, p_quote) } {
        sgx_quote3_error_t::SGX_QL_SUCCESS => Ok(()),
        err => {
            println!("Cannot get quote from QE. Status code: {:?}", err);
            SgxResult::Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
        }
    }
}

/// Generates quote inside the enclave and writes it to the file
/// Since this function will be used only for test and dev purposes,
/// we can ignore usages of `unwrap` or `expect`.
pub fn dump_dcap_quote(eid: sgx_enclave_id_t, filepath: &str) -> Result<(), Error> {
    let qe_target_info = get_qe_target_info()?;
    let quote_size = get_quote_size()?;
    let mut retval = std::mem::MaybeUninit::<types::AllocationWithResult>::uninit();

    let res = unsafe {
        enclave::ecall_dump_dcap_quote(eid, retval.as_mut_ptr(), &qe_target_info, quote_size)
    };

    if res != sgx_status_t::SGX_SUCCESS {
        panic!("Call to `ecall_dump_dcap_quote` failed. Reason: {:?}", res);
    }

    let quote_res = unsafe { retval.assume_init() };
    if quote_res.status != sgx_status_t::SGX_SUCCESS {
        panic!(
            "`ecall_dump_dcap_quote` returned error code: {:?}",
            quote_res.status
        );
    }

    let quote_vec = unsafe {
        Vec::from_raw_parts(
            quote_res.result_ptr,
            quote_res.result_size,
            quote_res.result_size,
        )
    };

    let mut quote_file =
        std::fs::File::create(filepath).expect("Cannot create file to write quote");

    quote_file
        .write_all(&quote_vec)
        .expect("Cannot write quote to file");

    Ok(())
}

pub fn verify_dcap_quote(eid: sgx_enclave_id_t, filepath: &str) -> Result<(), Error> {
    let mut file = std::fs::File::open(filepath).expect("Cannot open quote file");
    let mut quote_buf = Vec::new();
    let _ = file.read_to_end(&mut quote_buf)
        .expect("Cannot read quote file");

    let mut retval = sgx_status_t::SGX_ERROR_UNEXPECTED;
    let res = unsafe {
        enclave::ecall_verify_dcap_quote(
            eid,
            &mut retval,
            quote_buf.as_ptr(),
            quote_buf.len() as u32,
        )
    };

    if res != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Call to ecall_verify_dcap_quote failed. Reason: {:?}",
            res
        );
        return Err(Error::enclave_error(res));
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] ecall_verify_dcap_quote returned error code: {:?}",
            retval
        );
        return Err(Error::enclave_error(retval));
    }

    Ok(())
}

pub fn sgx_ql_qve_collateral_serialize(
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

        return out_size;
    };
}

pub fn sgx_ql_qve_collateral_deserialize(p_ser: *const u8, n_ser: u32) -> sgx_ql_qve_collateral_t {
    let mut res = sgx_ql_qve_collateral_t {
        version: 0,
        tee_type: 0,
        pck_crl_issuer_chain: std::ptr::null_mut(),
        pck_crl_issuer_chain_size: 0,
        root_ca_crl: std::ptr::null_mut(),
        root_ca_crl_size: 0,
        pck_crl: std::ptr::null_mut(),
        pck_crl_size: 0,
        tcb_info_issuer_chain: std::ptr::null_mut(),
        tcb_info_issuer_chain_size: 0,
        tcb_info: std::ptr::null_mut(),
        tcb_info_size: 0,
        qe_identity_issuer_chain: std::ptr::null_mut(),
        qe_identity_issuer_chain_size: 0,
        qe_identity: std::ptr::null_mut(),
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

    return res; // unreachable
}
