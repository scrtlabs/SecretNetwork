use crate::registration::{
    cert::{ocall_get_update_info, verify_quote_status},
    report::AttestationReport,
};

use enclave_ffi_types::NodeAuthResult;
use log::{error, warn};
use sgx_types::{sgx_platform_info_t, sgx_status_t, sgx_update_info_bit_t};

pub fn print_local_report_info(cert: &[u8]) {
    let report = match AttestationReport::from_cert(cert) {
        Ok(r) => r,
        Err(_) => {
            error!("Error parsing report");
            return;
        }
    };

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
            print_platform_info(&report)
        },
        _ => {}
    }
}

/// # Safety
/// Placeholder
pub unsafe fn print_platform_info(report: &AttestationReport) {
    if let Some(platform_info) = &report.platform_info_blob {
        let mut update_info = sgx_update_info_bit_t::default();
        let mut rt = sgx_status_t::default();
        let res = ocall_get_update_info(
            &mut rt as *mut sgx_status_t,
            platform_info[4..].as_ptr() as *const sgx_platform_info_t,
            1,
            &mut update_info,
        );

        if res != sgx_status_t::SGX_SUCCESS {
            error!("Error parsing attestation report {:?}", res);
            return;
        }

        if rt != sgx_status_t::SGX_SUCCESS {
            if update_info.ucodeUpdate != 0 {
                warn!("Processor Firmware Update (ucodeUpdate). A security upgrade for your computing\n\
                            device is required for this application to continue to provide you with a high degree of\n\
                            security. Please contact your device manufacturer’s support website for a BIOS update\n\
                            for this system");
            }

            if update_info.csmeFwUpdate != 0 {
                warn!("Intel Manageability Engine Update (csmeFwUpdate). A security upgrade for your\n\
                            computing device is required for this application to continue to provide you with a high\n\
                            degree of security. Please contact your device manufacturer’s support website for a\n\
                            BIOS and/or Intel® Manageability Engine update for this system");
            }

            if update_info.pswUpdate != 0 {
                warn!("Intel SGX Platform Software Update (pswUpdate). A security upgrade for your\n\
                              computing device is required for this application to continue to provide you with a high\n\
                              degree of security. Please visit this application’s support website for an Intel SGX\n\
                              Platform SW update");
            }
        }
    }
}
