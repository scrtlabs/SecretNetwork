use std::slice;
use tendermint::block::Commit;
use tendermint::block::Header;
use tendermint_proto::Protobuf;

use sgx_types::sgx_status_t;

use crate::r#const::VALIDATOR_SET_SEALING_PATH;
use crate::verify_block;
use enclave_crypto::SIVEncryptable;
use enclave_crypto::KEY_MANAGER;
use log::{debug, error};

use tendermint::block::signed_header::SignedHeader;
use tendermint::validator::Set;

#[no_mangle]
pub unsafe extern "C" fn ecall_submit_block_signatures(
    in_header: *const u8,
    in_header_len: u32,
    in_commit: *const u8,
    in_commit_len: u32,
    in_encrypted_random: *const u8,
    in_encrypted_random_len: u32,
    decrypted_random: &mut [u8; 32],
    // in_validator_set: *const u8,
    // in_validator_set_len: u32,
    // in_next_validator_set: *const u8,
    // in_next_validator_set_len: u32,
) -> sgx_status_t {
    let block_header_slice = slice::from_raw_parts(in_header, in_header_len as usize);
    let block_commit_slice = slice::from_raw_parts(in_commit, in_commit_len as usize);
    let encrypted_random_slice =
        slice::from_raw_parts(in_encrypted_random, in_encrypted_random_len as usize);
    // let validator_set_slice =
    //     slice::from_raw_parts(in_validator_set, in_validator_set_len as usize);
    // let next_validator_set_slice =
    //     slice::from_raw_parts(in_next_validator_set, in_next_validator_set_len as usize);

    let validator_set_result = enclave_utils::storage::unseal(&VALIDATOR_SET_SEALING_PATH);

    if validator_set_result.is_err() {
        return validator_set_result.unwrap_err();
    }
    let validator_set_slice = validator_set_result.unwrap();

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

    let validator_set = if let Ok(r) = Set::decode(validator_set_slice.as_slice()) {
        r
    } else {
        error!("Error parsing header from proto");
        return sgx_status_t::SGX_SUCCESS;
    };

    // let next_validator_set = if let Ok(r) = Set::decode(next_validator_set_slice) {
    //     r
    // } else {
    //     error!("Error parsing header from proto");
    //     return sgx_status_t::SGX_SUCCESS;
    // };
    // let commit = if let Ok(r) = Commit::decode(block_commit_slice) {
    //     r
    // } else {
    //     error!("Error parsing commit from proto");
    //     return sgx_status_t::SGX_SUCCESS;
    // };
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

    decrypted_random.copy_from_slice(&*decrypted);

    debug!("Done verifying block: {:?}", result);

    sgx_status_t::SGX_SUCCESS
}
