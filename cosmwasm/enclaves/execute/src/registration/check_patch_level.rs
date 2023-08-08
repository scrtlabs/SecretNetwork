use core::slice;

use log::error;

use enclave_crypto::consts::SIGNATURE_TYPE;
use enclave_ffi_types::NodeAuthResult;
use enclave_utils::validate_const_ptr;

use crate::registration::attestation::create_attestation_report;
use crate::registration::cert::{check_epid_gid_is_whitelisted, verify_quote_status};
use crate::registration::print_report::print_platform_info;
use crate::registration::report::AttestationReport;

#[no_mangle]
pub unsafe extern "C" fn ecall_check_patch_level(
    api_key: *const u8,
    api_key_len: u32,
) -> NodeAuthResult {
    validate_const_ptr!(api_key, api_key_len as usize, NodeAuthResult::InvalidInput);
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    // CREATE THE ATTESTATION REPORT
    // generate temporary key for attestation
    let temp_key_result = enclave_crypto::KeyPair::new().unwrap();

    let signed_report = match create_attestation_report(
        &temp_key_result.get_pubkey(),
        SIGNATURE_TYPE,
        api_key_slice,
        None,
    ) {
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
    return match node_auth_result {
        NodeAuthResult::GroupOutOfDate | NodeAuthResult::SwHardeningAndConfigurationNeeded => unsafe {
            print_platform_info(&report);
            node_auth_result
        },
        _ => NodeAuthResult::Success,
    };
}
