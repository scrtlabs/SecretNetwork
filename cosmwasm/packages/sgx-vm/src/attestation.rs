use std::net::{SocketAddr, TcpStream};
use std::os::unix::io::IntoRawFd;

use std::{self};

use log::*;
use sgx_types::*;
use sgx_types::{sgx_status_t, SgxResult};

use enclave_ffi_types::{NodeAuthResult, ENCRYPTED_SEED_SIZE};

use crate::enclave::ENCLAVE_DOORBELL;

extern "C" {
    pub fn ecall_get_attestation_report(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        spid: *const u8,
        spid_len: u32,
        api_key: *const u8,
        api_key_len: u32,
    ) -> sgx_status_t;
    pub fn ecall_authenticate_new_node(
        eid: sgx_enclave_id_t,
        retval: *mut NodeAuthResult,
        cert: *const u8,
        cert_len: u32,
        seed: &mut [u8; ENCRYPTED_SEED_SIZE],
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
pub extern "C" fn ocall_get_update_info(
    platform_blob: *const sgx_platform_info_t,
    enclave_trusted: i32,
    update_info: *mut sgx_update_info_bit_t,
) -> sgx_status_t {
    unsafe { sgx_report_attestation_status(platform_blob, enclave_trusted, update_info) }
}

pub fn create_attestation_report_u(spid: &[u8], api_key: &[u8]) -> SgxResult<()> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(false) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_get_attestation_report(
            eid,
            &mut retval,
            spid.as_ptr(),
            spid.len() as u32,
            api_key.as_ptr(),
            api_key.len() as u32,
        )
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
) -> SgxResult<Result<[u8; ENCRYPTED_SEED_SIZE], NodeAuthResult>> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(false) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;
    let eid = enclave.geteid();
    let mut retval = NodeAuthResult::Success;
    let mut seed = [0u8; ENCRYPTED_SEED_SIZE];
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
        return Err(status);
    }

    if retval != NodeAuthResult::Success {
        return Ok(Err(retval));
    }

    if seed.is_empty() {
        error!("Got empty seed from encryption");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    Ok(Ok(seed))
}

#[cfg(test)]
mod test {
    use crate::attestation::retry_quote;
    use crate::esgx::general::init_enclave_wrapper;
    use crate::instance::init_enclave as init_enclave_wrapper;

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
