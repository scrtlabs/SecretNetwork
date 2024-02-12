use std::net::{SocketAddr, TcpStream};
use std::os::unix::io::IntoRawFd;
use core::mem;

use std::{self, ptr};
use std::ptr::{null, null_mut};
use std::time::{SystemTime, UNIX_EPOCH};

use log::*;
use sgx_types::*;
use sgx_types::{sgx_status_t, SgxResult, sgx_ql_qve_collateral_t};

use enclave_ffi_types::{NodeAuthResult, OUTPUT_ENCRYPTED_SEED_SIZE, SINGLE_ENCRYPTED_SEED_SIZE};

use crate::enclave::ENCLAVE_DOORBELL;

extern "C" {
    pub fn ecall_get_attestation_report(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        api_key: *const u8,
        api_key_len: u32,
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

#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa_params(
    pQeInfo: *mut sgx_target_info_t,
    pQuoteSize: *mut u32
) -> sgx_status_t
{
    let mut ret = unsafe { sgx_qe_get_target_info(pQeInfo) };
    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("sgx_qe_get_target_info returned {}", ret);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    ret = unsafe { sgx_qe_get_quote_size(pQuoteSize) };
    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("sgx_qe_get_quote_size returned {}", ret);
        return sgx_status_t::SGX_ERROR_BUSY;
    }

    unsafe {
        trace!("*pQuoteSize = {}", *pQuoteSize);
    }

    sgx_status_t::SGX_SUCCESS
}

#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa(
    pReport: *const sgx_report_t,
    pQuote: *mut u8,
    nQuote: u32,
) -> sgx_status_t
{
    trace!("Entering ocall_get_quote_ecdsa");

    //let mut qe_target_info: sgx_target_info_t;
    //sgx_qe_get_target_info(&qe_target_info);

    let mut nQuoteAct: u32 = 0;
    let mut ret = unsafe { sgx_qe_get_quote_size(&mut nQuoteAct) };
    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("sgx_qe_get_quote_size returned {}", ret);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if nQuoteAct > nQuote {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    ret = unsafe { sgx_qe_get_quote(pReport, nQuote, pQuote) };
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

fn sgx_ql_qve_collateral_Serialize(
    pCol: *const u8,
    nCol: u32,
    pRes: *mut u8,
    nRes: u32,
) -> u32
{
    if nCol < mem::size_of::<sgx_ql_qve_collateral_t>() as u32 {
        return 0;
    }

    unsafe {
        let pQlCol = pCol as *const sgx_ql_qve_collateral_t;

        let size_extra =
            (*pQlCol).pck_crl_issuer_chain_size +
            (*pQlCol).root_ca_crl_size +
            (*pQlCol).pck_crl_size +
            (*pQlCol).tcb_info_issuer_chain_size +
            (*pQlCol).tcb_info_size +
            (*pQlCol).qe_identity_issuer_chain_size +
            (*pQlCol).qe_identity_size
            ;

        if nCol < mem::size_of::<sgx_ql_qve_collateral_t>() as u32 + size_extra {
            return 0;
        }

        let outSize: u32 = mem::size_of::<QlQveCollateral>() as u32 + size_extra;

        if nRes >= outSize {

            let x = QlQveCollateral {
                tee_type : (*pQlCol).tee_type,
                pck_crl_issuer_chain_size : (*pQlCol).pck_crl_issuer_chain_size,
                root_ca_crl_size : (*pQlCol).root_ca_crl_size,
                pck_crl_size : (*pQlCol).pck_crl_size,
                tcb_info_issuer_chain_size : (*pQlCol).tcb_info_issuer_chain_size,
                tcb_info_size : (*pQlCol).tcb_info_size,
                qe_identity_issuer_chain_size : (*pQlCol).qe_identity_issuer_chain_size,
                qe_identity_size : (*pQlCol).qe_identity_size
            };

            ptr::copy_nonoverlapping(&x as *const QlQveCollateral as *const u8, pRes, mem::size_of::<QlQveCollateral>());
            let mut offs = mem::size_of::<QlQveCollateral>();

            ptr::copy_nonoverlapping((*pQlCol).pck_crl_issuer_chain as *const u8, pRes.add(offs), x.pck_crl_issuer_chain_size as usize);
            offs += x.pck_crl_issuer_chain_size as usize;

            ptr::copy_nonoverlapping((*pQlCol).root_ca_crl as *const u8, pRes.add(offs), x.root_ca_crl_size as usize);
            offs += x.root_ca_crl_size as usize;

            ptr::copy_nonoverlapping((*pQlCol).pck_crl as *const u8, pRes.add(offs), x.pck_crl_size as usize);
            offs += x.pck_crl_size as usize;

            ptr::copy_nonoverlapping((*pQlCol).tcb_info_issuer_chain as *const u8, pRes.add(offs), x.tcb_info_issuer_chain_size as usize);
            offs += x.tcb_info_issuer_chain_size as usize;

            ptr::copy_nonoverlapping((*pQlCol).tcb_info as *const u8, pRes.add(offs), x.tcb_info_size as usize);
            offs += x.tcb_info_size as usize;

            ptr::copy_nonoverlapping((*pQlCol).qe_identity_issuer_chain as *const u8, pRes.add(offs), x.qe_identity_issuer_chain_size as usize);
            offs += x.qe_identity_issuer_chain_size as usize;

            ptr::copy_nonoverlapping((*pQlCol).qe_identity as *const u8, pRes.add(offs), x.qe_identity_size as usize);
            offs += x.qe_identity_size as usize;
        }

        return outSize;
    };

    0; // unreachable
}


fn sgx_ql_qve_collateral_Deserialize(
    pSer: *const u8,
    nSer: u32,
) -> sgx_ql_qve_collateral_t
{
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

    if nSer >= mem::size_of::<QlQveCollateral>() as u32 {

        unsafe {
            let pQlCol = pSer as *const QlQveCollateral;
            let size_extra =
                (*pQlCol).pck_crl_issuer_chain_size +
                (*pQlCol).root_ca_crl_size +
                (*pQlCol).pck_crl_size +
                (*pQlCol).tcb_info_issuer_chain_size +
                (*pQlCol).tcb_info_size +
                (*pQlCol).qe_identity_issuer_chain_size +
                (*pQlCol).qe_identity_size
                ;

            if nSer >= mem::size_of::<QlQveCollateral>() as u32 + size_extra {

                res.version = 1; // PCK Cert chain is in the Quote.
                res.tee_type = (*pQlCol).tee_type;
                res.pck_crl_issuer_chain_size = (*pQlCol).pck_crl_issuer_chain_size;
                res.root_ca_crl_size = (*pQlCol).root_ca_crl_size;
                res.pck_crl_size = (*pQlCol).pck_crl_size;
                res.tcb_info_issuer_chain_size = (*pQlCol).tcb_info_issuer_chain_size;
                res.tcb_info_size = (*pQlCol).tcb_info_size;
                res.qe_identity_issuer_chain_size = (*pQlCol).qe_identity_issuer_chain_size;
                res.qe_identity_size = (*pQlCol).qe_identity_size;

                let mut offs = mem::size_of::<QlQveCollateral>();

                res.pck_crl_issuer_chain = pSer.add(offs) as *mut i8;
                offs += res.pck_crl_issuer_chain_size as usize;

                res.root_ca_crl = pSer.add(offs) as *mut i8;
                offs += res.root_ca_crl_size as usize;

                res.pck_crl = pSer.add(offs) as *mut i8;
                offs += res.pck_crl_size as usize;

                res.tcb_info_issuer_chain = pSer.add(offs) as *mut i8;
                offs += res.tcb_info_issuer_chain_size as usize;

                res.tcb_info = pSer.add(offs) as *mut i8;
                offs += res.tcb_info_size as usize;

                res.qe_identity_issuer_chain = pSer.add(offs) as *mut i8;
                offs += res.qe_identity_issuer_chain_size as usize;

                res.qe_identity = pSer.add(offs) as *mut i8;
                offs += res.qe_identity_size as usize;
            }
        }

    };



    return res; // unreachable
}

#[no_mangle]
pub extern "C" fn ocall_get_quote_ecdsa_collateral(
    pQuote: *const u8,
    nQuote: u32,
    pCol: *mut u8,
    nCol: u32,
    pColOut: *mut u32
) -> sgx_status_t
{
    let mut pColMy : *mut u8 = 0 as *mut u8;
    let mut nColMy : u32 = 0;

    let ret = unsafe { tee_qv_get_collateral(pQuote, nQuote, &mut pColMy, &mut nColMy) };

    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        trace!("tee_qv_get_collateral returned {}", ret);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    unsafe {

        *pColOut = sgx_ql_qve_collateral_Serialize(pColMy, nColMy, pCol, nCol);

        tee_qv_free_collateral(pColMy);
    };

    sgx_status_t::SGX_SUCCESS
}

#[no_mangle]
pub extern "C" fn ocall_verify_quote_ecdsa(
    pQuote: *const u8,
    nQuote:u32,
    pCol: *const u8,
    nCol:u32,
    pTargetInfo: *const sgx_target_info_t,
    nTime: i64,
    p_qve_report_info: *mut sgx_ql_qe_report_info_t,
    pSuppData: *mut u8,
    nSuppData:u32,
    pSuppDataActual: *mut u32,
    pTime: *mut i64,
    pCollateral_expiration_status: *mut u32,
    pQvResult: *mut sgx_ql_qv_result_t,
) -> sgx_status_t
{
    let mut nTimeUse :time_t = nTime;
    if nTime == 0 {
        nTimeUse = SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_secs() as time_t;
    }

    unsafe {
        sgx_qv_set_enclave_load_policy(sgx_ql_request_policy_t::SGX_QL_PERSISTENT);
        sgx_qv_get_quote_supplemental_data_size(pSuppDataActual);

        if *pSuppDataActual > nSuppData {
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }

        (*p_qve_report_info).app_enclave_target_info = *pTargetInfo;

        let myCol = sgx_ql_qve_collateral_Deserialize(pCol, nCol);

        sgx_qv_verify_quote(
            pQuote, nQuote,
            &myCol,
            nTimeUse,
            pCollateral_expiration_status,
            pQvResult,
            p_qve_report_info,
            *pSuppDataActual,
            pSuppData);

        *pTime = nTimeUse;
    };

    sgx_status_t::SGX_SUCCESS
}







#[no_mangle]
pub extern "C" fn ocall_get_update_info(
    platform_blob: *const sgx_platform_info_t,
    enclave_trusted: i32,
    update_info: *mut sgx_update_info_bit_t,
) -> sgx_status_t {
    unsafe { sgx_report_attestation_status(platform_blob, enclave_trusted, update_info) }
}

pub fn create_attestation_report_u(api_key: &[u8]) -> SgxResult<()> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_get_attestation_report(eid, &mut retval, api_key.as_ptr(), api_key.len() as u32)
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
