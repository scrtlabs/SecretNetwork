use secret_attestation_token::{AttestationType, SecretAttestationToken};
use sgx_types::sgx_status_t;

pub fn impl_generate_authentication_material_sw(
    pub_k: &[u8; 32],
) -> Result<SecretAttestationToken, sgx_status_t> {
    Ok(SecretAttestationToken {
        attestation_type: AttestationType::SgxSw,
        data: pub_k.to_vec(),
        node_key: pub_k.clone(),
        block_info: Default::default(),
        signature: vec![],
        signing_cert: vec![],
    })
}
