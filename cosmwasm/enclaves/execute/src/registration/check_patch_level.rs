#![allow(unused_imports)]

use core::slice;

use log::error;

use enclave_ffi_types::NodeAuthResult;
use enclave_utils::validate_const_ptr;

#[cfg(feature = "SGX_MODE_HW")]
use crate::registration::attestation::get_quote_ecdsa_untested;

#[cfg(feature = "SGX_MODE_HW")]
use crate::registration::attestation::verify_quote_sgx;

#[cfg(feature = "SGX_MODE_HW")]
use enclave_utils::storage::write_to_untrusted;

#[cfg(feature = "SGX_MODE_HW")]
use crate::sgx_types::{
    sgx_ql_auth_data_t, sgx_ql_certification_data_t, sgx_ql_ecdsa_sig_data_t, sgx_ql_qv_result_t,
    sgx_quote_t,
};

#[cfg(feature = "SGX_MODE_HW")]
use std::{cmp, mem};

#[cfg(not(feature = "epid_whitelist_disabled"))]
use crate::registration::cert::check_epid_gid_is_whitelisted;

use crate::registration::report::AttestationReport;

/// # Safety
#[no_mangle]
#[cfg(not(feature = "SGX_MODE_HW"))]
pub unsafe extern "C" fn ecall_check_patch_level(
    _p_ppid: *mut u8,
    _n_ppid: u32,
    _p_ppid_size: *mut u32,
) -> NodeAuthResult {
    panic!("unimplemented")
}

#[cfg(feature = "SGX_MODE_HW")]
unsafe fn check_patch_level_dcap(pub_k: &[u8; 32]) -> (NodeAuthResult, Option<Vec<u8>>) {
    match get_quote_ecdsa_untested(pub_k) {
        Ok(attestation) => {
            match verify_quote_sgx(&attestation, 0, false) {
                Ok(r) => {
                    if r.1 != sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK {
                        println!("WARNING: {}", r.1);
                    }

                    let ppid = attestation.extract_cpu_cert();

                    println!("DCAP attestation obtained and verified ok");
                    return (NodeAuthResult::Success, ppid);
                }
                Err(e) => {
                    println!("DCAP quote obtained, but failed to verify it: {}", e);

                    let _ = write_to_untrusted(&attestation.quote, "dcap_quote.bin");
                    let _ = write_to_untrusted(&attestation.coll, "dcap_collateral.bin");
                }
            };
        }
        Err(e) => {
            println!("Failed to obtain DCAP attestation: {}", e);
        }
    }
    (NodeAuthResult::InvalidCert, None)
}

/// # Safety
/// Don't forget to check the input length of api_key_len
#[no_mangle]
#[cfg(feature = "SGX_MODE_HW")]

pub unsafe extern "C" fn ecall_check_patch_level(
    p_ppid: *mut u8,
    n_ppid: u32,
    p_ppid_size: *mut u32,
) -> NodeAuthResult {
    use enclave_utils::validate_mut_ptr;

    validate_mut_ptr!(p_ppid, n_ppid as usize, NodeAuthResult::BadQuoteStatus);

    let temp_key_result = enclave_crypto::KeyPair::new().unwrap();

    let (res, ppid) = check_patch_level_dcap(&temp_key_result.get_pubkey());

    if let Some(ppid_val) = ppid {
        *p_ppid_size = ppid_val.len() as u32;
        let size_out = cmp::min(ppid_val.len(), n_ppid as usize);
        std::ptr::copy_nonoverlapping(ppid_val.as_ptr(), p_ppid, size_out);
    } else {
        *p_ppid_size = 0;
    }

    println!("DCAP attestation: {}", res);

    res
}
