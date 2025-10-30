use log::*;
use sgx_types::*;
use sgx_types::{sgx_status_t, SgxResult};

use enclave_ffi_types::{NodeAuthResult, SINGLE_ENCRYPTED_SEED_SIZE};

use crate::enclave::ENCLAVE_DOORBELL;

extern "C" {
    pub fn ecall_get_attestation_report(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        flags: u32,
    ) -> sgx_status_t;
    pub fn ecall_authenticate_new_node(
        eid: sgx_enclave_id_t,
        retval: *mut NodeAuthResult,
        cert: *const u8,
        cert_len: u32,
        p_seeds: *mut u8,
        n_seeds: u32,
        p_seeds_size: *mut u32,
    ) -> sgx_status_t;
    pub fn ecall_get_genesis_seed(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        pk: *const u8,
        pk_len: u32,
        seed: &mut [u8; SINGLE_ENCRYPTED_SEED_SIZE as usize],
    ) -> sgx_status_t;
}

pub fn create_attestation_report_u(flags: u32) -> SgxResult<()> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe { ecall_get_attestation_report(eid, &mut retval, flags) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(())
}

pub fn untrusted_get_encrypted_seed(cert: &[u8]) -> SgxResult<Result<Vec<u8>, NodeAuthResult>> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;
    let eid = enclave.geteid();
    let mut retval = NodeAuthResult::Success;

    let mut seed_buffer = Vec::new();
    seed_buffer.resize(SINGLE_ENCRYPTED_SEED_SIZE * 100, 0); // should be enough. Resize in later version, when approaching the limit

    let mut seeds_size: u32 = 0;

    let status = unsafe {
        ecall_authenticate_new_node(
            eid,
            &mut retval,
            cert.as_ptr(),
            cert.len() as u32,
            seed_buffer.as_mut_ptr(),
            seed_buffer.len() as u32,
            &mut seeds_size,
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

    if seeds_size == 0 {
        error!("Got empty seed from encryption");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    seed_buffer.resize(seeds_size as usize, 0);

    debug!("Done auth, got seed: {}", hex::encode(&seed_buffer));

    Ok(Ok(seed_buffer))
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
