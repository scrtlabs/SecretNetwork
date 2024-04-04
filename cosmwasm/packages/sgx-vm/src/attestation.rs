use core::mem;
use std::net::{SocketAddr, TcpStream};
use std::os::unix::io::IntoRawFd;

use std::ptr::null_mut;
use std::time::{SystemTime, UNIX_EPOCH};
use std::{self, ptr};

use log::*;
use sgx_types::*;
use sgx_types::{sgx_ql_qve_collateral_t, sgx_status_t, SgxResult};

use enclave_ffi_types::{NodeAuthResult, OUTPUT_ENCRYPTED_SEED_SIZE, SINGLE_ENCRYPTED_SEED_SIZE};

use crate::enclave::ENCLAVE_DOORBELL;

extern "C" {
    pub fn ecall_get_attestation_report(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        api_key: *const u8,
        api_key_len: u32,
        flags: u32,
    ) -> sgx_status_t;
    pub fn ecall_authenticate_new_node(
        eid: sgx_enclave_id_t,
        retval: *mut NodeAuthResult,
        cert: *const u8,
        cert_len: u32,
        seed: &mut [u8; OUTPUT_ENCRYPTED_SEED_SIZE as usize],
    ) -> sgx_status_t;
    pub fn ecall_get_genesis_seed(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        pk: *const u8,
        pk_len: u32,
        seed: &mut [u8; SINGLE_ENCRYPTED_SEED_SIZE as usize],
    ) -> sgx_status_t;
}

#[no_mangle]
pub extern "C" fn ocall_sgx_init_quote(
    ret_ti: *mut sgx_target_info_t,
    ret_gid: *mut sgx_epid_group_id_t,
) -> sgx_status_t {
    trace!("Entering ocall_sgx_init_quote");
    unsafe { sgx_init_quote(ret_ti, ret_gid) }
}

pub fn lookup_ipv4(host: &str, port: u16) -> SocketAddr {
    use std::net::ToSocketAddrs;

    let addrs = (host, port).to_socket_addrs().unwrap();
    for addr in addrs {
        if let SocketAddr::V4(_) = addr {
            return addr;
        }
    }

    unreachable!("Cannot lookup address");
}

#[no_mangle]
pub extern "C" fn ocall_get_ias_socket(ret_fd: *mut c_int) -> sgx_status_t {
    let port = 443;
    let hostname = "api.trustedservices.intel.com";
    let addr = lookup_ipv4(hostname, port);
    let sock = TcpStream::connect(&addr).expect("[-] Connect tls server failed!");

    unsafe {
        *ret_fd = sock.into_raw_fd();
    }

    sgx_status_t::SGX_SUCCESS
}

#[no_mangle]
pub extern "C" fn ocall_get_sn_tss_socket(ret_fd: *mut c_int) -> sgx_status_t {
    let port = 443;
    let hostname = "secretnetwork.trustedservices.scrtlabs.com";
    let addr = lookup_ipv4(hostname, port);
    let sock = TcpStream::connect(&addr).expect("[-] Connect tls server failed!");

    unsafe {
        *ret_fd = sock.into_raw_fd();
    }

    sgx_status_t::SGX_SUCCESS
}

#[cfg(not(test))]
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
) -> sgx_status_t {
    trace!("Entering ocall_get_quote");

    let mut real_quote_len: u32 = 0;

    let ret = unsafe { sgx_calc_quote_size(p_sigrl, sigrl_len, &mut real_quote_len as *mut u32) };

    if ret != sgx_status_t::SGX_SUCCESS {
        trace!("sgx_calc_quote_size returned {}", ret);
        return ret;
    }

    trace!("quote size = {}", real_quote_len);
    unsafe {
        *p_quote_len = real_quote_len;
    }

    let ret = unsafe {
        sgx_get_quote(
            p_report,
            quote_type,
            p_spid,
            p_nonce,
            p_sigrl,
            sigrl_len,
            p_qe_report,
            p_quote as *mut sgx_quote_t,
            real_quote_len,
        )
    };

    if ret != sgx_status_t::SGX_SUCCESS {
        trace!("sgx_calc_quote_size returned {}", ret);
        return ret;
    }

    trace!("sgx_calc_quote_size returned {}", ret);
    ret
}

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
) -> sgx_status_t
{
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
    pub qe_identity_size: u32
}

fn sgx_ql_qve_collateral_serialize(
    p_col: *const u8,
    n_col: u32,
    p_res: *mut u8,
    n_res: u32,
) -> u32
{
    if n_col < mem::size_of::<sgx_ql_qve_collateral_t>() as u32 {
        return 0;
    }

    unsafe {
        let p_ql_col = p_col as *const sgx_ql_qve_collateral_t;

        let size_extra =
            (*p_ql_col).pck_crl_issuer_chain_size +
            (*p_ql_col).root_ca_crl_size +
            (*p_ql_col).pck_crl_size +
            (*p_ql_col).tcb_info_issuer_chain_size +
            (*p_ql_col).tcb_info_size +
            (*p_ql_col).qe_identity_issuer_chain_size +
            (*p_ql_col).qe_identity_size
            ;

        if n_col < mem::size_of::<sgx_ql_qve_collateral_t>() as u32 + size_extra {
            return 0;
        }

        let out_size: u32 = mem::size_of::<QlQveCollateral>() as u32 + size_extra;

        if n_res >= out_size {

            let x = QlQveCollateral {
                tee_type : (*p_ql_col).tee_type,
                pck_crl_issuer_chain_size : (*p_ql_col).pck_crl_issuer_chain_size,
                root_ca_crl_size : (*p_ql_col).root_ca_crl_size,
                pck_crl_size : (*p_ql_col).pck_crl_size,
                tcb_info_issuer_chain_size : (*p_ql_col).tcb_info_issuer_chain_size,
                tcb_info_size : (*p_ql_col).tcb_info_size,
                qe_identity_issuer_chain_size : (*p_ql_col).qe_identity_issuer_chain_size,
                qe_identity_size : (*p_ql_col).qe_identity_size
            };

            ptr::copy_nonoverlapping(&x as *const QlQveCollateral as *const u8, p_res, mem::size_of::<QlQveCollateral>());
            let mut offs = mem::size_of::<QlQveCollateral>();

            ptr::copy_nonoverlapping((*p_ql_col).pck_crl_issuer_chain as *const u8, p_res.add(offs), x.pck_crl_issuer_chain_size as usize);
            offs += x.pck_crl_issuer_chain_size as usize;

            ptr::copy_nonoverlapping((*p_ql_col).root_ca_crl as *const u8, p_res.add(offs), x.root_ca_crl_size as usize);
            offs += x.root_ca_crl_size as usize;

            ptr::copy_nonoverlapping((*p_ql_col).pck_crl as *const u8, p_res.add(offs), x.pck_crl_size as usize);
            offs += x.pck_crl_size as usize;

            ptr::copy_nonoverlapping((*p_ql_col).tcb_info_issuer_chain as *const u8, p_res.add(offs), x.tcb_info_issuer_chain_size as usize);
            offs += x.tcb_info_issuer_chain_size as usize;

            ptr::copy_nonoverlapping((*p_ql_col).tcb_info as *const u8, p_res.add(offs), x.tcb_info_size as usize);
            offs += x.tcb_info_size as usize;

            ptr::copy_nonoverlapping((*p_ql_col).qe_identity_issuer_chain as *const u8, p_res.add(offs), x.qe_identity_issuer_chain_size as usize);
            offs += x.qe_identity_issuer_chain_size as usize;

            ptr::copy_nonoverlapping((*p_ql_col).qe_identity as *const u8, p_res.add(offs), x.qe_identity_size as usize);
        }

        return out_size;
    };
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
        qe_identity_size: 0
    };

    if n_ser >= mem::size_of::<QlQveCollateral>() as u32 {

        unsafe {
            let p_ql_col = p_ser as *const QlQveCollateral;
            let size_extra =
                (*p_ql_col).pck_crl_issuer_chain_size +
                (*p_ql_col).root_ca_crl_size +
                (*p_ql_col).pck_crl_size +
                (*p_ql_col).tcb_info_issuer_chain_size +
                (*p_ql_col).tcb_info_size +
                (*p_ql_col).qe_identity_issuer_chain_size +
                (*p_ql_col).qe_identity_size
                ;

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

#[cfg(not(test))]
#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa_collateral(
    p_quote: *const u8,
    n_quote: u32,
    p_col: *mut u8,
    n_col: u32,
    p_col_size: *mut u32
) -> sgx_status_t
{
    let mut p_col_my : *mut u8 = 0 as *mut u8;
    let mut n_col_my : u32 = 0;

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
        sgx_qv_set_enclave_load_policy(sgx_ql_request_policy_t::SGX_QL_PERSISTENT);
        sgx_qv_get_quote_supplemental_data_size(p_supp_data_size);

        if *p_supp_data_size > n_supp_data {
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }

        (*p_qve_report_info).app_enclave_target_info = *p_target_info;

        let my_col = sgx_ql_qve_collateral_deserialize(p_col, n_col);

        sgx_qv_verify_quote(
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

        *p_time_s = time_use_s;
    };

    sgx_status_t::SGX_SUCCESS
}

#[cfg(test)]
#[no_mangle]
pub extern "C" fn ocall_get_quote(
    _p_sigrl: *const u8,
    _sigrl_len: u32,
    _p_report: *const sgx_report_t,
    _quote_type: sgx_quote_sign_type_t,
    _p_spid: *const sgx_spid_t,
    _p_nonce: *const sgx_quote_nonce_t,
    _p_qe_report: *mut sgx_report_t,
    _p_quote: *mut u8,
    _maxlen: u32,
    _p_quote_len: *mut u32,
) -> sgx_status_t {
    sgx_status_t::SGX_ERROR_SERVICE_UNAVAILABLE
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


#[no_mangle]
pub extern "C" fn ocall_get_update_info(
    platform_blob: *const sgx_platform_info_t,
    enclave_trusted: i32,
    update_info: *mut sgx_update_info_bit_t,
) -> sgx_status_t {
    unsafe { sgx_report_attestation_status(platform_blob, enclave_trusted, update_info) }
}

pub fn create_attestation_report_u(api_key: &[u8], flags: u32) -> SgxResult<()> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_get_attestation_report(eid, &mut retval, api_key.as_ptr(), api_key.len() as u32, flags)
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(())
}

pub fn untrusted_get_encrypted_seed(
    cert: &[u8],
) -> SgxResult<Result<[u8; OUTPUT_ENCRYPTED_SEED_SIZE as usize], NodeAuthResult>> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;
    let eid = enclave.geteid();
    let mut retval = NodeAuthResult::Success;

    let mut seed = [0u8; OUTPUT_ENCRYPTED_SEED_SIZE as usize];
    let status = unsafe {
        ecall_authenticate_new_node(
            eid,
            &mut retval,
            cert.as_ptr(),
            cert.len() as u32,
            &mut seed,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        debug!("Error from authenticate new node");
        return Err(status);
    }

    if retval != NodeAuthResult::Success {
        debug!("Error from authenticate new node, bad NodeAuthResult");
        return Ok(Err(retval));
    }

    debug!("Done auth, got seed: {:?}", seed);

    if seed.is_empty() {
        error!("Got empty seed from encryption");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    Ok(Ok(seed))
}

pub fn untrusted_get_encrypted_genesis_seed(
    pk: &[u8],
) -> SgxResult<[u8; SINGLE_ENCRYPTED_SEED_SIZE as usize]> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;
    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;

    let mut seed = [0u8; SINGLE_ENCRYPTED_SEED_SIZE as usize];
    let status = unsafe {
        ecall_get_genesis_seed(eid, &mut retval, pk.as_ptr(), pk.len() as u32, &mut seed)
    };

    if status != sgx_status_t::SGX_SUCCESS {
        debug!("Error from get genesis seed");
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        debug!("Error from get genesis seed, bad NodeAuthResult");
        return Err(retval);
    }

    debug!("Done getting genesis seed, got seed: {:?}", seed);

    if seed.is_empty() {
        error!("Got empty seed from encryption");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    Ok(seed)
}

#[cfg(test)]
mod test {
    // use crate::attestation::retry_quote;
    // use crate::esgx::general::init_enclave_wrapper;
    // use crate::instance::init_enclave as init_enclave_wrapper;

    // isans SPID = "3DDB338BD52EE314B01F1E4E1E84E8AA"
    // victors spid = 68A8730E9ABF1829EA3F7A66321E84D0
    //const SPID: &str = "B0335FD3BC1CCA8F804EB98A6420592D";

    // #[test]
    // fn test_produce_quote() {
    //     // initiate the enclave
    //     let enclave = init_enclave_wrapper().unwrap();
    //     // produce a quote
    //
    //     let tested_encoded_quote = match retry_quote(enclave.geteid(), &SPID, 18) {
    //         Ok(encoded_quote) => encoded_quote,
    //         Err(e) => {
    //             error!("Produce quote Err {}, {}", e.as_fail(), e.backtrace());
    //             assert_eq!(0, 1);
    //             return;
    //         }
    //     };
    //     debug!("-------------------------");
    //     debug!("{}", tested_encoded_quote);
    //     debug!("-------------------------");
    //     enclave.destroy();
    //     assert!(!tested_encoded_quote.is_empty());
    //     // assert_eq!(real_encoded_quote, tested_encoded_quote);
    // }

    // #[test]
    // fn test_produce_and_verify_qoute() {
    //     let enclave = init_enclave_wrapper().unwrap();
    //     let quote = retry_quote(enclave.geteid(), &SPID, 18).unwrap();
    //     let service = AttestationService::new(attestation_service::constants::ATTESTATION_SERVICE_URL);
    //     let as_response = service.get_report(quote).unwrap();
    //
    //     assert!(as_response.result.verify_report().unwrap());
    // }
    //
    // #[test]
    // fn test_signing_key_against_quote() {
    //     let enclave = init_enclave_wrapper().unwrap();
    //     let quote = retry_quote(enclave.geteid(), &SPID, 18).unwrap();
    //     let service = AttestationService::new(attestation_service::constants::ATTESTATION_SERVICE_URL);
    //     let as_response = service.get_report(quote).unwrap();
    //     assert!(as_response.result.verify_report().unwrap());
    //     let key = super::get_register_signing_address(enclave.geteid()).unwrap();
    //     let quote = as_response.get_quote().unwrap();
    //     assert_eq!(key, &quote.report_body.report_data[..20]);
    // }
}
