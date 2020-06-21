#![cfg_attr(not(feature = "SGX_MODE_HW"), allow(unused))]

use bit_vec::BitVec;
use chrono::Utc as TzUtc;
use chrono::{Duration, TimeZone};
#[cfg(feature = "SGX_MODE_HW")]
use itertools::Itertools;
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
use std::untrusted::time::SystemTimeEx;
use yasna::models::ObjectIdentifier;

use crate::consts::CERTEXPIRYDAYS;
#[cfg(feature = "SGX_MODE_HW")]
use crate::consts::{SigningMethod, SIGNING_METHOD};

#[cfg(feature = "SGX_MODE_HW")]
use super::report::{AttestationReport, SgxQuoteStatus};

extern "C" {
    #[allow(dead_code)]
    pub fn ocall_get_update_info(
        ret_val: *mut sgx_status_t,
        platformBlob: *const sgx_platform_info_t,
        enclaveTrusted: i32,
        update_info: *mut sgx_update_info_bit_t,
    ) -> sgx_status_t;
}

pub const IAS_REPORT_CA: &[u8] = include_bytes!("../../Intel_SGX_Attestation_RootCA.pem");

// todo: replace this with MRSIGNER/MRENCLAVE
pub const ENCLAVE_SIGNATURE: &[u8] = include_bytes!("../../Intel_SGX_Attestation_RootCA.pem");

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
                            writer.next().write_utf8_string(&ISSUER);
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
                            writer.next().write_utf8_string(&SUBJECT);
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
                ecc_handle.ecdsa_sign_slice(tbs, &prv_k).unwrap()
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

    let mut ca_reader = BufReader::new(&IAS_REPORT_CA[..]);

    let mut root_store = rustls::RootCertStore::empty();
    root_store
        .add_pem_file(&mut ca_reader)
        .expect("Failed to add CA");

    (ias_cert_dec, root_store)
}

#[cfg(not(feature = "SGX_MODE_HW"))]
pub fn verify_ra_cert(cert_der: &[u8]) -> SgxResult<Vec<u8>> {
    let payload =
        get_netscape_comment(cert_der).map_err(|_err| sgx_status_t::SGX_ERROR_UNEXPECTED)?;

    let pk = base64::decode(&payload).unwrap();

    Ok(pk)
}

/// # Verifies remote attestation cert
///
/// Logic:
/// 1. Extract public key
/// 2. Extract netscape comment == attestation report
/// 3.
///
#[cfg(feature = "SGX_MODE_HW")]
pub fn verify_ra_cert(cert_der: &[u8]) -> SgxResult<Vec<u8>> {
    // Before we reach here, Webpki already verifed the cert is properly signed

    let report =
        AttestationReport::from_cert(cert_der).map_err(|_| sgx_status_t::SGX_ERROR_UNEXPECTED)?;

    // 2. Verify quote status (mandatory field)

    match report.sgx_quote_status {
        SgxQuoteStatus::OK => (),
        SgxQuoteStatus::SwHardeningNeeded => {
            warn!("Attesting enclave is vulnerable, and should be patched");
        }
        _ => {
            error!("Invalid attestation quote status - cannot verify remote node");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    }

    match SIGNING_METHOD {
        SigningMethod::MRENCLAVE => {
            // todo: fill this in some time
            debug!("Validating using MRENCLAVE");
            if report.sgx_quote_body.isv_enclave_report.mr_enclave != ENCLAVE_SIGNATURE {
                error!("Remote node signature MRENCLAVE is different from expected");
                debug!(
                    "sgx quote mr_enclave = {:02x}",
                    report
                        .sgx_quote_body
                        .isv_enclave_report
                        .mr_enclave
                        .iter()
                        .format("")
                );
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }
        }
        SigningMethod::MRSIGNER => {
            // todo: fill this in some time
            debug!("Validating using MRSIGNER");
            if report.sgx_quote_body.isv_enclave_report.mr_signer != ENCLAVE_SIGNATURE {
                error!("Remote node signature MRSIGNER is different from expected");
                debug!(
                    "sgx quote mr_signer = {:02x}",
                    report
                        .sgx_quote_body
                        .isv_enclave_report
                        .mr_signer
                        .iter()
                        .format("")
                );
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }
        }
        _ => debug!("Ignoring signing method validation"),
    }

    let report_public_key = report.sgx_quote_body.isv_enclave_report.report_data[0..32].to_vec();
    Ok(report_public_key)
}

#[cfg(feature = "test")]
pub mod tests {
    use crate::crypto::KeyPair;

    use super::sgx_quote_sign_type_t;
    use super::verify_ra_cert;

    fn test_validate_certificate_valid_sw_mode() {
        pub const cert: &[u8] = include_bytes!("../testdata/attestation_cert");
        let result = verify_ra_cert(cert);
    }

    fn test_validate_certificate_valid_signed() {
        pub const cert: &[u8] = include_bytes!("../testdata/attestation_cert.der");
        let result = verify_ra_cert(cert);
    }

    fn test_validate_certificate_invalid() {
        pub const cert: &[u8] = include_bytes!("../testdata/attestation_cert_invalid");
        let result = verify_ra_cert(cert);
    }

    // we want a weird test because this should never crash
    fn test_random_bytes_as_certificate() {}
}
