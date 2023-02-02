mod epid_quote;
mod generate_hw;
mod generate_sw;
mod ias;
pub mod ocalls;
mod print;
pub mod types;

use log::error;

use crate::types::EndorsedEpidAttestationReport;
use secret_attestation_token::{
    AttestationNonce, AttestationType, AuthenticationMaterialVerify, FromAttestationToken,
    NodeAuthPublicKey, SecretAttestationToken, VerificationError,
};

use sgx_types::{sgx_quote_sign_type_t, sgx_status_t};

#[cfg(not(target_env = "sgx"))]
#[macro_use]
extern crate sgx_tstd as std;

extern crate core;
extern crate sgx_rand;
extern crate sgx_tcrypto;
extern crate sgx_tse;
extern crate sgx_types;
// #[cfg(feature = "SGX_MODE_HW")]
// use super::report::EndorsedAttestationReport;

/// extra_data size that will store the public key of the attesting node
// #[cfg(feature = "SGX_MODE_HW")]
const REPORT_DATA_SIZE: usize = 32;

/// this creates data structure that will be used by the network to validate the node
/// this function is called during the "Generation" phase
#[allow(const_err)]
pub fn generate_authentication_material(
    pub_k: &[u8; 32],
    sign_type: sgx_quote_sign_type_t,
    api_key_file: &[u8],
    auth_type: AttestationType,
    optional_nonce: Option<AttestationNonce>,
) -> Result<SecretAttestationToken, sgx_status_t> {
    match auth_type {
        AttestationType::SgxEpid => generate_hw::impl_generate_authentication_material_hw(
            pub_k,
            sign_type,
            api_key_file,
            optional_nonce,
        ),

        AttestationType::SgxSw => generate_sw::impl_generate_authentication_material_sw(pub_k),
        _ => {
            error!("Unsupported EPID authentication type");
            Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER)
        }
    }
}

#[allow(const_err)]
pub fn verify_authentication_material(
    material: &SecretAttestationToken,
) -> Result<NodeAuthPublicKey, VerificationError> {
    match material.attestation_type {
        AttestationType::SgxEpid => {
            let report = EndorsedEpidAttestationReport::from_attestation_token(material);
            report.verify()
        }
        AttestationType::SgxSw => Ok(material.node_key),
        _ => {
            error!("Unsupported EPID authentication type");
            Err(VerificationError::ErrorGeneric)
        }
    }
}
