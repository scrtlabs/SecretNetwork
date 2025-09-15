#![allow(unused_imports)]

use core::slice;

use log::error;

use enclave_crypto::consts::SIGNATURE_TYPE;
use enclave_ffi_types::NodeAuthResult;
use enclave_utils::validate_const_ptr;

#[cfg(feature = "SGX_MODE_HW")]
use crate::registration::attestation::create_attestation_report;

#[cfg(feature = "SGX_MODE_HW")]
use crate::registration::cert::verify_quote_status;

#[cfg(feature = "SGX_MODE_HW")]
use crate::registration::attestation::get_quote_ecdsa_untested;

#[cfg(feature = "SGX_MODE_HW")]
use crate::registration::attestation::verify_quote_sgx;

#[cfg(feature = "SGX_MODE_HW")]
use enclave_utils::storage::write_to_untrusted;

#[cfg(feature = "SGX_MODE_HW")]
use crate::sgx_types::sgx_ql_qv_result_t;

#[cfg(not(feature = "epid_whitelist_disabled"))]
use crate::registration::cert::check_epid_gid_is_whitelisted;

#[cfg(feature = "SGX_MODE_HW")]
use crate::registration::print_report::print_platform_info;

use crate::registration::report::AttestationReport;

/// # Safety
#[no_mangle]
#[cfg(not(feature = "SGX_MODE_HW"))]
pub unsafe extern "C" fn ecall_check_patch_level() -> NodeAuthResult {
    panic!("unimplemented")
}

#[cfg(feature = "SGX_MODE_HW")]
unsafe fn check_patch_level_dcap(pub_k: &[u8; 32]) -> NodeAuthResult {
    match get_quote_ecdsa_untested(pub_k) {
        Ok((vec_quote, vec_coll)) => {
            match verify_quote_sgx(&vec_quote, &vec_coll, 0) {
                Ok(r) => {
                    if r.1 != sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK {
                        println!("WARNING: {}", r.1);
                    }

                    println!("DCAP attestation obtained and verified ok");
                    return NodeAuthResult::Success;
                }
                Err(e) => {
                    println!("DCAP quote obtained, but failed to verify it: {}", e);

                    let _ = write_to_untrusted(&vec_quote, "dcap_quote.bin");
                    let _ = write_to_untrusted(&vec_coll, "dcap_collateral.bin");
                }
            };
        }
        Err(e) => {
            println!("Failed to obtain DCAP attestation: {}", e);
        }
    }
    NodeAuthResult::InvalidCert
}

/// # Safety
/// Don't forget to check the input length of api_key_len
#[no_mangle]
#[cfg(feature = "SGX_MODE_HW")]
pub unsafe extern "C" fn ecall_check_patch_level() -> NodeAuthResult {
    let temp_key_result = enclave_crypto::KeyPair::new().unwrap();

    let res = check_patch_level_dcap(&temp_key_result.get_pubkey());

    println!("DCAP attestation: {}", res);

    res
}
