use log::error;
use sgx_types::sgx_status_t;
use tendermint::block::signed_header::SignedHeader;
use tendermint::block::{Commit, Header};
use tendermint::validator::Set;
use tendermint_light_client_verifier::types::UntrustedBlockState;
use tendermint_proto::Protobuf;

use crate::verify::block::verify_block;

pub fn validate_block_header(
    block_header_slice: &[u8],
    validator_set: &Set,
    height: u64,
    commit: Commit,
) -> Result<SignedHeader, sgx_status_t> {
    let header = Header::decode(block_header_slice).map_err(|e| {
        error!("Error parsing header from proto: {:?}", e);
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    })?;

    let signed_header = SignedHeader::new(header, commit).map_err(|e| {
        error!("Error creating signed header: {:?}", e);
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    })?;

    // validate that we have the validator set for the current height
    if signed_header.header.height.value() != height {
        error!("Validator set height does not match stored validator set");
        // we use this error code to signal that the validator set is not synced with the current block
        return Err(sgx_status_t::SGX_ERROR_FILE_RECOVERY_NEEDED);
    }

    let untrusted_block = UntrustedBlockState {
        signed_header: &signed_header,
        validators: validator_set,
        next_validators: None,
    };

    let result = verify_block(&untrusted_block);

    if !result {
        error!("Error verifying block header!");
        return Err(sgx_status_t::SGX_ERROR_INVALID_SIGNATURE);
    }

    Ok(signed_header)
}
