use std::slice;

use tendermint_proto::Protobuf;

use sgx_types::sgx_status_t;

use enclave_utils::{validate_const_ptr, validate_input_length, validate_mut_ptr};
use log::error;

use log::debug;

use tendermint::validator::Set;

macro_rules! unwrap_or_return {
    ($result:expr) => {
        match $result {
            Ok(commit) => commit,
            Err(e) => return e.into(),
        }
    };
}

use crate::txs::tx_from_bytes;
use crate::wasm_messages::VERIFIED_MESSAGES;

use crate::verify::validator_set::get_validator_set_for_height;
use enclave_utils::validator_set::ValidatorSetForHeight;

const MAX_VARIABLE_LENGTH: u32 = 100_000;
const RANDOM_PROOF_LEN: u32 = 80;
const MAX_TXS_LENGTH: u32 = 10 * 1024 * 1024;
const TX_THRESHOLD: usize = 100_000;

#[no_mangle]
#[allow(unused_variables)]
pub unsafe fn submit_block_signatures_impl(
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
    if let Err(e) = validate_inputs(
        in_header,
        in_header_len,
        in_commit,
        in_commit_len,
        in_txs,
        in_txs_len,
        in_encrypted_random,
        in_encrypted_random_len,
        decrypted_random,
    ) {
        return e;
    }

    let block_header_slice = slice::from_raw_parts(in_header, in_header_len as usize);
    let block_commit_slice = slice::from_raw_parts(in_commit, in_commit_len as usize);

    // todo: from_raw_parts caused a crash when txs was empty. Investigate and see if this still happens
    let txs_slice = if in_txs_len != 0 && !in_txs.is_null() {
        slice::from_raw_parts(in_txs, in_txs_len as usize)
    } else {
        &[]
    };

    let validator_set_for_height = unwrap_or_return!(get_validator_set_for_height());

    let validator_set = unwrap_or_return!(Set::decode(
        validator_set_for_height.validator_set.as_slice()
    )
    .map_err(|e| {
        error!("Error parsing validator set from proto: {:?}", e);
        sgx_status_t::SGX_SUCCESS
    }));

    let commit = unwrap_or_return!(crate::verify::commit::decode(block_commit_slice));

    let header = unwrap_or_return!(crate::verify::header::validate_block_header(
        block_header_slice,
        &validator_set,
        validator_set_for_height.height,
        commit,
    ));

    let txs = unwrap_or_return!(crate::verify::txs::validate_txs(txs_slice, &header));

    let mut message_verifier = VERIFIED_MESSAGES.lock().unwrap();

    if message_verifier.remaining() != 0 {
        // this will happen if a tx fails - the message queue doesn't get cleared.
        // todo: add clearing of message queue if a tx fails?
        debug!(
                "Wasm verified out of sync?? Adding new messages but old one is not empty?? - remaining: {}",
                message_verifier.remaining()
            );

        // new tx, so messages should always be empty
        message_verifier.clear();
    }

    for tx in txs.tx.iter() {
        // doing this a different way makes the code unreadable or requires creating a copy of
        // tx. Feel free to change this if someone finds a better way
        log::trace!(
            "Got tx: {}",
            if tx.len() < TX_THRESHOLD {
                format!("{:?}", hex::encode(tx))
            } else {
                String::new()
            }
        );

        let parsed_tx = unwrap_or_return!(tx_from_bytes(tx.as_slice()).map_err(|_| {
            error!("Unable to parse tx bytes from proto");
            sgx_status_t::SGX_ERROR_INVALID_PARAMETER
        }));

        message_verifier.append_wasm_from_tx(parsed_tx);
    }

    message_verifier.set_block_info(
        header.header.height.value(),
        header.header.time.unix_timestamp_nanos(),
    );

    #[cfg(feature = "random")]
    {
        let encrypted_random_slice =
            slice::from_raw_parts(in_encrypted_random, in_encrypted_random_len as usize);

        let decrypted = unwrap_or_return!(crate::verify::random::validate_encrypted_random(
            encrypted_random_slice,
            validator_set.hash(),
            header.header.app_hash.as_bytes(),
            header.header.height.value(),
        ));

        decrypted_random.copy_from_slice(&decrypted);
    }

    debug!(
        "Done verifying block height: {:?}",
        header.header.height.value()
    );

    sgx_status_t::SGX_SUCCESS
}

#[allow(clippy::too_many_arguments)]
#[allow(unused_variables)]
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
        Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER)
    );
    validate_const_ptr!(
        in_commit,
        in_commit_len as usize,
        Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER)
    );

    #[cfg(feature = "random")]
    validate_const_ptr!(
        in_encrypted_random,
        in_encrypted_random_len as usize,
        Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER)
    );

    validate_mut_ptr!(
        decrypted_random.as_mut_ptr(),
        decrypted_random.len(),
        Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER)
    );

    if in_txs_len != 0 && !in_txs.is_null() {
        validate_const_ptr!(
            in_txs,
            in_txs_len as usize,
            Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER)
        );
    }

    Ok(())
}
