#![cfg(feature = "random")]

use enclave_crypto::SIVEncryptable;
use enclave_utils::{Keychain, KEY_MANAGER};
use log::{debug, error};
use sgx_types::sgx_status_t;
use tendermint::Hash;

pub fn validate_random_proof(
    random_slice: &[u8],
    proof_slice: &[u8],
    block_hash_slice: &[u8],
    height: u64,
) -> Result<Option<u16>, sgx_status_t> {
    let irs = KEY_MANAGER.initial_randomness_seed.unwrap();
    let calculated_proof =
        enclave_utils::random::create_random_proof(&irs, height, random_slice, block_hash_slice);

    if calculated_proof == proof_slice {
        return Ok(None);
    }

    println!("************* validate_random_proof failed with latest seed");

    // try older seeds
    let seeds = KEY_MANAGER.get_consensus_seed().unwrap();
    let extra = KEY_MANAGER.extra_data.lock().unwrap();

    println!(
        "************* Total seeds: {}, last used: {}",
        seeds.arr.len(),
        extra.last_block_seed
    );

    for i_seed in extra.last_block_seed..seeds.arr.len() as u16 {
        let randomness_seed = Keychain::generate_randomness_seed(&seeds.arr[i_seed as usize]);

        let calculated_proof_prev = enclave_utils::random::create_random_proof(
            &randomness_seed,
            height,
            random_slice,
            block_hash_slice,
        );

        if calculated_proof_prev == proof_slice {
            println!("************** succeeded to verify with seed {}", i_seed);
            return Ok(Some(i_seed));
        }

        println!(
            "************* validate_random failed to verify with seed {}",
            i_seed
        );
    }

    let legacy_proof =
        enclave_utils::random::create_legacy_proof(&irs, height, random_slice, block_hash_slice);
    if legacy_proof == proof_slice {
        return Ok(None);
    }

    error!("Error validating random");

    Err(sgx_status_t::SGX_ERROR_INVALID_SIGNATURE)
}

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

    let i_seed_option =
        match validate_random_proof(encrypted_random_slice, rand_proof, app_hash, height) {
            Ok(x) => x,
            Err(e) => {
                return Err(e);
            }
        };

    debug!(
        "Encrypted random slice len: {}",
        encrypted_random_slice.len()
    );

    let random_key = if let Some(i_seed) = i_seed_option {
        Keychain::generate_random_key(
            &KEY_MANAGER.get_consensus_seed().unwrap().arr[i_seed as usize],
        )
    } else {
        KEY_MANAGER.random_encryption_key.unwrap()
    };

    let decrypted = random_key
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
