#![cfg_attr(not(feature = "SGX_MODE_HW"), allow(unused))]

use bit_vec::BitVec;
use chrono::Utc as TzUtc;
use chrono::{Duration, TimeZone};
#[cfg(feature = "SGX_MODE_HW")]
use log::*;
use num_bigint::BigUint;
use sgx_tcrypto::SgxEccHandle;
use sgx_types::{
    sgx_ec256_private_t, sgx_ec256_public_t, sgx_platform_info_t, sgx_status_t,
    sgx_update_info_bit_t, SgxResult,
};

use std::io::BufReader;
use std::str;
use std::time::{SystemTime, UNIX_EPOCH};
use yasna::models::ObjectIdentifier;

use enclave_crypto::consts::{SigningMethod, CERTEXPIRYDAYS};
#[cfg(feature = "SGX_MODE_HW")]
use enclave_crypto::consts::{MRSIGNER, SIGNING_METHOD};
use enclave_ffi_types::NodeAuthResult;

use crate::registration::report::AdvisoryIDs;

#[cfg(feature = "SGX_MODE_HW")]
use super::attestation::get_mr_enclave;
#[cfg(feature = "SGX_MODE_HW")]
use super::report::{AttestationReport, SgxQuoteStatus};

extern "C" {
    pub fn ocall_get_update_info(
        ret_val: *mut sgx_status_t,
        platformBlob: *const sgx_platform_info_t,
        enclaveTrusted: i32,
        update_info: *mut sgx_update_info_bit_t,
    ) -> sgx_status_t;
}

pub const IAS_REPORT_CA: &[u8] = include_bytes!("../../Intel_SGX_Attestation_RootCA.pem");

const ISSUER: &str = "SecretTEE";
const SUBJECT: &str = "Secret Network Node Certificate";

pub enum Error {
    GenericError,
}

pub fn gen_ecc_cert(
    payload: String,
    prv_k: &sgx_ec256_private_t,
    pub_k: &sgx_ec256_public_t,
    ecc_handle: &SgxEccHandle,
) -> SgxResult<(Vec<u8>, Vec<u8>)> {
    // Generate public key bytes since both DER will use it
    let mut pub_key_bytes: Vec<u8> = vec![4];
    let mut pk_gx = pub_k.gx;
    pk_gx.reverse();
    let mut pk_gy = pub_k.gy;
    pk_gy.reverse();
    pub_key_bytes.extend_from_slice(&pk_gx);
    pub_key_bytes.extend_from_slice(&pk_gy);

    // Generate Certificate DER
    let cert_der = yasna::construct_der(|writer| {
        writer.write_sequence(|writer| {
            writer.next().write_sequence(|writer| {
                // Certificate Version
                writer
                    .next()
                    .write_tagged(yasna::Tag::context(0), |writer| {
                        writer.write_i8(2);
                    });
                // Certificate Serial Number (unused but required)
                writer.next().write_u8(1);
                // Signature Algorithm: ecdsa-with-SHA256
                writer.next().write_sequence(|writer| {
                    writer
                        .next()
                        .write_oid(&ObjectIdentifier::from_slice(&[1, 2, 840, 10045, 4, 3, 2]));
                });
                // Issuer: CN=MesaTEE (unused but required)
                writer.next().write_sequence(|writer| {
                    writer.next().write_set(|writer| {
                        writer.next().write_sequence(|writer| {
                            writer
                                .next()
                                .write_oid(&ObjectIdentifier::from_slice(&[2, 5, 4, 3]));
                            writer.next().write_utf8_string(ISSUER);
                        });
                    });
                });
                // Validity: Issuing/Expiring Time (unused but required)
                let now = SystemTime::now().duration_since(UNIX_EPOCH).unwrap();
                let issue_ts = TzUtc.timestamp(now.as_secs() as i64, 0);
                let expire = now + Duration::days(CERTEXPIRYDAYS).to_std().unwrap();
                let expire_ts = TzUtc.timestamp(expire.as_secs() as i64, 0);
                writer.next().write_sequence(|writer| {
                    writer
                        .next()
                        .write_utctime(&yasna::models::UTCTime::from_datetime(&issue_ts));
                    writer
                        .next()
                        .write_utctime(&yasna::models::UTCTime::from_datetime(&expire_ts));
                });
                // Subject: CN=MesaTEE (unused but required)
                writer.next().write_sequence(|writer| {
                    writer.next().write_set(|writer| {
                        writer.next().write_sequence(|writer| {
                            writer
                                .next()
                                .write_oid(&ObjectIdentifier::from_slice(&[2, 5, 4, 3]));
                            writer.next().write_utf8_string(SUBJECT);
                        });
                    });
                });
                writer.next().write_sequence(|writer| {
                    // Public Key Algorithm
                    writer.next().write_sequence(|writer| {
                        // id-ecPublicKey
                        writer
                            .next()
                            .write_oid(&ObjectIdentifier::from_slice(&[1, 2, 840, 10045, 2, 1]));
                        // prime256v1
                        writer
                            .next()
                            .write_oid(&ObjectIdentifier::from_slice(&[1, 2, 840, 10045, 3, 1, 7]));
                    });
                    // Public Key
                    writer
                        .next()
                        .write_bitvec(&BitVec::from_bytes(&pub_key_bytes));
                });
                // Certificate V3 Extension
                writer
                    .next()
                    .write_tagged(yasna::Tag::context(3), |writer| {
                        writer.write_sequence(|writer| {
                            writer.next().write_sequence(|writer| {
                                writer.next().write_oid(&ObjectIdentifier::from_slice(&[
                                    2, 16, 840, 1, 113_730, 1, 13,
                                ]));
                                writer.next().write_bytes(&payload.into_bytes());
                            });
                        });
                    });
            });
            // Signature Algorithm: ecdsa-with-SHA256
            writer.next().write_sequence(|writer| {
                writer
                    .next()
                    .write_oid(&ObjectIdentifier::from_slice(&[1, 2, 840, 10045, 4, 3, 2]));
            });
            // Signature
            let sig = {
                let tbs = &writer.buf[4..];
                ecc_handle.ecdsa_sign_slice(tbs, prv_k).unwrap()
            };
            let sig_der = yasna::construct_der(|writer| {
                writer.write_sequence(|writer| {
                    let mut sig_x = sig.x;
                    sig_x.reverse();
                    let mut sig_y = sig.y;
                    sig_y.reverse();
                    writer.next().write_biguint(&BigUint::from_slice(&sig_x));
                    writer.next().write_biguint(&BigUint::from_slice(&sig_y));
                });
            });
            writer.next().write_bitvec(&BitVec::from_bytes(&sig_der));
        });
    });

    // Generate Private Key DER
    let key_der = yasna::construct_der(|writer| {
        writer.write_sequence(|writer| {
            writer.next().write_u8(0);
            writer.next().write_sequence(|writer| {
                writer
                    .next()
                    .write_oid(&ObjectIdentifier::from_slice(&[1, 2, 840, 10045, 2, 1]));
                writer
                    .next()
                    .write_oid(&ObjectIdentifier::from_slice(&[1, 2, 840, 10045, 3, 1, 7]));
            });
            let inner_key_der = yasna::construct_der(|writer| {
                writer.write_sequence(|writer| {
                    writer.next().write_u8(1);
                    let mut prv_k_r = prv_k.r;
                    prv_k_r.reverse();
                    writer.next().write_bytes(&prv_k_r);
                    writer
                        .next()
                        .write_tagged(yasna::Tag::context(1), |writer| {
                            writer.write_bitvec(&BitVec::from_bytes(&pub_key_bytes));
                        });
                });
            });
            writer.next().write_bytes(&inner_key_der);
        });
    });

    Ok((key_der, cert_der))
}

fn extract_asn1_value(cert: &[u8], oid: &[u8]) -> Result<Vec<u8>, Error> {
    let mut offset = match cert.windows(oid.len()).position(|window| window == oid) {
        Some(size) => size,
        None => {
            return Err(Error::GenericError);
        }
    };

    offset += 12; // 11 + TAG (0x04)

    if offset + 2 >= cert.len() {
        return Err(Error::GenericError);
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
        return Err(Error::GenericError);
    }

    let payload = cert[offset..offset + len].to_vec();

    Ok(payload)
}

pub fn get_netscape_comment(cert_der: &[u8]) -> Result<Vec<u8>, Error> {
    // Search for Netscape Comment OID
    let ns_cmt_oid = &[
        0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x86, 0xF8, 0x42, 0x01, 0x0D,
    ];
    extract_asn1_value(cert_der, ns_cmt_oid)
}

#[allow(dead_code)]
pub fn get_cert_pubkey(cert_der: &[u8]) -> Result<Vec<u8>, Error> {
    // Search for Public Key prime256v1 OID
    let prime256v1_oid = &[0x06, 0x08, 0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x03, 0x01, 0x07];
    extract_asn1_value(cert_der, prime256v1_oid)
}

pub fn get_ias_auth_config() -> (Vec<u8>, rustls::RootCertStore) {
    // Verify if the signing cert is issued by Intel CA
    let mut ias_ca_stripped = IAS_REPORT_CA.to_vec();
    ias_ca_stripped.retain(|&x| x != 0x0d && x != 0x0a);
    let head_len = "-----BEGIN CERTIFICATE-----".len();
    let tail_len = "-----END CERTIFICATE-----".len();
    let full_len = ias_ca_stripped.len();
    let ias_ca_core: &[u8] = &ias_ca_stripped[head_len..full_len - tail_len];
    let ias_cert_dec = base64::decode_config(ias_ca_core, base64::STANDARD).unwrap();

    let mut ca_reader = BufReader::new(IAS_REPORT_CA);

    let mut root_store = rustls::RootCertStore::empty();
    root_store
        .add_pem_file(&mut ca_reader)
        .expect("Failed to add CA");

    (ias_cert_dec, root_store)
}

#[cfg(not(feature = "SGX_MODE_HW"))]
pub fn verify_ra_cert(
    cert_der: &[u8],
    override_verify: Option<SigningMethod>,
    _check_tcb_version: bool,
) -> Result<Vec<u8>, NodeAuthResult> {
    let payload = get_netscape_comment(cert_der).map_err(|_err| NodeAuthResult::InvalidCert)?;

    let pk = base64::decode(&payload).map_err(|_err| NodeAuthResult::InvalidCert)?;

    Ok(pk)
}

/// # Verifies remote attestation cert
///
/// Logic:
/// 1. Extract public key
/// 2. Extract netscape comment - where the attestation report is located
/// 3. Parse the report itself (verify it is signed by intel)
/// 4. Extract public key from report body
/// 5. Verify enclave signature (mr enclave/signer)
///
#[cfg(feature = "SGX_MODE_HW")]
pub fn verify_ra_cert(
    cert_der: &[u8],
    override_verify_type: Option<SigningMethod>,
    check_tcb_version: bool,
) -> Result<Vec<u8>, NodeAuthResult> {
    let report = AttestationReport::from_cert(cert_der).map_err(|_| NodeAuthResult::InvalidCert)?;

    // this is a small hack - override_verify_type is only used when verifying the master certificate
    // and in that case we don't care about checking vulns etc. Master certificate will also have
    // a bad GID in prod, so there's no reason to verify it
    if override_verify_type.is_none() {
        verify_quote_status(&report, &report.advisory_ids)?;
    }

    let signing_method: SigningMethod = match override_verify_type {
        Some(method) => method,
        None => SIGNING_METHOD,
    };

    // verify certificate
    match signing_method {
        SigningMethod::MRENCLAVE => {
            let this_mr_enclave = get_mr_enclave();
            let this_mr_signer = MRSIGNER;

            let crate::registration::report::SgxEnclaveReport {
                mr_enclave: report_mr_enclave,
                mr_signer: report_mr_signer,
                ..
            } = report.sgx_quote_body.isv_enclave_report;

            if report_mr_enclave != this_mr_enclave || report_mr_signer != this_mr_signer {
                error!(
                    "Got a different mr_enclave or mr_signer than expected. Invalid certificate"
                );
                warn!(
                    "mr_enclave: received: {:?} \n expected: {:?}",
                    report_mr_enclave, this_mr_enclave
                );
                warn!(
                    "mr_signer: received: {:?} \n expected: {:?}",
                    report_mr_signer, this_mr_signer
                );
                return Err(NodeAuthResult::MrEnclaveMismatch);
            }

            if check_tcb_version {
                // todo: change this to a parameters or const when we migrate the code to main
                if report.tcb_eval_data_number < 16 {
                    info!("Got an outdated certificate");
                    return Err(NodeAuthResult::GroupOutOfDate);
                }
            }
        }
        SigningMethod::MRSIGNER => {
            if report.sgx_quote_body.isv_enclave_report.mr_signer != MRSIGNER {
                error!("Got a different mrsigner than expected. Invalid certificate");
                warn!(
                    "received: {:?} \n expected: {:?}",
                    report.sgx_quote_body.isv_enclave_report.mr_signer, MRSIGNER
                );
                return Err(NodeAuthResult::MrSignerMismatch);
            }
        }
        SigningMethod::NONE => {}
    }

    let report_public_key = report.sgx_quote_body.isv_enclave_report.report_data[0..32].to_vec();
    Ok(report_public_key)
}

// fn transform_u32_to_array_of_u8(x: u32) -> [u8; 4] {
//     let b1: u8 = ((x >> 24) & 0xff) as u8;
//     let b2: u8 = ((x >> 16) & 0xff) as u8;
//     let b3: u8 = ((x >> 8) & 0xff) as u8;
//     let b4: u8 = (x & 0xff) as u8;
//     return [b1, b2, b3, b4];
// }

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
pub fn verify_quote_status(
    report: &AttestationReport,
    advisories: &AdvisoryIDs,
) -> Result<NodeAuthResult, NodeAuthResult> {
    // info!(
    //     "Got GID: {:?}",
    //     transform_u32_to_array_of_u8(report.sgx_quote_body.gid)
    // );
    #[cfg(not(feature = "epid_whitelist_disabled"))]
    if !check_epid_gid_is_whitelisted(&report.sgx_quote_body.gid) {
        error!(
            "Platform verification error: quote status {:?}",
            &report.sgx_quote_body.gid
        );
        return Err(NodeAuthResult::BadQuoteStatus);
    }

    match &report.sgx_quote_status {
        SgxQuoteStatus::OK
        | SgxQuoteStatus::SwHardeningNeeded
        | SgxQuoteStatus::ConfigurationAndSwHardeningNeeded => {
            check_advisories(&report.sgx_quote_status, advisories)?;

            Ok(NodeAuthResult::Success)
        }
        _ => {
            error!(
                "Invalid attestation quote status - cannot verify remote node: {:?}",
                &report.sgx_quote_status
            );
            Err(NodeAuthResult::from(&report.sgx_quote_status))
        }
    }
}

// the difference here is that we allow GROUP_OUT_OF_DATE for testnet machines to make joining a bit
// easier
#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub fn verify_quote_status(
    report: &AttestationReport,
    advisories: &AdvisoryIDs,
) -> Result<NodeAuthResult, NodeAuthResult> {
    match &report.sgx_quote_status {
        SgxQuoteStatus::OK
        | SgxQuoteStatus::SwHardeningNeeded
        | SgxQuoteStatus::ConfigurationAndSwHardeningNeeded
        | SgxQuoteStatus::GroupOutOfDate => {
            let results = check_advisories(&report.sgx_quote_status, advisories);

            if let Err(results) = results {
                warn!("This platform has vulnerabilities that will not be approved on mainnet");
                return Ok(results); // Allow in non-production
            }

            // if !advisories.contains_lvi_injection() {
            //     return Err(NodeAuthResult::EnclaveQuoteStatus);
            // }

            Ok(NodeAuthResult::Success)
        }
        _ => {
            error!(
                "Invalid attestation quote status - cannot verify remote node: {:?}",
                &report.sgx_quote_status
            );
            Err(NodeAuthResult::from(&report.sgx_quote_status))
        }
    }
}

#[cfg(all(feature = "SGX_MODE_HW", feature = "production", not(feature = "test")))]
#[allow(dead_code)]
const WHITELIST_FROM_FILE: &str = include_str!("../../whitelist.txt");

#[cfg(all(
    not(all(feature = "SGX_MODE_HW", feature = "production")),
    feature = "test"
))]
const WHITELIST_FROM_FILE: &str = include_str!("fixtures/test_whitelist.txt");

#[cfg(not(feature = "epid_whitelist_disabled"))]
pub fn check_epid_gid_is_whitelisted(epid_gid: &u32) -> bool {
    let decoded = base64::decode(WHITELIST_FROM_FILE.trim()).unwrap(); //will never fail since data is constant
    decoded.as_chunks::<4>().0.iter().any(|&arr| {
        if epid_gid == &u32::from_be_bytes(arr) {
            return true;
        }
        false
    })
}

#[cfg(feature = "SGX_MODE_HW")]
fn check_advisories(
    quote_status: &SgxQuoteStatus,
    advisories: &AdvisoryIDs,
) -> Result<(), NodeAuthResult> {
    // this checks if there are any vulnerabilities that are not on in the whitelisted list
    let vulnerable = advisories.vulnerable();
    if vulnerable.is_empty() {
        Ok(())
    } else {
        error!("Platform is updated but requires further BIOS configuration");
        error!(
            "The following vulnerabilities must be mitigated: {:?}",
            vulnerable
        );
        Err(NodeAuthResult::from(quote_status))
    }
}

#[cfg(feature = "test")]
pub mod tests {
    use std::io::Read;
    use std::untrusted::fs::File;

    use enclave_ffi_types::NodeAuthResult;

    use crate::registration::report::AttestationReport;

    use super::verify_ra_cert;

    // #[cfg(feature = "SGX_MODE_HW")]
    // fn tls_ra_cert_der_out_of_date() -> Vec<u8> {
    //     let mut cert = vec![];
    //     let mut f =
    //         File::open("../execute/src/registration/fixtures/attestation_cert_out_of_date.der")
    //             .unwrap();
    //     f.read_to_end(&mut cert).unwrap();
    //
    //     cert
    // }

    fn tls_ra_cert_der_sw_config_needed() -> Vec<u8> {
        let mut cert = vec![];
        let mut f = File::open(
            "../execute/src/registration/fixtures/attestation_cert_sw_config_needed.der",
        )
        .unwrap();
        f.read_to_end(&mut cert).unwrap();

        cert
    }

    #[cfg(feature = "SGX_MODE_HW")]
    fn tls_ra_cert_der_valid() -> Vec<u8> {
        let mut cert = vec![];
        let mut f =
            File::open("../execute/src/registration/fixtures/attestation_cert_hw_v2").unwrap();
        f.read_to_end(&mut cert).unwrap();

        cert
    }

    #[cfg(not(feature = "SGX_MODE_HW"))]
    fn tls_ra_cert_der_valid() -> Vec<u8> {
        let mut cert = vec![];
        let mut f = File::open("../execute/src/registration/fixtures/attestation_cert_sw").unwrap();
        f.read_to_end(&mut cert).unwrap();

        cert
    }

    #[cfg(not(feature = "SGX_MODE_HW"))]
    pub fn test_certificate_invalid_configuration_needed() {}

    #[cfg(feature = "SGX_MODE_HW")]
    pub fn test_certificate_invalid_configuration_needed() {
        let tls_ra_cert = tls_ra_cert_der_sw_config_needed();
        let report = AttestationReport::from_cert(&tls_ra_cert);
        assert!(report.is_ok());

        let res = verify_ra_cert(&tls_ra_cert, None, false);

        assert!(res.is_ok());

        // assert_eq!(result, NodeAuthResult::SwHardeningAndConfigurationNeeded)
    }

    // #[cfg(not(feature = "SGX_MODE_HW"))]
    // pub fn test_certificate_invalid_group_out_of_date() {}
    //
    // #[cfg(feature = "SGX_MODE_HW")]
    // pub fn test_certificate_invalid_group_out_of_date() {
    //     let tls_ra_cert = tls_ra_cert_der_out_of_date();
    //     let report = AttestationReport::from_cert(&tls_ra_cert);
    //     assert!(report.is_ok());
    //
    //     let result =
    //         verify_ra_cert(&tls_ra_cert, None).expect_err("Certificate should not pass validation");
    //
    //     assert_eq!(result, NodeAuthResult::GroupOutOfDate)
    // }

    #[cfg(not(feature = "epid_whitelist_disabled"))]
    pub fn test_epid_whitelist() {
        // check that we parse this correctly
        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0xc12 as u32));
        assert_eq!(res, true);

        // check that 2nd number works
        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0x6942 as u32));
        assert_eq!(res, true);

        // check all kinds of failures that should return false
        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0x0 as u32));
        assert_eq!(res, false);

        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0x120c as u32));
        assert_eq!(res, false);

        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0xc120000 as u32));
        assert_eq!(res, false);

        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0x1242 as u32));
        assert_eq!(res, false);
    }

    pub fn test_certificate_valid() {
        let tls_ra_cert = tls_ra_cert_der_valid();
        let _ = verify_ra_cert(&tls_ra_cert, None, false).unwrap();
    }
}
