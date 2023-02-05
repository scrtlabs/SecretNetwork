use std::slice;
use tendermint::block::Commit;
use tendermint::block::Header;
use tendermint_proto::Protobuf;

use sgx_types::sgx_status_t;

use crate::verify_block;
use enclave_crypto::SIVEncryptable;
use enclave_crypto::KEY_MANAGER;
use log::{debug, error};

use tendermint::block::signed_header::SignedHeader;
use tendermint::validator::Set;

use enclave_utils::validator_set::ValidatorSetForHeight;

#[no_mangle]
pub unsafe extern "C" fn ecall_submit_block_signatures(
    in_header: *const u8,
    in_header_len: u32,
    in_commit: *const u8,
    in_commit_len: u32,
    in_encrypted_random: *const u8,
    in_encrypted_random_len: u32,
    decrypted_random: &mut [u8; 32],
) -> sgx_status_t {
    let block_header_slice = slice::from_raw_parts(in_header, in_header_len as usize);
    let block_commit_slice = slice::from_raw_parts(in_commit, in_commit_len as usize);
    let encrypted_random_slice =
        slice::from_raw_parts(in_encrypted_random, in_encrypted_random_len as usize);

    let validator_set_result = ValidatorSetForHeight::unseal();
    if validator_set_result.is_err() {
        return validator_set_result.unwrap_err();
    }
    let validator_set_for_height: ValidatorSetForHeight = validator_set_result.unwrap();

    // As of now this is not working because of a difference in behavior between tendermint and tendermint-rs
    // Ref: https://github.com/informalsystems/tendermint-rs/issues/1255
    let header = if let Ok(r) = Header::decode(block_header_slice) {
        r
    } else {
        error!("Error parsing header from proto");
        return sgx_status_t::SGX_SUCCESS;
    };

    let commit = if let Ok(res) = Commit::decode(block_commit_slice).map_err(|e| {
        error!("Error parsing commit from proto: {:?}", e);
        sgx_status_t::SGX_SUCCESS
    }) {
        res
    } else {
        return sgx_status_t::SGX_SUCCESS;
    };

    if header.height.value() != validator_set_for_height.height {
        error!("Validator set height does not match stored validator set");
        // we use this error code to signal that the validator set is not synced with the current block
        return sgx_status_t::SGX_ERROR_FILE_RECOVERY_NEEDED;
    }

    let validator_set =
        if let Ok(r) = Set::decode(validator_set_for_height.validator_set.as_slice()) {
            r
        } else {
            error!("Error parsing header from proto");
            return sgx_status_t::SGX_SUCCESS;
        };

    let validator_hash = validator_set.hash();

    let signed_header = SignedHeader::new(header, commit).unwrap();
    let untrusted_block = tendermint_light_client_verifier::types::UntrustedBlockState {
        signed_header: &signed_header,
        validators: &validator_set,
        next_validators: None,
    };
    let result = verify_block(&untrusted_block);

    if !result {
        error!("Error verifying encrypted random!");
        return sgx_status_t::SGX_ERROR_INVALID_SIGNATURE;
    }

    let decrypted = match KEY_MANAGER
        .random_encryption_key
        .unwrap()
        .decrypt_siv(encrypted_random_slice, Some(&[validator_hash.as_bytes()]))
    {
        Ok(res) => res,
        Err(_) => {
            error!("Error decrypting random slice");
            return sgx_status_t::SGX_ERROR_INVALID_SIGNATURE;
        }
    };

    // this is here so that the decrypted randomness is a function of the validator hash
    // that way an attempt to fuck with the block verification by submitting a fake validator set and signatures
    // will result in different random values
    let decrypted_randomness =
        enclave_crypto::hkdf_sha_256(decrypted, &[validator_hash.as_bytes()]);

    decrypted_random.copy_from_slice(&*decrypted_randomness);

    debug!("Done verifying block: {:?}", result);

    sgx_status_t::SGX_SUCCESS
}
