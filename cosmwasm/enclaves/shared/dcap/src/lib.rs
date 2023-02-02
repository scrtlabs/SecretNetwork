#[allow(dead_code)]
mod generate;
mod ocalls;
//mod types;
// mod verify;

// use dcap_ql::quote::Quote;
use sgx_types::{sgx_quote_sign_type_t, sgx_status_t};

use secret_attestation_token::{NodeAuthPublicKey, SecretAttestationToken, VerificationError};

extern crate sgx_tse;
extern crate sgx_types;

#[allow(const_err)]
pub fn generate_authentication_material(
    _pub_k: &[u8; 32],
    _sign_type: sgx_quote_sign_type_t,
    _api_key_file: &[u8],
) -> Result<SecretAttestationToken, sgx_status_t> {
    Ok(SecretAttestationToken::default())
}

pub fn verify_authentication_material(
    _material: &SecretAttestationToken,
) -> Result<NodeAuthPublicKey, VerificationError> {
    Ok(NodeAuthPublicKey::default())
}

// #[cfg(feature = "SGX_MODE_HW")]
// #[allow(const_err)]
// pub fn create_attestation_report(
//     pub_k: &UserData,
//     sign_type: sgx_quote_sign_type_t,
//     api_key_file: &[u8],
// ) -> SgxResult<SecretAttestationToken> {
//     let quote = generate_quote(pub_k)?;
//
//     let parsed = Quote::parse(&quote).unwrap();
//
//     parsed.signature();
//
//     // this quote has type `sgx_quote3_t` and is structured as:
//     // sgx_quote3_t {
//     //     header: sgx_quote_header_t,
//     //     report_body: sgx_report_body_t,
//     //     signature_data_len: uint32_t,  // 1116
//     //     signature_data {               // 1116 bytes payload
//     //         sig_data: sgx_ql_ecdsa_sig_data_t { // 576 = 64x3 +384 header
//     //             sig: [uint8_t; 64],
//     //             attest_pub_key: [uint8_t; 64],
//     //             qe3_report: sgx_report_body_t, //  384
//     //             qe3_report_sig: [uint8_t; 64],
//     //             auth_certification_data { // 2 + 32 = 34
//     //                 sgx_ql_auth_data_t: u16 // observed 32, size of following auth_data
//     //                 auth_data: [u8; sgx_ql_auth_data_t]
//     //             }
//     //             sgx_ql_certification_data_t {/ 2 + 4 + 500
//     //                 cert_key_type: uint16_t,
//     //                 size: uint32_t, // observed 500, size of following certificateion_data
//     //                 certification_data { // 500 bytes
//     //                 }
//     //             }
//     //         }
//     //     }
//     //  }
//
//     let report = SecretAttestationToken::default();
//     Ok(report)
// }
