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
pub unsafe extern "C" fn ecall_check_patch_level(
    _api_key: *const u8,
    _api_key_len: u32,
) -> NodeAuthResult {
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

#[cfg(feature = "SGX_MODE_HW")]
unsafe fn check_patch_level_epid(
    pub_k: &[u8; 32],
    api_key: *const u8,
    api_key_len: u32,
) -> NodeAuthResult {
    validate_const_ptr!(api_key, api_key_len as usize, NodeAuthResult::InvalidInput);
    if api_key_len > 100 {
        error!("API key malformed");
        return NodeAuthResult::InvalidInput;
    }

    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let signed_report =
        match create_attestation_report(pub_k, SIGNATURE_TYPE, api_key_slice, None, true) {
            Ok(r) => r,
            Err(_e) => {
                error!("Error creating attestation report");
                return NodeAuthResult::InvalidCert;
            }
        };

    let payload: String = serde_json::to_string(&signed_report)
        .map_err(|_| {
            error!("Error serializing report. May be malformed, or badly encoded");
            NodeAuthResult::InvalidCert
        })
        .unwrap();

    // extract private key from KeyPair
    let ecc_handle = sgx_tcrypto::SgxEccHandle::new();
    let _result = ecc_handle.open();

    let (prv_k, pub_k) = ecc_handle.create_key_pair().unwrap();

    let _result = ecc_handle.open();
    let (_key_der, cert) = super::cert::gen_ecc_cert(payload, &prv_k, &pub_k, &ecc_handle).unwrap();
    let _result = ecc_handle.close();

    let report = AttestationReport::from_cert(&cert)
        .map_err(|_| {
            error!("Failed to create report from certificate");
            NodeAuthResult::InvalidCert
        })
        .unwrap();

    // PERFORM EPID CHECK
    #[cfg(not(feature = "epid_whitelist_disabled"))]
    if !check_epid_gid_is_whitelisted(&report.sgx_quote_body.gid) {
        error!(
            "Platform verification error: quote status {:?}",
            &report.sgx_quote_body.gid
        );
        error!("Your current platform is probably not up to date, and may require a BIOS or PSW update. \n \
                Please see https://docs.scrt.network/secret-network-documentation/infrastructure/setting-up-a-node-validator/hardware-setup/patching-your-node \
                for more information");
        error!("If you think this message appeared in error, please contact us on Telegram or Discord, and attach your quote status from the message above");
        return NodeAuthResult::BadQuoteStatus;
    }

    if report.tcb_eval_data_number < 16 {
        error!("Your current platform is probably not up to date, and may require a BIOS or PSW update. \n \
                Please see https://docs.scrt.network/secret-network-documentation/infrastructure/setting-up-a-node-validator/hardware-setup/patching-your-node \
                for more information");
        println!(
            "Tried to attest using old data: {}",
            report.tcb_eval_data_number
        );
        return NodeAuthResult::GroupOutOfDate;
    }

    // PERFORM STATUS CHECKS
    let node_auth_result = NodeAuthResult::from(&report.sgx_quote_status);
    // print
    match verify_quote_status(&report, &report.advisory_ids) {
        Err(status) => match status {
            NodeAuthResult::SwHardeningAndConfigurationNeeded => {
                println!("Platform status is SW_HARDENING_AND_CONFIGURATION_NEEDED. This means is updated but requires further BIOS configuration");
            }
            NodeAuthResult::GroupOutOfDate => {
                println!("Platform status is GROUP_OUT_OF_DATE. This means that one of the system components is missing a security update");
            }
            _ => {
                println!("Platform status is {:?}", status);
            }
        },
        _ => println!("Platform Okay!"),
    }

    // print platform blob info
    match node_auth_result {
        NodeAuthResult::GroupOutOfDate | NodeAuthResult::SwHardeningAndConfigurationNeeded => unsafe {
            print_platform_info(&report);
            node_auth_result
        },
        _ => NodeAuthResult::Success,
    }
}

/// # Safety
/// Don't forget to check the input length of api_key_len
#[no_mangle]
#[cfg(feature = "SGX_MODE_HW")]
pub unsafe extern "C" fn ecall_check_patch_level(
    api_key: *const u8,
    api_key_len: u32,
) -> NodeAuthResult {
    let temp_key_result = enclave_crypto::KeyPair::new().unwrap();

    let res1 = check_patch_level_dcap(&temp_key_result.get_pubkey());
    let res2 = check_patch_level_epid(&temp_key_result.get_pubkey(), api_key, api_key_len);

    println!("DCAP attestation: {}", res1);
    println!("EPID attestation: {}", res2);

    if NodeAuthResult::Success == res1 {
        return res1;
    }

    res2
}
