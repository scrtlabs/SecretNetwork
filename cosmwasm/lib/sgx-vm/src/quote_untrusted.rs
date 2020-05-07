use base64;
use crate::errors::Error;
use sgx_types::*;
use std::thread::sleep;
use std::{self, time};
use log::*;
use crate::wasmi::{ecall_get_registration_quote, ecall_get_attestation_report};

pub fn retry_quote(eid: sgx_enclave_id_t, spid: &str, times: usize) -> Result<String, Error> {
    let mut quote = String::new();
    for _ in 0..times {
        quote = match produce_quote(eid, spid) {
            Ok(q) => q,
            Err(e) => {
                error!("problem with quote, trying again: {:?}", e);
                continue;
            }
        };

        if !quote.chars().all(|cur_c| cur_c == 'A') {
            return Ok(quote);
        } else {
            sleep(time::Duration::new(5, 0));
        }
    }
    Err(Error::SdkErr { inner: sgx_status_t::SGX_ERROR_SERVICE_UNAVAILABLE }.into())
}

fn check_busy<T, F>(func: F) -> (sgx_status_t, T)
where F: Fn() -> (sgx_status_t, T) {
    loop {
        let (status, rest) = func();
        if status != sgx_status_t::SGX_ERROR_BUSY {
            return (status, rest);
        }
        info!("sleeping");
        sleep(time::Duration::new(1, 500_000_000));
    }
}

pub fn produce_report(eid: sgx_enclave_id_t) {
    let mut retval = sgx_status_t::SGX_SUCCESS;
    info!("Entered produce report");
    let status = unsafe { ecall_get_attestation_report(eid, &mut retval) };
}


pub fn produce_quote(eid: sgx_enclave_id_t, spid: &str) -> Result<String, Error> {
    info!("produce_quote entered");
    let spid = hex::decode(spid).unwrap();
    let mut id = [0; 16];
    id.copy_from_slice(&spid);
    let spid: sgx_spid_t = sgx_spid_t { id };
    info!("before check_busy");
    // create quote
    let (status, (target_info, _gid)) = check_busy(|| {
        let mut target_info = sgx_target_info_t::default();
        let mut gid = sgx_epid_group_id_t::default();
        info!("before sgx_init_quote");
        let status = unsafe { sgx_init_quote(&mut target_info, &mut gid) };
        (status, (target_info, gid))
    });
    if status != sgx_status_t::SGX_SUCCESS {
        return Err(Error::SdkErr { inner: status }.into());
    }

    // create report
    let (status, (report, retval)) = check_busy(move || {
        let mut report = sgx_report_t::default();
        let mut retval = sgx_status_t::SGX_SUCCESS;
        info!("before ecall_get_registration_quote");
        let status = unsafe { ecall_get_registration_quote(eid, &mut retval, &target_info, &mut report) };
        (status, (report, retval))
    });

    if status != sgx_status_t::SGX_SUCCESS || retval != sgx_status_t::SGX_SUCCESS {
        return Err(Error::SdkErr { inner: status }.into());
    }


    // calc quote size
    let (status, quote_size) = check_busy(|| {
        let mut quote_size: u32 = 0;
        info!("before sgx_calc_quote_size");
        let status = unsafe { sgx_calc_quote_size(std::ptr::null(), 0, &mut quote_size) };
        (status, quote_size)
    });
    if status != sgx_status_t::SGX_SUCCESS || quote_size == 0 {
        return Err(Error::SdkErr { inner: status }.into());
    }

    // get the actual quote
    let (status, the_quote) = check_busy(|| {
        let mut the_quote = vec![0u8; quote_size as usize].into_boxed_slice();
        // all of this is according to this: https://software.intel.com/en-us/sgx-sdk-dev-reference-sgx-get-quote
        // the `p_qe_report` is null together with the nonce because we don't have an ISV enclave that needs to verify this
        // and we don't care about replay attacks because the signing key will stay the same and that's what's important.
        let status = unsafe {
            info!("before sgx_get_quote");
            sgx_get_quote(&report,
                          sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE,
                          &spid,
                          std::ptr::null(),
                          std::ptr::null(),
                          0,
                          std::ptr::null_mut(),
                          the_quote.as_mut_ptr() as *mut sgx_quote_t,
                          quote_size,
            )
        };
        (status, the_quote)
    });
    if status != sgx_status_t::SGX_SUCCESS {
        return Err(Error::SdkErr { inner: status }.into());
    }

    let encoded_quote = base64::encode(&the_quote);
    info!("encoded_quote: {:?}", encoded_quote);
    Ok(encoded_quote)
}
#[cfg(test)]
mod test {
    use crate::esgx::general::init_enclave_wrapper;
    use crate::quote_untrusted::retry_quote;
    use crate::instance::init_enclave as init_enclave_wrapper;
    // isans SPID = "3DDB338BD52EE314B01F1E4E1E84E8AA"
    // victors spid = 68A8730E9ABF1829EA3F7A66321E84D0
    const SPID: &str = "B0335FD3BC1CCA8F804EB98A6420592D"; // Elichai's SPID

    #[test]
    fn test_produce_quote() {
        // initiate the enclave
        let enclave = init_enclave_wrapper().unwrap();
        // produce a quote

        let tested_encoded_quote = match retry_quote(enclave.geteid(), &SPID, 18) {
            Ok(encoded_quote) => encoded_quote,
            Err(e) => {
                error!("Produce quote Err {}, {}", e.as_fail(), e.backtrace());
                assert_eq!(0, 1);
                return;
            }
        };
        debug!("-------------------------");
        debug!("{}", tested_encoded_quote);
        debug!("-------------------------");
        enclave.destroy();
        assert!(!tested_encoded_quote.is_empty());
        // assert_eq!(real_encoded_quote, tested_encoded_quote);
    }

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
