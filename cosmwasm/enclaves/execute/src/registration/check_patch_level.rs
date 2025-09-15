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
use crate::sgx_types::{
    sgx_ql_auth_data_t, sgx_ql_certification_data_t, sgx_ql_ecdsa_sig_data_t, sgx_ql_qv_result_t,
    sgx_quote_t,
};

#[cfg(feature = "SGX_MODE_HW")]
use std::{cmp, mem};

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
    p_ppid: *mut u8,
    n_ppid: u32,
    p_ppid_size: *mut u32,
) -> NodeAuthResult {
    panic!("unimplemented")
}

fn extract_asn1_value(cert: &[u8], oid: &[u8]) -> Option<Vec<u8>> {
    let mut offset = match cert.windows(oid.len()).position(|window| window == oid) {
        Some(size) => size,
        None => {
            return None;
        }
    };

    offset += 12; // 11 + TAG (0x04)

    if offset + 2 >= cert.len() {
        return None;
    }

    // Obtain Netscape Comment length
    let mut len = cert[offset] as usize;
    if len > 0x80 {
        len = (cert[offset + 1] as usize) * 0x100 + (cert[offset + 2] as usize);
        offset += 2;
    }

    // Obtain Netscape Comment
    offset += 1;

    if offset + len >= cert.len() {
        return None;
    }

    let payload = cert[offset..offset + len].to_vec();

    Some(payload)
}

fn extract_cpu_cert_from_cert(cert_data: &[u8]) -> Option<Vec<u8>> {
    //println!("******** cert_data: {}", orig_hex::encode(cert_data));

    let pem_text = match std::str::from_utf8(cert_data) {
        Ok(x) => x,
        Err(_) => {
            return None;
        }
    };

    //println!("******** pem: {}", pem_text);

    // Find the first PEM block
    let begin_marker = "-----BEGIN CERTIFICATE-----";
    let end_marker = "-----END CERTIFICATE-----";
    let start = match pem_text.find(begin_marker) {
        Some(x) => x + begin_marker.len(),
        None => {
            println!("no begin");
            return None;
        }
    };

    let end = match pem_text.find(end_marker) {
        Some(x) => x,
        None => {
            println!("no end");
            return None;
        }
    };
    let b64 = &pem_text[start..end];

    // Remove whitespace and line breaks
    let b64_clean: String = b64.chars().filter(|c| !c.is_whitespace()).collect();

    // Decode Base64 into DER
    let der_bytes = match base64::decode(&b64_clean) {
        Ok(x) => x,
        Err(_) => {
            return None;
        }
    };

    //println!("Leaf certificate: {}", orig_hex::encode(&der_bytes));

    let ppid_oid = &[
        0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF8, 0x4D, 0x01, 0x0D, 0x01,
    ];

    let res = match extract_asn1_value(&der_bytes, ppid_oid) {
        Some(x) => x,
        None => {
            return None;
        }
    };

    Some(res)
}

unsafe fn extract_cpu_cert_from_quote(vec_quote: &[u8]) -> Option<Vec<u8>> {
    let my_p_quote = vec_quote.as_ptr() as *const sgx_quote_t;

    let sig_len = (*my_p_quote).signature_len as usize;
    let whole_len = sig_len.wrapping_add(mem::size_of::<sgx_quote_t>());
    if (whole_len > sig_len)
        && (whole_len <= vec_quote.len())
        && (sig_len >= mem::size_of::<sgx_ql_ecdsa_sig_data_t>())
    {
        let p_ecdsa_sig = (*my_p_quote).signature.as_ptr() as *const sgx_ql_ecdsa_sig_data_t;

        let auth_size_brutto = sig_len - mem::size_of::<sgx_ql_ecdsa_sig_data_t>();
        if auth_size_brutto >= mem::size_of::<sgx_ql_auth_data_t>() {
            let auth_size_max = auth_size_brutto - mem::size_of::<sgx_ql_auth_data_t>();

            let auth_data_wrapper =
                (*p_ecdsa_sig).auth_certification_data.as_ptr() as *const sgx_ql_auth_data_t;

            let auth_hdr_size = (*auth_data_wrapper).size as usize;
            if auth_hdr_size <= auth_size_max {
                let auth_size = auth_size_max - auth_hdr_size;

                if auth_size > mem::size_of::<sgx_ql_certification_data_t>() {
                    let cert_data = (*auth_data_wrapper)
                        .auth_data
                        .as_ptr()
                        .offset(auth_hdr_size as isize)
                        as *const sgx_ql_certification_data_t;

                    let cert_size_max = auth_size - mem::size_of::<sgx_ql_certification_data_t>();
                    let cert_size = (*cert_data).size as usize;
                    if (cert_size <= cert_size_max) && ((*cert_data).cert_key_type == 5) {
                        let cert_data = slice::from_raw_parts(
                            (*cert_data).certification_data.as_ptr(),
                            cert_size,
                        );

                        return extract_cpu_cert_from_cert(cert_data);
                    }
                }
            }
        }
    }

    None
}

#[cfg(feature = "SGX_MODE_HW")]
unsafe fn check_patch_level_dcap(pub_k: &[u8; 32]) -> (NodeAuthResult, Option<Vec<u8>>) {
    match get_quote_ecdsa_untested(pub_k) {
        Ok((vec_quote, vec_coll)) => {
            match verify_quote_sgx(&vec_quote, &vec_coll, 0) {
                Ok(r) => {
                    if r.1 != sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK {
                        println!("WARNING: {}", r.1);
                    }

                    let ppid = extract_cpu_cert_from_quote(&vec_quote);

                    println!("DCAP attestation obtained and verified ok");
                    return (NodeAuthResult::Success, ppid);
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
    (NodeAuthResult::InvalidCert, None)
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
    p_ppid: *mut u8,
    n_ppid: u32,
    p_ppid_size: *mut u32,
) -> NodeAuthResult {
    use enclave_utils::validate_mut_ptr;

    validate_mut_ptr!(p_ppid, n_ppid as usize, NodeAuthResult::BadQuoteStatus);

    let temp_key_result = enclave_crypto::KeyPair::new().unwrap();

    let (res1, ppid) = check_patch_level_dcap(&temp_key_result.get_pubkey());
    let res2 = check_patch_level_epid(&temp_key_result.get_pubkey(), api_key, api_key_len);

    if let Some(ppid_val) = ppid {
        *p_ppid_size = ppid_val.len() as u32;
        let size_out = cmp::min(ppid_val.len(), n_ppid as usize);
        std::ptr::copy_nonoverlapping(ppid_val.as_ptr(), p_ppid, size_out);
    } else {
        *p_ppid_size = 0;
    }

    println!("DCAP attestation: {}", res1);
    println!("EPID attestation: {}", res2);

    if NodeAuthResult::Success == res1 {
        return res1;
    }

    res2
}
