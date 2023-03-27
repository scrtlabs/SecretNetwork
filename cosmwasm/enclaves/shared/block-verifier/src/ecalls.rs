use std::slice;
use tendermint::block::Commit;
use tendermint::block::Header;
use tendermint_proto::Protobuf;

use sgx_types::sgx_status_t;

use crate::{txs, verify_block};
use log::{debug, error};
use std::io::Stderr;

use enclave_utils::{validate_const_ptr, validate_input_length, validate_mut_ptr};
use tendermint::block::signed_header::SignedHeader;
use tendermint::validator::Set;
use tendermint::Hash;
use tendermint::Hash::Sha256;

#[cfg(feature = "random")]
use enclave_crypto::{SIVEncryptable, KEY_MANAGER};

use crate::txs::txs_hash;

#[cfg(feature = "light-client-validation")]
use crate::txs::tx_from_bytes;
#[cfg(feature = "light-client-validation")]
use crate::wasm_messages::VERIFIED_MESSAGES;

#[cfg(feature = "random")]
use crate::random::create_proof;

use enclave_utils::validator_set::ValidatorSetForHeight;

const MAX_VARIABLE_LENGTH: u32 = 100_000;
const RANDOM_PROOF_LEN: u32 = 80;
const MAX_TXS_LENGTH: u32 = 10 * 1024 * 1024;

#[cfg(feature = "light-client-validation")]
const TX_THRESHOLD: usize = 100_000;

/// # Safety
///  This function reads buffers which must be correctly initialized by the caller,
/// see safety section of slice::[from_raw_parts](https://doc.rust-lang.org/std/slice/fn.from_raw_parts.html#safety)
///
#[no_mangle]
#[allow(unused_variables)]
pub unsafe extern "C" fn ecall_submit_block_signatures(
    in_header: *const u8,
    in_header_len: u32,
    in_commit: *const u8,
    in_commit_len: u32,
    in_txs: *const u8,
    in_txs_len: u32,
    in_encrypted_random: *const u8,
    in_encrypted_random_len: u32,
    decrypted_random: &mut [u8; 32],
) -> sgx_status_t {
    validate_inputs(
        in_header,
        in_header_len,
        in_commit,
        in_commit_len,
        in_txs,
        in_txs_len,
        in_encrypted_random,
        in_encrypted_random_len,
        decrypted_random,
    )?;

    let block_header_slice = slice::from_raw_parts(in_header, in_header_len as usize);
    let block_commit_slice = slice::from_raw_parts(in_commit, in_commit_len as usize);

    let txs_slice = if in_txs_len != 0 && !in_txs.is_null() {
        slice::from_raw_parts(in_txs, in_txs_len as usize)
    } else {
        &[]
    };

    let validator_set_for_height: ValidatorSetForHeight =
        get_validator_set_for_height(block_header_slice)?;

    let header = crate::verify::header::validate_block_header(
        &block_header_slice,
        &validator_set_for_height,
    )?;
    let commit = crate::verify::commit::validate_commit(&block_commit_slice)?;

    crate::verify::txs::validate_txs(&txs_slice, &header)?;

    #[cfg(feature = "light-client-validation")]
    {
        verify_and_append_txs_to_verified_messages(&txs_slice)?;
    }

    #[cfg(feature = "random")]
    {
        let encrypted_random_slice =
            slice::from_raw_parts(in_encrypted_random, in_encrypted_random_len as usize);
        let validator_hash = get_validator_hash(&validator_set_for_height)?;

        validate_encrypted_random(
            &encrypted_random_slice,
            validator_set_for_height.hash(),
            signed_header.header.app_hash.as_bytes(),
            signed_header.header.height.value(),
        )?;

        let decrypted = decrypt_random(
            &KEY_MANAGER.random_encryption_key.unwrap(),
            &encrypted_random_slice,
            validator_hash.as_bytes(),
        )?;
        decrypted_random.copy_from_slice(&*decrypted);
    }

    debug!(
        "Done verifying block height: {:?}",
        validator_set_for_height.height
    );

    sgx_status_t::SGX_SUCCESS
}

fn get_validator_set_for_height(
    block_header_slice: &[u8],
) -> Result<ValidatorSetForHeight, sgx_status_t> {
    let validator_set_result = ValidatorSetForHeight::unseal()?;

    // validate that we have the validator set for the current height
    if block_header_slice.get_height().value() != validator_set_result.height {
        error!("Validator set height does not match stored validator set");
        // we use this error code to signal that the validator set is not synced with the current block
        return Err(sgx_status_t::SGX_ERROR_FILE_RECOVERY_NEEDED);
    }

    Ok(validator_set_result)
}

#[cfg(feature = "random")]
fn validate_encrypted_random(
    encrypted_random_slice: &[u8],
    validator_set_hash: Hash,
    app_hash: &[u8],
    height: u64,
) -> Result<[u8; 32], sgx_status_t> {
    let encrypted_random_slice = encrypted_random_slice
        .get(..48)
        .ok_or_else(|| sgx_status_t::SGX_ERROR_INVALID_PARAMETER)?;
    let rand_proof = encrypted_random_slice
        .get(48..)
        .ok_or_else(|| sgx_status_t::SGX_ERROR_INVALID_PARAMETER)?;

    let calculated_proof =
        crate::verify::random::create_proof(height, app_hash, &encrypted_random_slice);

    if calculated_proof != rand_proof {
        error!("Error validating random");
        return Err(sgx_status_t::SGX_ERROR_INVALID_SIGNATURE);
    }

    println!(
        "Encrypted random slice len: {}",
        encrypted_random_slice.len()
    );

    let decrypted = KEY_MANAGER
        .random_encryption_key
        .as_ref()
        .ok_or_else(|| sgx_status_t::SGX_ERROR_INVALID_PARAMETER)?
        .decrypt_siv(
            &encrypted_random_slice,
            Some(&[validator_set_hash.as_bytes()]),
        )
        .map_err(|_| {
            error!("Error decrypting random slice");
            sgx_status_t::SGX_ERROR_INVALID_SIGNATURE
        })?;

    let mut decrypted_random = [0u8; 32];
    decrypted_random.copy_from_slice(&*decrypted);
    Ok(decrypted_random)
}

#[cfg(not(feature = "random"))]
fn validate_encrypted_random(
    _encrypted_random_slice: &[u8],
    _validator_set: &Set,
    _app_hash: &[u8],
) -> Result<[u8; 32], Box<dyn Error>> {
    Ok([0u8; 32])
}

fn validate_inputs(
    in_header: *const u8,
    in_header_len: u32,
    in_commit: *const u8,
    in_commit_len: u32,
    in_txs: *const u8,
    in_txs_len: u32,
    in_encrypted_random: *const u8,
    in_encrypted_random_len: u32,
    decrypted_random: &mut [u8; 32],
) -> Result<(), sgx_status_t> {
    validate_input_length!(in_header_len, "header", MAX_VARIABLE_LENGTH);
    validate_input_length!(in_commit_len, "commit", MAX_VARIABLE_LENGTH);
    validate_input_length!(in_txs_len, "txs", MAX_TXS_LENGTH);
    validate_input_length!(
        in_encrypted_random_len,
        "encrypted random",
        RANDOM_PROOF_LEN
    );

    validate_const_ptr!(
        in_header,
        in_header_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );
    validate_const_ptr!(
        in_commit,
        in_commit_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    #[cfg(feature = "random")]
    validate_const_ptr!(
        in_encrypted_random,
        in_encrypted_random_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    validate_mut_ptr!(
        decrypted_random.as_mut_ptr(),
        decrypted_random.len(),
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    if in_txs_len != 0 && !in_txs.is_null() {
        validate_const_ptr!(
            in_txs,
            in_txs_len as usize,
            sgx_status_t::SGX_ERROR_INVALID_PARAMETER
        );
    }

    Ok(())
}
