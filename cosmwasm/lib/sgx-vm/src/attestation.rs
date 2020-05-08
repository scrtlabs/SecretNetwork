use base64;
use crate::errors::Error;
use sgx_types::*;
use std::thread::sleep;
use std::{self, time};
use log::*;
use sgx_urts::SgxResult;
use crate::wasmi::ecall_get_attestation_report;
use sgx_types::{SgxResult, sgx_status_t};

pub fn inner_create_report(eid: sgx_enclave_id_t) -> SgxResult<sgx_status_t> {

    info!("Entered produce report");
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe { ecall_get_attestation_report(eid, &mut retval) };

    if status != sgx_status_t::SGX_SUCCESS  {
        return Err(Error::SdkErr { inner: status }.into());
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(Error::SdkErr { inner: retval }.into());
    }

    Ok(sgx_status_t::SGX_SUCCESS)
}


#[cfg(test)]
mod test {
    use crate::esgx::general::init_enclave_wrapper;
    use crate::attestation::retry_quote;
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
