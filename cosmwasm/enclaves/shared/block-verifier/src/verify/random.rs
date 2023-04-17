#![cfg(feature = "random")]

use enclave_crypto::{sha_256, SIVEncryptable, KEY_MANAGER};
use log::{debug, error, trace};
use sgx_types::sgx_status_t;
use tendermint::Hash;

pub fn create_proof(height: u64, random: &[u8], block_hash: &[u8]) -> [u8; 32] {
    trace!(
        "Height: {:?}\nRandom: {:?}\nApphash: {:?}",
        height,
        random,
        block_hash
    );
    let irs = KEY_MANAGER.initial_randomness_seed.unwrap();

    let height_bytes = height.to_be_bytes();
    let irs_bytes = irs.get();

    let data_len = height_bytes.len() + random.len() + block_hash.len() + irs_bytes.len();
    let mut data = Vec::with_capacity(data_len);

    data.extend_from_slice(&height_bytes);
    data.extend_from_slice(random);
    data.extend_from_slice(block_hash);
    data.extend_from_slice(irs_bytes);

    sha_256(data.as_slice())
}

#[cfg(feature = "random")]
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

    let calculated_proof = create_proof(height, encrypted_random_slice, app_hash);

    if calculated_proof != rand_proof {
        error!(
            "Error validating random: {:?} != {:?}",
            calculated_proof, rand_proof
        );
        return Err(sgx_status_t::SGX_ERROR_INVALID_SIGNATURE);
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
