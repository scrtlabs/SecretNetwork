use core::{mem, slice};

use enclave_crypto::dcap::verify_quote_any;
use enclave_crypto::KeyPair;
use std::collections::HashSet;
use std::vec::Vec;

use log::*;

#[cfg(feature = "SGX_MODE_HW")]
use itertools::Itertools;

#[cfg(feature = "SGX_MODE_HW")]
use sgx_rand::{os, Rng};

#[cfg(feature = "SGX_MODE_HW")]
use sgx_tse::{rsgx_create_report, rsgx_verify_report};

#[cfg(feature = "SGX_MODE_HW")]
use sgx_tcrypto::rsgx_sha256_slice;

use sgx_tcrypto::SgxEccHandle;

use sgx_types::{
    sgx_ql_auth_data_t, sgx_ql_certification_data_t, sgx_ql_ecdsa_sig_data_t, sgx_ql_qv_result_t,
    sgx_quote_sign_type_t, sgx_quote_t, sgx_report_body_t, sgx_status_t,
};

#[cfg(feature = "SGX_MODE_HW")]
use sgx_types::{
    c_int, sgx_epid_group_id_t, sgx_quote_nonce_t, sgx_report_data_t, sgx_report_t, sgx_spid_t,
    sgx_target_info_t, SgxResult,
};

#[cfg(feature = "SGX_MODE_HW")]
use std::{
    io::{Read, Write},
    net::TcpStream,
    ptr, str,
    string::String,
    sync::Arc,
};

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
use crate::registration::cert::verify_ra_cert;
#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
use crate::registration::offchain::get_attestation_report_dcap;

#[cfg(feature = "SGX_MODE_HW")]
use enclave_crypto::consts::*;

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
use std::sgxfs::remove as SgxFsRemove;

#[cfg(feature = "SGX_MODE_HW")]
use super::ocalls::{
    ocall_get_ias_socket, ocall_get_quote, ocall_get_quote_ecdsa, ocall_get_quote_ecdsa_collateral,
    ocall_get_quote_ecdsa_params, ocall_sgx_init_quote,
};

#[cfg(feature = "SGX_MODE_HW")]
use super::{hex, report::EndorsedAttestationReport};

#[cfg(feature = "SGX_MODE_HW")]
use ::hex as orig_hex;

#[cfg(feature = "SGX_MODE_HW")]
pub const DEV_HOSTNAME: &str = "api.trustedservices.intel.com";

#[cfg(feature = "production")]
pub const SIGRL_SUFFIX: &str = "/sgx/attestation/v5/sigrl/";
#[cfg(feature = "production")]
pub const REPORT_SUFFIX: &str = "/sgx/attestation/v5/report?update=early";

#[cfg(feature = "production")]
pub const LEGACY_REPORT_SUFFIX: &str = "/sgx/attestation/v5/report";
#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const LEGACY_REPORT_SUFFIX: &str = "/sgx/dev/attestation/v5/report";

// #[cfg(feature = "SGX_MODE_HW")]
// pub const SN_TSS_HOSTNAME: &str = "secretnetwork.trustedservices.scrtlabs.com";
// #[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
// pub const SN_TSS_GID_LIST: &str = "/dev/get-gids";
// #[cfg(feature = "production")]
// pub const SN_TSS_GID_LIST: &str = "/get-gids";

#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const SIGRL_SUFFIX: &str = "/sgx/dev/attestation/v5/sigrl/";
#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const REPORT_SUFFIX: &str = "/sgx/dev/attestation/v5/report?update=early";

/// extra_data size that will store the public key of the attesting node
#[cfg(feature = "SGX_MODE_HW")]
const REPORT_DATA_SIZE: usize = 32;

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
pub const SPID: &str = "783C75FD041E28AEA2DBCD48617577FE";
#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const SPID: &str = "D0A5D0AF1E244EC7EA2175BC2E32093B";

#[cfg(not(feature = "SGX_MODE_HW"))]
pub fn create_attestation_certificate(
    kp: &KeyPair,
    _sign_type: sgx_quote_sign_type_t,
    _api_key: &[u8],
    _challenge: Option<&[u8]>,
) -> Result<(Vec<u8>, Vec<u8>), sgx_status_t> {
    // init sgx ecc
    let ecc_handle = SgxEccHandle::new();
    let _result = ecc_handle.open();

    // convert keypair private to sgx ecc private
    let (prv_k, pub_k) = ecc_handle.create_key_pair().unwrap();

    // this is the ed25519 public key we want to encode
    let encoded_pubkey = base64::encode(kp.get_pubkey());

    let (key_der, cert_der) =
        super::cert::gen_ecc_cert(encoded_pubkey, &prv_k, &pub_k, &ecc_handle)?;
    let _result = ecc_handle.close();

    Ok((key_der, cert_der))
}

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
pub fn validate_enclave_version(
    kp: &KeyPair,
    sign_type: sgx_quote_sign_type_t,
    api_key: &[u8],
    challenge: Option<&[u8]>,
) -> Result<(), sgx_status_t> {
    let res_dcap = unsafe { get_attestation_report_dcap(&kp.get_pubkey()) };
    if res_dcap.is_ok() {
        return Ok(());
    }

    // extract private key from KeyPair
    let ecc_handle = SgxEccHandle::new();
    let _result = ecc_handle.open();

    // use ephemeral key
    let (prv_k, pub_k) = ecc_handle.create_key_pair().unwrap();

    // call create_report using the secp256k1 public key, and __not__ the P256 one
    let signed_report =
        match create_attestation_report(&kp.get_pubkey(), sign_type, api_key, challenge, true) {
            Ok(r) => r,
            Err(e) => {
                error!("Error creating attestation report");
                return Err(e);
            }
        };

    let payload: String = serde_json::to_string(&signed_report).map_err(|_| {
        error!("Error serializing report. May be malformed, or badly encoded");
        sgx_status_t::SGX_ERROR_UNEXPECTED
    })?;

    let (_key_der, cert_der) = super::cert::gen_ecc_cert(payload, &prv_k, &pub_k, &ecc_handle)?;
    let _result = ecc_handle.close();

    if verify_ra_cert(&cert_der, None, true).is_err() {
        error!("Error verifying report.");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    Ok(())
}

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
fn remove_secret_file(file_name: &str) {
    let _ = SgxFsRemove(make_sgx_secret_path(file_name));
}

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
fn remove_all_keys() {
    remove_secret_file(&SEALED_FILE_UNITED);
    remove_secret_file(SEALED_FILE_REGISTRATION_KEY);
    remove_secret_file(SEALED_FILE_ENCRYPTED_SEED_KEY_GENESIS);
    remove_secret_file(SEALED_FILE_ENCRYPTED_SEED_KEY_CURRENT);
    remove_secret_file(SEALED_FILE_IRS);
    remove_secret_file(SEALED_FILE_REK);
    remove_secret_file(SEALED_FILE_TX_BYTES);
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn create_attestation_certificate(
    kp: &KeyPair,
    sign_type: sgx_quote_sign_type_t,
    api_key: &[u8],
    challenge: Option<&[u8]>,
) -> Result<(Vec<u8>, Vec<u8>), sgx_status_t> {
    // extract private key from KeyPair
    let ecc_handle = SgxEccHandle::new();
    let _result = ecc_handle.open();

    // use ephemeral key
    let (prv_k, pub_k) = ecc_handle.create_key_pair().unwrap();

    // call create_report using the secp256k1 public key, and __not__ the P256 one
    let signed_report =
        match create_attestation_report(&kp.get_pubkey(), sign_type, api_key, challenge, true) {
            Ok(r) => r,
            Err(e) => {
                error!("Error creating attestation report");
                return Err(e);
            }
        };

    let payload: String = serde_json::to_string(&signed_report).map_err(|_| {
        error!("Error serializing report. May be malformed, or badly encoded");
        sgx_status_t::SGX_ERROR_UNEXPECTED
    })?;
    let (key_der, cert_der) = super::cert::gen_ecc_cert(payload, &prv_k, &pub_k, &ecc_handle)?;
    let _result = ecc_handle.close();

    #[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
    validate_report(&cert_der, None);

    Ok((key_der, cert_der))
}

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
pub fn validate_report(cert: &[u8], _override_verify: Option<SigningMethod>) {
    let _ = verify_ra_cert(cert, None, true).map_err(|e| {
        info!("Error validating created certificate: {:?}", e);
        remove_all_keys();
    });
}

#[cfg(feature = "SGX_MODE_HW")]
#[allow(dead_code)]
pub fn in_grace_period(timestamp: u64) -> bool {
    // Friday, August 21, 2023 2:00:00 PM UTC
    timestamp < 1692626400_u64
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

    let res = match crate::registration::cert::extract_asn1_value(&der_bytes, ppid_oid) {
        Ok(x) => x,
        Err(_) => {
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

lazy_static::lazy_static! {

    static ref PPID_WHITELIST: HashSet<[u8; 20]>  = {
        let mut set: HashSet<[u8; 20]> = HashSet::new();

        set.insert([0x01,0x50,0x7c,0x95,0x77,0x89,0xb7,0xc1,0xaf,0xde,0x97,0x2d,0x67,0xf1,0xfd,0xd5,0x3a,0xf1,0xa8,0xda]);
        set.insert([0x04,0xf0,0x14,0x07,0xb7,0x62,0xaf,0x16,0xdb,0x04,0xac,0x64,0x8a,0xee,0x5f,0xeb,0x24,0xcf,0x6e,0xb8]);
        set.insert([0x05,0x04,0x30,0x40,0x8a,0x4c,0xea,0xe5,0xd0,0xb9,0xd9,0x57,0x21,0xd6,0x51,0x22,0x2c,0xbd,0x83,0xd3]);
        set.insert([0x05,0x1f,0x83,0xeb,0x42,0xdf,0xdd,0x78,0x50,0x08,0x6c,0xf5,0x69,0x6f,0xbb,0x36,0x53,0xf8,0x83,0xae]);
        set.insert([0x09,0xe9,0x87,0x5e,0xd7,0xac,0xd4,0x2c,0x7d,0xd1,0x9d,0x72,0xa3,0x95,0x24,0xf6,0xae,0x3e,0x87,0xfa]);
        set.insert([0x11,0x21,0xe6,0xd9,0xa7,0x70,0xc9,0xe5,0x62,0xae,0x42,0x30,0x12,0x08,0x0e,0x52,0x76,0x5e,0xcf,0x71]);
        set.insert([0x13,0xf1,0x08,0xbd,0xf8,0xfd,0x3f,0x11,0xe7,0x50,0x26,0xd9,0x8e,0xd1,0x80,0x30,0x75,0x25,0x73,0x54]);
        set.insert([0x14,0xd1,0x23,0x60,0x33,0xfd,0xe3,0x1e,0x0b,0xcd,0x57,0x0c,0x32,0x91,0xea,0xa9,0xbd,0xb7,0xe2,0x5e]);
        set.insert([0x16,0x32,0xea,0x13,0x05,0x1c,0xc4,0xd3,0x92,0xcb,0x8d,0x3e,0x01,0x6e,0xb5,0x61,0x7d,0x8e,0x8b,0x2a]);
        set.insert([0x18,0x81,0x20,0xcd,0x27,0xf6,0x58,0xa7,0x29,0x2c,0x46,0x6f,0x6c,0x7d,0xf5,0xaf,0x6c,0xdb,0x96,0x6e]);
        set.insert([0x1a,0xe4,0xfb,0x51,0x62,0x6d,0x0d,0x27,0x4c,0x0b,0xd3,0x85,0x1e,0x5a,0x04,0xde,0xe0,0xb0,0xd5,0x3b]);
        set.insert([0x1b,0x3a,0xad,0xe4,0x41,0xad,0xd6,0x58,0xf9,0xf9,0x5c,0x10,0xce,0x0f,0x3d,0x75,0x4f,0x92,0xa3,0x27]);
        set.insert([0x1c,0x8b,0xe8,0x11,0xc0,0x91,0x85,0xa9,0xf6,0xc7,0xdc,0x3f,0xba,0x81,0xe8,0x78,0xa5,0x8d,0x0a,0x1b]);
        set.insert([0x20,0x59,0x8f,0xcb,0x4b,0xa5,0x79,0xc5,0xa0,0x9e,0x8c,0x1a,0x42,0xec,0x9c,0x02,0x10,0xcb,0x43,0xf3]);
        set.insert([0x21,0xca,0xe8,0x22,0x31,0x4c,0x17,0x72,0x16,0x77,0xc4,0x1c,0xd8,0x42,0x34,0x64,0x98,0x35,0x53,0xc1]);
        set.insert([0x22,0x1c,0xd8,0x23,0x4e,0x36,0x9e,0xf1,0x9d,0xfd,0xad,0x94,0xf4,0x37,0xd3,0xa1,0x90,0xcb,0x26,0x73]);
        set.insert([0x24,0x91,0xb9,0xa2,0x2f,0x4b,0x3d,0x45,0x82,0xaf,0x20,0x92,0x19,0x19,0xbc,0x27,0x71,0xc8,0x0f,0xde]);
        set.insert([0x26,0x67,0x1a,0x09,0x31,0xeb,0xde,0x25,0x27,0x37,0x36,0xbf,0x6b,0x4b,0x2e,0xd1,0x03,0x8c,0x44,0x43]);
        set.insert([0x28,0x04,0x9f,0x3f,0x69,0xf6,0x55,0x83,0x24,0xe5,0x2d,0x44,0xec,0xde,0xf8,0x97,0xb1,0xa3,0xb3,0x90]);
        set.insert([0x2a,0x00,0x3f,0x2b,0x90,0x8b,0x08,0x47,0x6b,0xe2,0x5d,0x01,0x1c,0x57,0xfe,0x11,0xfc,0x68,0x92,0xf9]);
        set.insert([0x30,0x7b,0xbc,0xfb,0x24,0xab,0xdb,0x94,0xa6,0xb7,0xca,0xba,0xdd,0xb9,0x19,0x04,0x64,0x03,0xff,0xe6]);
        set.insert([0x31,0x5d,0xb0,0x4d,0xf6,0x69,0x89,0x4a,0x3e,0x35,0x42,0x96,0x91,0xb8,0xb3,0x50,0xd8,0xcb,0xcd,0xe2]);
        set.insert([0x35,0x99,0xc4,0x8a,0x2a,0x1b,0xbf,0xf8,0x74,0xda,0xc4,0x6d,0x98,0x6a,0x51,0x2f,0x69,0xfb,0xc8,0xd0]);
        set.insert([0x36,0xa3,0x75,0x06,0xdc,0xc6,0x2a,0x21,0x53,0xae,0x3d,0xa7,0x41,0xf5,0x6a,0x01,0xbe,0xd5,0x42,0x67]);
        set.insert([0x3e,0x68,0x30,0xd6,0xa2,0xd8,0x39,0xbf,0x36,0xbb,0x10,0x8c,0xa8,0xcc,0xc2,0x5a,0x16,0x78,0xf8,0x2f]);
        set.insert([0x40,0x83,0x2a,0x64,0xb2,0x7c,0x12,0xc7,0xab,0xe6,0xbe,0x09,0x47,0x16,0x3e,0xe4,0x83,0x47,0x8c,0x61]);
        set.insert([0x41,0x01,0xa8,0x18,0x7d,0x20,0x88,0x9c,0x4f,0xd2,0x47,0xb4,0xc8,0x27,0x9b,0x66,0x31,0x8e,0xf0,0x91]);
        set.insert([0x42,0x26,0xd0,0x78,0x02,0x9b,0x9b,0xe4,0xf3,0x2a,0x61,0x5e,0x92,0x90,0xdb,0xcd,0xe3,0xd5,0x5f,0x76]);
        set.insert([0x44,0x3b,0x01,0xf0,0x77,0x19,0xd7,0xce,0x6c,0xbd,0xe8,0x61,0x08,0x2a,0x33,0x18,0x1b,0xb2,0x4e,0x4c]);
        set.insert([0x44,0xe1,0x05,0x97,0xb4,0x2e,0x57,0x14,0x23,0x91,0x53,0x85,0xfe,0x85,0xce,0xd0,0xe8,0x40,0x85,0x0c]);
        set.insert([0x46,0x1b,0xe5,0xde,0x74,0xce,0x83,0x3d,0x38,0x28,0xfc,0xb5,0x7c,0x24,0x3b,0x60,0x15,0xd7,0x6d,0x7f]);
        set.insert([0x48,0x82,0x28,0x89,0x4e,0x72,0x65,0xff,0x74,0xd9,0x74,0x95,0xd2,0xd5,0x36,0xb9,0xc7,0xf5,0x74,0x11]);
        set.insert([0x4a,0xf5,0xd9,0x4e,0x22,0x81,0xe2,0xa4,0xc2,0x3e,0xd8,0x4c,0x4a,0x05,0xaa,0x5e,0x7a,0x31,0x78,0xd2]);
        set.insert([0x4e,0x09,0x78,0x81,0x59,0x00,0x8f,0x91,0xd7,0xf3,0xd6,0x8b,0x1a,0xf2,0x35,0x5b,0x3c,0x17,0x72,0xcc]);
        set.insert([0x51,0xc1,0xed,0xe1,0x8e,0x73,0xfc,0x7e,0x00,0xe8,0xfc,0x01,0x8e,0xde,0x53,0x63,0x2f,0x8b,0xb6,0xd9]);
        set.insert([0x53,0x54,0x3f,0x1a,0x7d,0x08,0xc1,0x3d,0x69,0x38,0x5b,0x81,0x20,0x56,0x2a,0x76,0xb8,0x71,0xf8,0xec]);
        set.insert([0x57,0x19,0x8e,0xb3,0x6e,0x5c,0x99,0x68,0x63,0xb6,0x26,0xeb,0x6f,0x23,0xe3,0x87,0x7c,0xb2,0xed,0x17]);
        set.insert([0x57,0xa6,0x87,0x7d,0x0e,0x96,0xce,0x77,0xa3,0xfa,0xfb,0x0c,0x2f,0x7e,0x9e,0xd9,0xe2,0x8f,0xb0,0x37]);
        set.insert([0x5d,0x73,0xa5,0x89,0xb3,0x57,0xf4,0xe7,0xad,0x59,0x9b,0x4a,0x4a,0x7e,0x43,0x38,0xcd,0x73,0x30,0x18]);
        set.insert([0x67,0x44,0x97,0x91,0x8b,0x42,0x04,0xd3,0xe6,0x86,0xba,0x23,0x40,0x8a,0x9a,0xa2,0x16,0xb2,0x22,0x7a]);
        set.insert([0x68,0x92,0x8a,0xa2,0xc3,0x16,0x7d,0x75,0xd5,0x66,0xe5,0x4b,0x47,0x54,0x62,0xaa,0x28,0x72,0xcb,0x2f]);
        set.insert([0x6a,0x81,0xac,0x2a,0x32,0xcc,0x2e,0xac,0x4b,0x97,0x4b,0x18,0x19,0xed,0x2b,0x75,0x68,0xc8,0x3c,0x04]);
        set.insert([0x6c,0xe7,0x2a,0xa2,0x20,0x18,0x62,0x2c,0x24,0xa2,0xc0,0xaa,0xc4,0x5f,0x14,0x61,0x7a,0xec,0x59,0xe5]);
        set.insert([0x74,0x02,0xb6,0x3c,0x09,0xf3,0x52,0x09,0x31,0xda,0xc2,0xf9,0xf7,0x02,0xce,0x16,0x50,0x3d,0x36,0x48]);
        set.insert([0x78,0x27,0x70,0x25,0x3c,0x0a,0xe7,0x2e,0xdd,0x91,0x13,0x8a,0xd0,0x01,0x7c,0xdb,0x9a,0x7d,0x8d,0xba]);
        set.insert([0x79,0x17,0x73,0xaa,0x0b,0x8a,0xe2,0xdc,0x8c,0xb7,0x5e,0xb9,0xd5,0x64,0xa0,0xd5,0x98,0x12,0x55,0x72]);
        set.insert([0x7a,0x89,0xc9,0xdd,0x73,0x83,0xdd,0xe2,0x74,0x19,0xb3,0x6e,0x2c,0x6d,0x0d,0x94,0x2e,0xee,0x85,0xd9]);
        set.insert([0x80,0xfc,0xb4,0xf6,0xf0,0x86,0xe6,0xbb,0xd8,0x32,0x50,0x0c,0x2b,0x72,0x9c,0x26,0xb3,0xbf,0x1a,0xd2]);
        set.insert([0x81,0x3b,0xf8,0x20,0xfd,0x54,0x35,0xd0,0x3d,0xb7,0xbb,0xeb,0x04,0xd2,0xc3,0x42,0x17,0xf2,0xf9,0x49]);
        set.insert([0x84,0xca,0x01,0x28,0x46,0x31,0x54,0x15,0x12,0x65,0x21,0xd8,0xaf,0xdf,0x5d,0xda,0x5f,0x77,0x53,0x06]);
        set.insert([0x8f,0x06,0xc7,0xf4,0xf5,0xf8,0x68,0x42,0xae,0xe7,0x84,0x33,0x70,0x21,0xeb,0xd5,0x9d,0x4a,0xf9,0xf1]);
        set.insert([0x8f,0xc9,0x82,0x1d,0xbf,0xa8,0x3b,0x21,0x7f,0x8f,0x61,0xf1,0xf4,0x1f,0x05,0x74,0x25,0xdc,0xd3,0xb6]);
        set.insert([0x8f,0xfb,0xc5,0xef,0x59,0xef,0x9f,0x22,0x89,0xe7,0x4a,0x37,0x45,0x03,0x9d,0x90,0x0f,0xba,0x30,0xfe]);
        set.insert([0x90,0x48,0xc1,0x42,0xde,0x1f,0xe7,0x0d,0x3e,0xf4,0xa9,0x10,0x7c,0xcf,0xbe,0x23,0x5d,0xf5,0x36,0xc0]);
        set.insert([0x93,0xbd,0x0a,0x2c,0x4b,0x01,0xd3,0xfb,0x0f,0x21,0xb0,0x72,0xc0,0x4f,0xee,0xea,0x7e,0x64,0x9a,0xe2]);
        set.insert([0x97,0x58,0x7e,0x41,0x8d,0xc1,0xaf,0xbc,0xa9,0x93,0x9c,0x06,0xcc,0xb9,0x7f,0x31,0x15,0xc5,0x8c,0x65]);
        set.insert([0x98,0xcb,0x37,0x1d,0x43,0x68,0x2e,0xb8,0x7d,0x6f,0xb1,0xac,0x1c,0x95,0x89,0xd7,0x9f,0xcd,0x69,0xce]);
        set.insert([0x9a,0xce,0x47,0xea,0xcb,0xe6,0xab,0x51,0x12,0xbd,0x6d,0x8e,0xbc,0x55,0x5b,0x0f,0xef,0x9f,0x23,0x32]);
        set.insert([0x9c,0x80,0xbc,0x6b,0xf4,0x58,0x6e,0xdb,0x26,0x89,0xc3,0x09,0x3a,0x9a,0x0a,0xec,0xa3,0x01,0x48,0x05]);
        set.insert([0x9d,0x08,0x17,0x1e,0xc1,0xca,0xe6,0xce,0x1e,0xa0,0xce,0x3f,0x36,0x74,0xfa,0x5d,0x01,0xf6,0xf1,0xb3]);
        set.insert([0x9e,0xbd,0x6f,0x9e,0x55,0x0f,0x0d,0x6d,0xd0,0x0d,0xe5,0x25,0x5f,0x03,0xde,0xd6,0x1e,0x17,0xe2,0xc7]);
        set.insert([0xa0,0x2f,0x25,0xaf,0xce,0x5e,0xea,0xb3,0x21,0xa4,0xe7,0x69,0x43,0xe5,0x6d,0x6e,0x9a,0xa7,0x3f,0xe4]);
        set.insert([0xa0,0x41,0x7d,0x62,0x7d,0x22,0x5e,0x21,0x54,0x3c,0xf9,0xa9,0x1b,0x2d,0x83,0xb9,0x5f,0x09,0x1d,0xc6]);
        set.insert([0xa4,0x0b,0x94,0x76,0xfb,0x30,0x5c,0x1d,0x39,0x75,0x2f,0xd4,0x85,0x48,0xc5,0xfa,0x3c,0x37,0x15,0x71]);
        set.insert([0xa4,0x68,0x49,0x55,0x2c,0x73,0x63,0x28,0x5b,0xae,0xdb,0x5a,0x6b,0x68,0x14,0x80,0x9a,0x59,0xed,0x92]);
        set.insert([0xa6,0xad,0xab,0xb2,0xa2,0x5d,0x23,0x3c,0x62,0xaf,0xd1,0xfc,0x0e,0x99,0x1d,0x25,0x13,0xdc,0x11,0xd4]);
        set.insert([0xac,0x47,0xf3,0x11,0x51,0x09,0xba,0xeb,0x98,0xde,0xaf,0xdc,0xbd,0x98,0x01,0x17,0xb4,0xe2,0x86,0xba]);
        set.insert([0xaf,0x15,0x78,0xfb,0xef,0x76,0x67,0x2f,0xef,0x3b,0x92,0x50,0xb3,0x0c,0xef,0x40,0x7e,0x78,0xfd,0x26]);
        set.insert([0xb2,0xed,0x08,0x99,0x4a,0xe4,0xd6,0xdb,0x46,0xb3,0x16,0xd3,0x84,0x38,0x0f,0x69,0x63,0x64,0x04,0xfa]);
        set.insert([0xb3,0xc0,0xa1,0x40,0x68,0x94,0xe9,0x31,0x96,0x1e,0x0f,0xe3,0x4a,0x68,0xbd,0x01,0x48,0x33,0x7e,0x58]);
        set.insert([0xb4,0xcd,0xc2,0xe6,0x62,0x0d,0xfd,0x8c,0x5d,0x56,0x50,0x41,0x28,0x47,0x14,0xc3,0x80,0x79,0xc7,0x4c]);
        set.insert([0xb7,0xed,0x7f,0x4a,0x12,0x88,0xc9,0x8c,0x49,0xb4,0x6a,0x6a,0xf8,0x56,0x53,0x2e,0x65,0xea,0xed,0xab]);
        set.insert([0xbb,0x98,0x12,0x7d,0x89,0xb2,0xe3,0xf4,0xf9,0xdb,0x41,0x0b,0x06,0x0e,0x0a,0xff,0xa4,0x8e,0x07,0xef]);
        set.insert([0xbb,0xed,0x9a,0xc0,0xc3,0xb9,0x20,0x3d,0x4d,0xa1,0x9c,0xab,0x34,0xfb,0xfb,0x8c,0xe6,0xa4,0x41,0x1c]);
        set.insert([0xc1,0x33,0xf0,0xc3,0x75,0x1a,0x90,0x85,0x84,0xdc,0xdf,0x32,0x2d,0x72,0xb5,0x17,0x7c,0x12,0xe7,0x32]);
        set.insert([0xc2,0xf7,0xf3,0xb6,0x0a,0xcd,0xe8,0xdd,0xff,0x16,0x71,0x04,0xc5,0x6e,0x1e,0xd5,0xa1,0xe3,0xeb,0x74]);
        set.insert([0xc3,0x10,0x8f,0xa4,0x05,0xef,0x44,0xa7,0x1a,0x5d,0x88,0x03,0x21,0xe0,0x9a,0x62,0x0f,0x7f,0x4f,0x76]);
        set.insert([0xc3,0x1a,0x7d,0x5c,0x50,0x80,0xd9,0x0a,0x62,0x0d,0x36,0x48,0x54,0xba,0x33,0x6d,0xc8,0x2c,0xb2,0xc1]);
        set.insert([0xc4,0xad,0x39,0x80,0xab,0x87,0x72,0xd4,0xd6,0x3b,0x0a,0x7f,0x51,0xba,0xdf,0x00,0x65,0xdc,0x09,0x5a]);
        set.insert([0xc4,0xc1,0x07,0x66,0xc4,0x48,0x9b,0x71,0x77,0xcf,0x5a,0x21,0xa0,0xdc,0x48,0xe6,0x89,0x99,0x9a,0xe8]);
        set.insert([0xc4,0xc2,0xef,0x79,0x5e,0x37,0x11,0x16,0xee,0xa7,0xfe,0x8b,0x98,0x1e,0x38,0xb7,0xe0,0xcf,0x10,0x17]);
        set.insert([0xc6,0x45,0xec,0xce,0xbc,0x5a,0xa8,0x19,0x3b,0x4b,0xe8,0x5c,0x71,0xec,0x55,0x0f,0x6e,0x0b,0xf6,0xe9]);
        set.insert([0xc6,0x65,0x51,0xe2,0xc5,0x57,0x6b,0x91,0xe2,0x95,0x4d,0xca,0x76,0x79,0xa9,0x26,0x65,0xf5,0x89,0x4c]);
        set.insert([0xc8,0x63,0x8d,0xc5,0xe9,0x29,0x33,0x70,0x9f,0x64,0x7c,0xa7,0xab,0x78,0xee,0xb3,0x9d,0x39,0x95,0x75]);
        set.insert([0xcb,0xfd,0xda,0x92,0x33,0x07,0xf8,0xab,0x91,0x89,0x71,0x31,0xb6,0x13,0xb7,0xd8,0xe7,0xd6,0x22,0xe2]);
        set.insert([0xcd,0xd1,0x28,0x05,0x6d,0x8f,0x6b,0xb1,0xcb,0x31,0x17,0xf3,0x7f,0x0a,0xea,0xdc,0x1a,0x51,0xf2,0x9b]);
        set.insert([0xce,0x8a,0x5c,0x63,0x01,0xb1,0x8a,0xac,0xde,0x48,0xff,0x4c,0x83,0x8b,0x59,0xe3,0x87,0x63,0xe6,0x05]);
        set.insert([0xd0,0xc4,0x65,0x7e,0xb4,0x0a,0xdc,0x66,0x23,0x78,0xad,0x0b,0x6b,0x25,0x13,0xc6,0x13,0x8e,0xf5,0x51]);
        set.insert([0xd2,0x9e,0x7c,0x5a,0x8b,0x1c,0xbd,0xf6,0x36,0x29,0xf0,0x86,0x28,0x93,0xa8,0xf8,0x90,0x77,0xb4,0x1a]);
        set.insert([0xd7,0x54,0xcb,0xa1,0x3f,0xc2,0xe3,0x7b,0x0b,0x8f,0x92,0x10,0x65,0x21,0x45,0x0c,0x78,0x24,0xe8,0x5a]);
        set.insert([0xd7,0xda,0x52,0x0c,0xcc,0x85,0xd5,0x6c,0x19,0x25,0x6f,0xe6,0x78,0x52,0xe8,0x7a,0x3d,0x09,0x9d,0xe6]);
        set.insert([0xd9,0x63,0xe8,0x1a,0xcf,0x29,0x1c,0x7a,0xee,0xb5,0xfa,0x0f,0xf6,0x15,0x91,0xef,0xb8,0x6d,0x30,0xd2]);
        set.insert([0xdb,0x76,0x94,0xa4,0x75,0xe9,0xd6,0xaf,0x40,0x09,0xb6,0x0c,0x4b,0xe0,0xec,0x87,0xad,0x1a,0x6e,0xc6]);
        set.insert([0xdd,0x14,0xdb,0x55,0xa2,0x49,0x43,0x8d,0x2e,0xc6,0xd1,0xbe,0x1f,0x7c,0x1d,0x57,0x96,0x6b,0x9a,0xe8]);
        set.insert([0xdf,0x25,0xed,0x09,0xfc,0xa7,0x7a,0x19,0xd8,0x86,0xad,0x4f,0x2b,0xca,0x26,0x63,0xc0,0xa5,0xc1,0x13]);
        set.insert([0xe0,0xae,0x16,0xc0,0x87,0x51,0x05,0xd0,0xde,0x9c,0xa5,0xfe,0x53,0xa9,0x4c,0x93,0xa4,0x01,0xc9,0xb1]);
        set.insert([0xe0,0xd1,0x22,0xf6,0x11,0xe3,0x79,0x5b,0x28,0xa2,0x41,0x57,0xa9,0xc0,0x10,0x8b,0xca,0x57,0x92,0xf4]);
        set.insert([0xe2,0x55,0x63,0x2d,0xa8,0xcb,0x96,0x8a,0x9b,0x8a,0x7c,0xcc,0xb6,0xcf,0x02,0x0c,0xce,0xca,0xfe,0x22]);
        set.insert([0xe3,0x5a,0x36,0xf5,0xa1,0xee,0x2b,0xa8,0xfc,0x85,0x69,0x05,0x29,0x9c,0x84,0xf7,0x5b,0x61,0xe8,0x30]);
        set.insert([0xe5,0x71,0x83,0xd0,0xf0,0x63,0x54,0x86,0x89,0xc5,0xa1,0x4d,0x3b,0x72,0xd8,0x77,0x12,0x14,0x41,0xf1]);
        set.insert([0xe6,0x80,0x2c,0xc0,0x7c,0xa9,0xdc,0xc9,0x1e,0xde,0xa1,0xd5,0xa8,0x8a,0x59,0xa2,0x01,0x09,0x06,0x11]);
        set.insert([0xe8,0xbf,0x93,0x90,0xba,0x54,0xcd,0x80,0x00,0x13,0x08,0xc6,0xe2,0x57,0xac,0x77,0xd0,0x3c,0x61,0x8c]);
        set.insert([0xed,0xbe,0x8c,0xf5,0x9a,0x64,0x2b,0xac,0xd1,0x89,0xe4,0xe0,0x05,0x62,0xdf,0xab,0xae,0xfd,0x11,0x20]);
        set.insert([0xf0,0x2c,0x4e,0xef,0x50,0xb2,0xb4,0x66,0x0d,0x29,0x1c,0x81,0x36,0xa8,0x46,0x34,0x2f,0x79,0x0b,0x9d]);
        set.insert([0xf2,0x28,0xe8,0xbb,0x0a,0xea,0x2e,0x37,0x06,0x6f,0x93,0xdb,0x5c,0xbf,0xd9,0x34,0x2d,0x8f,0xff,0x0d]);
        set.insert([0xf4,0xe1,0x2f,0x72,0xac,0xcd,0xc8,0x3e,0xa3,0xe1,0x6b,0x73,0x07,0x79,0x69,0x0b,0x4a,0x9e,0x0b,0xed]);
        set.insert([0xf8,0xe6,0x1a,0x4a,0x80,0xc7,0x70,0x28,0xbe,0xb3,0x3a,0xe4,0xc4,0x6e,0x69,0xe6,0xf4,0x90,0x4e,0x61]);
        set.insert([0xf9,0x63,0xa2,0xbd,0x05,0x71,0x6c,0x42,0x9d,0x66,0x64,0x7b,0x6e,0xf1,0x53,0x87,0x47,0xcc,0xe5,0x5c]);
        set.insert([0xfc,0x2c,0x11,0xc9,0x5a,0x5a,0xad,0xfd,0x35,0x00,0x89,0x1e,0xce,0x06,0x65,0x1c,0x0b,0x4e,0xe0,0xe5]);
        set.insert([0xfe,0xc9,0x34,0x2e,0x9e,0xe4,0x18,0x64,0x53,0xf8,0xa7,0xe0,0x27,0xfa,0xc8,0xc2,0x4e,0x7c,0x0c,0x60]);


        set
    };

    static ref FMSPC_EOL: HashSet<&'static str> = HashSet::from([
        "00706A100000",
        "00706A800000",
        "00706E470000",
        "00806EA60000",
        "00806EB70000",
        "00906EA10000",
        "00906EA50000",
        "00906EB10000",
        "00906EC10000",
        "00906EC50000",
        "00906ED50000",
        "00A065510000",
        "20806EB70000",
        "20906EC10000",
    ]);
}

unsafe fn extract_fmspc_from_collateral(vec_coll: &[u8]) -> Option<String> {

    struct CollHdr {
        sizes: [u32; 8],
    }
    let i_tcb_idx = 5;
    
    let my_p_hdr = vec_coll.as_ptr() as *const CollHdr;

    let mut size0: u64 = mem::size_of::<CollHdr>() as u64;
    for i in 0..i_tcb_idx {
        size0 += (*my_p_hdr).sizes[i] as u64;
    }

    let size_tcb_info = (*my_p_hdr).sizes[i_tcb_idx];
    let size1 = size0 + size_tcb_info as u64;

    if (size1 > size0) && (size1 <= vec_coll.len() as u64) {
        let sub_slice = &vec_coll[size0 as usize .. (size1 - 1) as usize];

        let my_val: Result<serde_json::Value, _> = serde_json::from_slice(sub_slice);
        if let Ok(json_val) = my_val {

            // Navigate to fmspc
            let fmspc = &json_val["tcbInfo"]["fmspc"];
            if let Some(fmspc_str) = fmspc.as_str() {
                return Some(fmspc_str.to_string());
            }
        }
    }


    None

}


unsafe fn verify_fmspc_from_collateral(vec_coll: &[u8]) -> bool {

    if let Some(fmspc) = extract_fmspc_from_collateral(vec_coll) {

        let set = &FMSPC_EOL;
        let fmspc_str :&str = &fmspc;
        if set.contains(fmspc_str) {
            warn!("The CPU is deprecated");
        }
        // fmspc.starts_with("0090")

    } else {
        warn!("failed to fetch fmspc from attestation");
    }

    true
}

pub fn verify_quote_sgx(
    vec_quote: &[u8],
    vec_coll: &[u8],
    time_s: i64,
    check_ppid_wl: bool,
) -> Result<(sgx_report_body_t, sgx_ql_qv_result_t), sgx_status_t> {
    let qv_result = verify_quote_any(vec_quote, vec_coll, time_s)?;

    if vec_quote.len() < mem::size_of::<sgx_quote_t>() {
        trace!("Quote too small");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    let my_p_quote = vec_quote.as_ptr() as *const sgx_quote_t;

    unsafe {
        let version = (*my_p_quote).version;
        if version != 3 {
            trace!("Unrecognized quote version: {}", version);
            Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
        } else {
            let report_body = (*my_p_quote).report_body;

            if !verify_fmspc_from_collateral(vec_coll) {
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }

            let is_in_wl = match extract_cpu_cert_from_quote(vec_quote) {
                Some(ppid) => {
                    let ppid_addr = crate::registration::offchain::calculate_truncated_hash(&ppid);

                    let wl = &PPID_WHITELIST;
                    if wl.contains(&ppid_addr) {
                        true
                    } else {
                        println!("Unknown Machine ID: {}", orig_hex::encode(&ppid_addr));
                        false
                    }
                }
                None => {
                    println!("Machine ID couldn't be extracted");
                    false
                }
            };

            if check_ppid_wl && !is_in_wl {
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }

            Ok((report_body, qv_result))
        }
    }
}

#[cfg(feature = "SGX_MODE_HW")]
fn test_sgx_call_res(
    res: sgx_status_t,
    retval: sgx_status_t,
) -> Result<sgx_status_t, sgx_status_t> {
    if sgx_status_t::SGX_SUCCESS != res {
        return Err(res);
    }

    if sgx_status_t::SGX_SUCCESS != retval {
        return Err(retval);
    }

    Ok(sgx_status_t::SGX_SUCCESS)
}

#[cfg(not(feature = "SGX_MODE_HW"))]
pub fn get_quote_ecdsa(_pub_k: &[u8]) -> Result<(Vec<u8>, Vec<u8>), sgx_status_t> {
    Err(sgx_status_t::SGX_ERROR_NO_DEVICE)
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn get_quote_ecdsa_untested(pub_k: &[u8]) -> Result<(Vec<u8>, Vec<u8>), sgx_status_t> {
    let mut qe_target_info = sgx_target_info_t::default();
    let mut quote_size: u32 = 0;
    let mut rt: sgx_status_t = sgx_status_t::default();

    let mut res: sgx_status_t = unsafe {
        ocall_get_quote_ecdsa_params(
            &mut rt as *mut sgx_status_t,
            &mut qe_target_info,
            &mut quote_size,
        )
    };

    if let Err(e) = test_sgx_call_res(res, rt) {
        trace!("ocall_get_quote_ecdsa_params err = {}", e);
        return Err(e);
    }

    trace!("ECDSA quote size = {}", quote_size);

    let mut report_data: sgx_report_data_t = sgx_report_data_t::default();
    report_data.d[..pub_k.len()].copy_from_slice(pub_k);

    let my_report: sgx_report_t = match rsgx_create_report(&qe_target_info, &report_data) {
        Ok(r) => r,
        Err(e) => {
            trace!("sgx_create_report = {}", e);
            return Err(e);
        }
    };

    let mut vec_quote: Vec<u8> = vec![0; quote_size as usize];

    res = unsafe {
        ocall_get_quote_ecdsa(
            &mut rt as *mut sgx_status_t,
            &my_report,
            vec_quote.as_mut_ptr(),
            vec_quote.len() as u32,
        )
    };

    if let Err(e) = test_sgx_call_res(res, rt) {
        trace!("ocall_get_quote_ecdsa err = {}", e);
        return Err(e);
    }

    let mut vec_coll: Vec<u8> = vec![0; 0x4000];
    let mut size_coll: u32 = 0;

    res = unsafe {
        ocall_get_quote_ecdsa_collateral(
            &mut rt as *mut sgx_status_t,
            vec_quote.as_ptr(),
            vec_quote.len() as u32,
            vec_coll.as_mut_ptr(),
            vec_coll.len() as u32,
            &mut size_coll,
        )
    };

    if let Err(e) = test_sgx_call_res(res, rt) {
        trace!("ocall_get_quote_ecdsa_collateral err = {}", e);
        return Err(e);
    }

    trace!("Collateral size = {}", size_coll);

    let call_again = size_coll > vec_coll.len() as u32;
    vec_coll.resize(size_coll as usize, 0);

    if call_again {
        res = unsafe {
            ocall_get_quote_ecdsa_collateral(
                &mut rt as *mut sgx_status_t,
                vec_quote.as_ptr(),
                vec_quote.len() as u32,
                vec_coll.as_mut_ptr(),
                vec_coll.len() as u32,
                &mut size_coll,
            )
        };

        if let Err(e) = test_sgx_call_res(res, rt) {
            trace!("ocall_get_quote_ecdsa_collateral again err = {}", e);
            return Err(e);
        }
    }

    println!(
        "mr_signer = {}",
        orig_hex::encode(my_report.body.mr_signer.m)
    );
    println!(
        "mr_enclave = {}",
        orig_hex::encode(my_report.body.mr_enclave.m)
    );
    println!(
        "report_data = {}",
        orig_hex::encode(my_report.body.report_data.d)
    );

    Ok((vec_quote, vec_coll))
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn get_quote_ecdsa(pub_k: &[u8]) -> Result<(Vec<u8>, Vec<u8>), sgx_status_t> {
    let (vec_quote, vec_coll) = get_quote_ecdsa_untested(pub_k)?;

    // test self
    match verify_quote_sgx(&vec_quote, &vec_coll, 0, false) {
        Ok(r) => {
            trace!("Self quote verified ok");
            if r.1 != sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK {
                // TODO: strict policy wrt own quote verification
                trace!("WARNING: {}", r.1);
            }
        }
        Err(e) => {
            trace!("Self quote verification failed: {}", e);
            return Err(e);
        }
    };

    Ok((vec_quote, vec_coll))
}

//input: pub_k: &sgx_ec256_public_t, todo: make this the pubkey of the node
#[cfg(feature = "SGX_MODE_HW")]
pub fn create_attestation_report(
    pub_k: &[u8; 32],
    sign_type: sgx_quote_sign_type_t,
    api_key_file: &[u8],
    challenge: Option<&[u8]>,
    early: bool,
) -> Result<EndorsedAttestationReport, sgx_status_t> {
    // Workflow:
    // (1) ocall to get the target_info structure (ti) and epid group id (eg)
    // (1.5) get sigrl
    // (2) call sgx_create_report with ti+data, produce an sgx_report_t
    // (3) ocall to sgx_get_quote to generate (*mut sgx-quote_t, uint32_t)

    // (1) get ti + eg
    let mut ti: sgx_target_info_t = sgx_target_info_t::default();
    let mut eg: sgx_epid_group_id_t = sgx_epid_group_id_t::default();
    let mut rt: sgx_status_t = sgx_status_t::SGX_ERROR_UNEXPECTED;

    let res = unsafe {
        ocall_sgx_init_quote(
            &mut rt as *mut sgx_status_t,
            &mut ti as *mut sgx_target_info_t,
            &mut eg as *mut sgx_epid_group_id_t,
        )
    };

    trace!("EPID group = {:?}", eg);

    if res != sgx_status_t::SGX_SUCCESS {
        return Err(res);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        return Err(rt);
    }

    let eg_num = as_u32_le(eg);

    // (1.5) get sigrl
    let mut ias_sock: i32 = 0;

    let res =
        unsafe { ocall_get_ias_socket(&mut rt as *mut sgx_status_t, &mut ias_sock as *mut i32) };

    if res != sgx_status_t::SGX_SUCCESS {
        return Err(res);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        return Err(rt);
    }

    trace!("Got ias_sock successfully = {}", ias_sock);

    // Now sigrl_vec is the revocation list, a vec<u8>
    let sigrl_vec: Vec<u8> = get_sigrl_from_intel(ias_sock, eg_num, api_key_file);

    // (2) Generate the report
    // Fill ecc256 public key into report_data
    let mut report_data: sgx_report_data_t = sgx_report_data_t::default();

    report_data.d[..32].copy_from_slice(pub_k);
    if let Some(c) = challenge {
        report_data.d[32..36].copy_from_slice(c);
    }

    let rep = match rsgx_create_report(&ti, &report_data) {
        Ok(r) => {
            match SIGNING_METHOD {
                SigningMethod::MRENCLAVE => {
                    trace!(
                        "Report creation => success. Using MR_SIGNER: {:?}",
                        r.body.mr_signer.m
                    );
                }
                SigningMethod::MRSIGNER => {
                    trace!(
                        "Report creation => success. Got MR_ENCLAVE {:?}",
                        r.body.mr_signer.m
                    );
                }
                SigningMethod::NONE => {
                    trace!("Report creation => success. Not using any verification");
                }
            }
            r
        }
        Err(e) => {
            error!("Report creation => failed {:?}", e);
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    let mut quote_nonce = sgx_quote_nonce_t { rand: [0; 16] };
    let mut os_rng = os::SgxRng::new().unwrap();
    os_rng.fill_bytes(&mut quote_nonce.rand);
    trace!("Nonce generated successfully");
    let mut qe_report = sgx_report_t::default();
    const RET_QUOTE_BUF_LEN: u32 = 2048;
    let mut return_quote_buf: [u8; RET_QUOTE_BUF_LEN as usize] = [0; RET_QUOTE_BUF_LEN as usize];
    let mut quote_len: u32 = 0;

    // (3) Generate the quote
    // Args:
    //       1. sigrl: ptr + len
    //       2. report: ptr 432bytes
    //       3. linkable: u32, unlinkable=0, linkable=1
    //       4. spid: sgx_spid_t ptr 16bytes
    //       5. sgx_quote_nonce_t ptr 16bytes
    //       6. p_sig_rl + sigrl size ( same to sigrl)
    //       7. [out]p_qe_report need further check
    //       8. [out]p_quote
    //       9. quote_size
    let (p_sigrl, sigrl_len) = if sigrl_vec.is_empty() {
        (ptr::null(), 0)
    } else {
        (sigrl_vec.as_ptr(), sigrl_vec.len() as u32)
    };
    let p_report = (&rep) as *const sgx_report_t;
    let quote_type = sign_type;

    //&String::from_utf8_lossy(spid_file)
    let spid: sgx_spid_t = hex::decode_spid(SPID);

    let p_spid = &spid as *const sgx_spid_t;
    let p_nonce = &quote_nonce as *const sgx_quote_nonce_t;
    let p_qe_report = &mut qe_report as *mut sgx_report_t;
    let p_quote = return_quote_buf.as_mut_ptr();
    let maxlen = RET_QUOTE_BUF_LEN;
    let p_quote_len = &mut quote_len as *mut u32;

    let result = unsafe {
        ocall_get_quote(
            &mut rt as *mut sgx_status_t,
            p_sigrl,
            sigrl_len,
            p_report,
            quote_type,
            p_spid,
            p_nonce,
            p_qe_report,
            p_quote,
            maxlen,
            p_quote_len,
        )
    };

    if result != sgx_status_t::SGX_SUCCESS {
        warn!("ocall_get_quote returned {}", result);
        return Err(result);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        warn!("ocall_get_quote returned {}", rt);
        return Err(rt);
    }

    // Added 09-28-2018
    // Perform a check on qe_report to verify if the qe_report is valid
    match rsgx_verify_report(&qe_report) {
        Ok(()) => trace!("rsgx_verify_report passed!"),
        Err(x) => {
            warn!("rsgx_verify_report failed with {:?}", x);
            return Err(x);
        }
    }

    // Check if the qe_report is produced on the same platform
    if ti.mr_enclave.m != qe_report.body.mr_enclave.m
        || ti.attributes.flags != qe_report.body.attributes.flags
        || ti.attributes.xfrm != qe_report.body.attributes.xfrm
    {
        error!("qe_report does not match current target_info!");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    trace!("QE report check passed");

    // Debug
    // for i in 0..quote_len {
    //     print!("{:02X}", unsafe {*p_quote.offset(i as isize)});
    // }

    // Check qe_report to defend against replay attack
    // The purpose of p_qe_report is for the ISV enclave to confirm the QUOTE
    // it received is not modified by the untrusted SW stack, and not a replay.
    // The implementation in QE is to generate a REPORT targeting the ISV
    // enclave (target info from p_report) , with the lower 32Bytes in
    // report.data = SHA256(p_nonce||p_quote). The ISV enclave can verify the
    // p_qe_report and report.data to confirm the QUOTE has not be modified and
    // is not a replay. It is optional.

    let mut rhs_vec: Vec<u8> = quote_nonce.rand.to_vec();
    rhs_vec.extend(&return_quote_buf[..quote_len as usize]);
    let rhs_hash = rsgx_sha256_slice(&rhs_vec[..]).unwrap();
    let lhs_hash = &qe_report.body.report_data.d[..REPORT_DATA_SIZE];

    trace!("Report rhs hash = {:02X}", rhs_hash.iter().format(""));
    trace!("Report lhs hash = {:02X}", lhs_hash.iter().format(""));

    if rhs_hash != lhs_hash {
        error!("Quote is tampered!");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    let quote_vec: Vec<u8> = return_quote_buf[..quote_len as usize].to_vec();
    let res =
        unsafe { ocall_get_ias_socket(&mut rt as *mut sgx_status_t, &mut ias_sock as *mut i32) };

    if res != sgx_status_t::SGX_SUCCESS {
        return Err(res);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        return Err(rt);
    }

    let (attn_report, signature, signing_cert) =
        get_report_from_intel(ias_sock, quote_vec, api_key_file, early)?;
    Ok(EndorsedAttestationReport {
        report: attn_report.into_bytes(),
        signature,
        signing_cert,
    })
}

#[cfg(feature = "SGX_MODE_HW")]
fn parse_response_attn_report(resp: &[u8]) -> SgxResult<(String, Vec<u8>, Vec<u8>)> {
    trace!("parse_response_attn_report");
    let mut headers = [httparse::EMPTY_HEADER; 16];
    let mut respp = httparse::Response::new(&mut headers);
    let result = respp.parse(resp);
    trace!("parse result {:?}", result);

    match respp.code {
        Some(200) => info!("Response okay"),
        Some(401) => {
            error!("Unauthorized Failed to authenticate or authorize request.");
            return Err(sgx_status_t::SGX_ERROR_INVALID_ENCLAVE);
        }
        Some(404) => {
            error!("Not Found GID does not refer to a valid EPID group ID.");
            return Err(sgx_status_t::SGX_ERROR_INVALID_ENCLAVE);
        }
        Some(500) => {
            error!("Internal error occurred in IAS server");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
        Some(503) => {
            error!(
                "Service is currently not able to process the request (due to
            a temporary overloading or maintenance). This is a
            temporary state – the same request can be repeated after
            some time. "
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
        _ => {
            error!(
                "response from IAS server :{} - unknown error or response code",
                respp.code.unwrap()
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    }

    let mut len_num: u32 = 0;

    let mut sig = String::new();
    let mut cert = String::new();
    let mut attn_report = String::new();

    for i in 0..respp.headers.len() {
        let h = respp.headers[i];
        //println!("{} : {}", h.name, str::from_utf8(h.value).unwrap());
        match h.name {
            "Content-Length" => {
                let len_str = String::from_utf8(h.value.to_vec()).unwrap();
                len_num = len_str.parse::<u32>().unwrap();
                trace!("content length = {}", len_num);
            }
            "X-IASReport-Signature" => sig = str::from_utf8(h.value).unwrap().to_string(),
            "X-IASReport-Signing-Certificate" => {
                cert = str::from_utf8(h.value).unwrap().to_string()
            }
            _ => (),
        }
    }

    // Remove %0A from cert, and only obtain the signing cert
    cert = cert.replace("%0A", "");
    cert = hex::percent_decode(cert);

    let v: Vec<&str> = cert.split("-----").collect();

    if v.len() < 3 {
        error!("Error decoding response from IAS server");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }
    let sig_cert = v[2].to_string();

    if len_num != 0 {
        let header_len = result.unwrap().unwrap();
        let resp_body = &resp[header_len..];
        attn_report = str::from_utf8(resp_body).unwrap().to_string();
        info!("Attestation report: {}", attn_report);
    }

    let sig_bytes = base64::decode(sig).unwrap();
    let sig_cert_bytes = base64::decode(sig_cert).unwrap();
    // len_num == 0
    Ok((attn_report, sig_bytes, sig_cert_bytes))
}

#[cfg(feature = "SGX_MODE_HW")]
fn parse_response_sigrl(resp: &[u8]) -> Vec<u8> {
    trace!("parse_response_sigrl");
    let mut headers = [httparse::EMPTY_HEADER; 16];
    let mut respp = httparse::Response::new(&mut headers);
    let result = respp.parse(resp);
    trace!("parse result {:?}", result);
    trace!("parse response{:?}", respp);

    let msg: &'static str = match respp.code {
        Some(200) => "OK Operation Successful",
        Some(401) => "Unauthorized Failed to authenticate or authorize request.",
        Some(404) => "Not Found GID does not refer to a valid EPID group ID.",
        Some(500) => "Internal error occurred",
        Some(503) => {
            "Service is currently not able to process the request (due to
            a temporary overloading or maintenance). This is a
            temporary state – the same request can be repeated after
            some time. "
        }
        _ => "Unknown error occurred",
    };

    info!("{}", msg);
    let mut len_num: u32 = 0;

    for i in 0..respp.headers.len() {
        let h = respp.headers[i];
        if h.name == "content-length" {
            let len_str = String::from_utf8(h.value.to_vec()).unwrap();
            len_num = len_str.parse::<u32>().unwrap();
            trace!("content length = {}", len_num);
        }
    }

    if len_num != 0 {
        let header_len = result.unwrap().unwrap();
        let resp_body = &resp[header_len..];
        trace!("Base64-encoded SigRL: {:?}", resp_body);

        return base64::decode(str::from_utf8(resp_body).unwrap()).unwrap();
    }

    // len_num == 0
    Vec::new()
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn make_ias_client_config() -> rustls::ClientConfig {
    let mut config = rustls::ClientConfig::new();

    config
        .root_store
        .add_server_trust_anchors(&webpki_roots::TLS_SERVER_ROOTS);

    config
}

#[cfg(feature = "SGX_MODE_HW")]
#[allow(dead_code)]
pub fn get_gids_from_sn_tss(_fd: c_int, _cert: Vec<u8>) {
    // trace!("entered get_gids_from_sn_tss fd = {:?}", fd);
    // let config = make_ias_client_config();
    //
    // let cert_as_base64 = base64::encode(&cert);
    //
    // let req = format!(
    //     "POST {} HTTP/1.1\r\nHOST: {}\r\nConnection: Close\r\nAccept: */*\r\nContent-Type: application/json\r\nContent-Length: {}\r\n{}\r\n\r\n",
    //     SN_TSS_GID_LIST,
    //     SN_TSS_HOSTNAME,
    //     cert_as_base64.len(),
    //     cert_as_base64
    // );
    //
    // trace!("request to sn tss: {}", req);
    //
    // let dns_name = webpki::DNSNameRef::try_from_ascii_str(SN_TSS_HOSTNAME).unwrap();
    // let mut sess = rustls::ClientSession::new(&Arc::new(config), dns_name);
    // let mut sock = TcpStream::new(fd).unwrap();
    // let mut tls = rustls::Stream::new(&mut sess, &mut sock);
    //
    // let _result = tls.write(req.as_bytes());
    // let mut plaintext = Vec::new();
    //
    // info!("write complete");
    //
    // match tls.read_to_end(&mut plaintext) {
    //     Ok(_) => (),
    //     Err(e) => {
    //         warn!("get_gids_from_sn_tss tls.read_to_end: {:?}", e);
    //         panic!("Communication error with SN TSS");
    //     }
    // }
    // info!("read_to_end complete");
    // let resp_string = String::from_utf8(plaintext.clone()).unwrap();
    //
    // trace!("{}", resp_string);

    //resp_string

    //parse_response_sigrl(&plaintext)
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn get_sigrl_from_intel(fd: c_int, gid: u32, api_key_file: &[u8]) -> Vec<u8> {
    trace!("get_sigrl_from_intel fd = {:?}", fd);
    let config = make_ias_client_config();
    let ias_key = String::from_utf8_lossy(api_key_file).trim_end().to_owned();

    let req = format!("GET {}{:08x} HTTP/1.1\r\nHOST: {}\r\nOcp-Apim-Subscription-Key: {}\r\nConnection: Close\r\n\r\n",
                      SIGRL_SUFFIX,
                      gid,
                      DEV_HOSTNAME,
                      ias_key);

    trace!("get_sigrl_from_intel: {}", req);

    let dns_name = webpki::DNSNameRef::try_from_ascii_str(DEV_HOSTNAME).unwrap();
    let mut sess = rustls::ClientSession::new(&Arc::new(config), dns_name);
    let mut sock = TcpStream::new(fd).unwrap();
    let mut tls = rustls::Stream::new(&mut sess, &mut sock);

    let _result = tls.write(req.as_bytes());
    let mut plaintext = Vec::new();

    info!("write complete");

    match tls.read_to_end(&mut plaintext) {
        Ok(_) => (),
        Err(e) => {
            warn!("get_sigrl_from_intel tls.read_to_end: {:?}", e);
            panic!("Communication error with IAS");
        }
    }
    info!("read_to_end complete");
    let resp_string = String::from_utf8(plaintext.clone()).unwrap();

    trace!("{}", resp_string);

    // resp_string

    parse_response_sigrl(&plaintext)
}

// TODO: support pse
#[cfg(feature = "SGX_MODE_HW")]
pub fn get_report_from_intel(
    fd: c_int,
    quote: Vec<u8>,
    api_key_file: &[u8],
    early: bool,
) -> SgxResult<(String, Vec<u8>, Vec<u8>)> {
    trace!("get_report_from_intel fd = {:?}", fd);
    let config = make_ias_client_config();
    let encoded_quote = base64::encode(&quote[..]);
    let encoded_json = format!("{{\"isvEnclaveQuote\":\"{}\"}}\r\n", encoded_quote);
    let ias_key = String::from_utf8_lossy(api_key_file).trim_end().to_owned();

    let endpoint = if early {
        REPORT_SUFFIX
    } else {
        LEGACY_REPORT_SUFFIX
    };

    let req = format!("POST {} HTTP/1.1\r\nHOST: {}\r\nOcp-Apim-Subscription-Key:{}\r\nContent-Length:{}\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{}",
                      endpoint,
                      DEV_HOSTNAME,
                      ias_key,
                      encoded_json.len(),
                      encoded_json);

    trace!("{}", req);
    let dns_name = webpki::DNSNameRef::try_from_ascii_str(DEV_HOSTNAME).unwrap();
    let mut sess = rustls::ClientSession::new(&Arc::new(config), dns_name);
    let mut sock = TcpStream::new(fd).unwrap();
    let mut tls = rustls::Stream::new(&mut sess, &mut sock);

    let _result = tls.write(req.as_bytes());
    let mut plaintext = Vec::new();

    info!("write complete");

    tls.read_to_end(&mut plaintext).unwrap();
    info!("read_to_end complete");
    let resp_string = String::from_utf8(plaintext.clone()).unwrap();

    trace!("resp_string = {}", resp_string);

    parse_response_attn_report(&plaintext)
}

#[cfg(feature = "SGX_MODE_HW")]
fn as_u32_le(array: [u8; 4]) -> u32 {
    (array[0] as u32)
        + ((array[1] as u32) << 8)
        + ((array[2] as u32) << 16)
        + ((array[3] as u32) << 24)
}
