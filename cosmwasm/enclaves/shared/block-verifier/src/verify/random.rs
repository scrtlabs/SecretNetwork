#![cfg(feature = "random")]

use enclave_crypto::{SIVEncryptable, KEY_MANAGER};
use enclave_utils::random::{create_legacy_proof, create_random_proof};
use log::{debug, error};
use sgx_types::sgx_status_t;
use tendermint::Hash;

pub fn validate_encrypted_random(
    random_and_proof: &[u8],
    validator_set_hash: Hash,
    app_hash: &[u8],
    height: u64,
) -> Result<[u8; 32], sgx_status_t> {
    let encrypted_random_slice = random_and_proof
        .get(..48)
        .ok_or(sgx_status_t::SGX_ERROR_INVALID_PARAMETER)?;
    let rand_proof = random_and_proof
        .get(48..)
        .ok_or(sgx_status_t::SGX_ERROR_INVALID_PARAMETER)?;

    let irs = KEY_MANAGER.initial_randomness_seed.unwrap();
    let calculated_proof = create_random_proof(&irs, height, encrypted_random_slice, app_hash);

    if calculated_proof != rand_proof {
        let legacy_proof = create_legacy_proof(&irs, height, encrypted_random_slice, app_hash);

        if legacy_proof != rand_proof {
            error!(
                "Error validating random: {:?} != {:?} != {:?}",
                calculated_proof, rand_proof, legacy_proof
            );
            return Err(sgx_status_t::SGX_ERROR_INVALID_SIGNATURE);
        }
    }

    debug!(
        "Encrypted random slice len: {}",
        encrypted_random_slice.len()
    );

    let decrypted = KEY_MANAGER
        .random_encryption_key
        .as_ref()
        .ok_or(sgx_status_t::SGX_ERROR_INVALID_PARAMETER)?
        .decrypt_siv(
            encrypted_random_slice,
            Some(&[validator_set_hash.as_bytes()]),
        )
        .map_err(|_| {
            error!("Error decrypting random slice");
            sgx_status_t::SGX_ERROR_INVALID_SIGNATURE
        })?;

    let mut decrypted_random = [0u8; 32];
    decrypted_random.copy_from_slice(&decrypted);
    Ok(decrypted_random)
}
