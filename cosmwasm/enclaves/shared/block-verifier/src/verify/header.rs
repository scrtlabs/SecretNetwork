use crate::{txs, verify_block};
use enclave_utils::validator_set::ValidatorSetForHeight;
use log::{debug, error};
use sgx_types::sgx_status_t;
use tendermint::block::signed_header::SignedHeader;
use tendermint::block::{Commit, Header};
use tendermint::validator::Set;
use tendermint::Hash::Sha256;
use tendermint_light_client_verifier::types::UntrustedBlockState;
use tendermint_proto::Protobuf;

pub fn validate_block_header(
    block_header_slice: &[u8],
    validator_set_for_height: &ValidatorSetForHeight,
) -> Result<SignedHeader, sgx_status_t> {
    let header = Header::decode(block_header_slice).map_err(|e| {
        error!("Error parsing header from proto: {:?}", e);
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    })?;

    // validate the tx bytes with the hash in the header
    let txs_slice = block_header_slice.get_data();
    let txs = txs::txs_from_bytes(txs_slice).map_err(|e| {
        error!("Error parsing txs from proto: {:?}", e);
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    })?;
    let calculated_tx_hash = txs::txs_hash(&txs);
    if Some(Sha256(calculated_tx_hash)) != header.data_hash {
        error!("Error verifying data hash");
        return Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER);
    }

    let validator_set =
        Set::decode(validator_set_for_height.validator_set.as_slice()).map_err(|e| {
            error!("Error parsing validator set from proto: {:?}", e);
            sgx_status_t::SGX_SUCCESS
        })?;

    let signed_header = SignedHeader::new(header, Commit::default()).map_err(|e| {
        error!("Error creating signed header: {:?}", e);
        sgx_status_t::SGX_SUCCESS
    })?;

    let untrusted_block = UntrustedBlockState {
        signed_header: &signed_header,
        validators: &validator_set,
        next_validators: None,
    };

    let result = verify_block(&untrusted_block);

    if !result {
        error!("Error verifying block header!");
        return Err(sgx_status_t::SGX_ERROR_INVALID_SIGNATURE);
    }

    debug!(
        "Done verifying block height: {:?}",
        validator_set_for_height.height
    );

    Ok(signed_header)
}
