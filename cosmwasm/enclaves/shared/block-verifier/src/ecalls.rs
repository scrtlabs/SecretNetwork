use std::slice;

#[cfg(feature = "light-client-validation")]
use tendermint_proto::Protobuf;

use sgx_types::sgx_status_t;

use enclave_utils::{validate_const_ptr, validate_input_length, validate_mut_ptr};
use log::error;

#[cfg(feature = "light-client-validation")]
use log::debug;

#[cfg(feature = "light-client-validation")]
use tendermint::validator::Set;

#[cfg(feature = "light-client-validation")]
macro_rules! unwrap_or_error {
    ($result:expr) => {
        match $result {
            Ok(commit) => commit,
            Err(e) => return e.into(),
        }
    };
}

#[cfg(feature = "light-client-validation")]
use crate::txs::tx_from_bytes;
#[cfg(feature = "light-client-validation")]
use crate::wasm_messages::VERIFIED_MESSAGES;

#[cfg(feature = "light-client-validation")]
use crate::verify::validator_set::get_validator_set_for_height;
#[cfg(feature = "light-client-validation")]
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

    let txs_slice = if in_txs_len != 0 && !in_txs.is_null() {
        slice::from_raw_parts(in_txs, in_txs_len as usize)
    } else {
        &[]
    };

    #[cfg(feature = "light-client-validation")]
    {
        let validator_set_for_height: ValidatorSetForHeight =
            unwrap_or_error!(get_validator_set_for_height());

        let validator_set = unwrap_or_error!(Set::decode(
            validator_set_for_height.validator_set.as_slice()
        )
        .map_err(|e| {
            error!("Error parsing validator set from proto: {:?}", e);
            sgx_status_t::SGX_SUCCESS
        }));

        let commit = unwrap_or_error!(crate::verify::commit::decode(block_commit_slice));

        let header = unwrap_or_error!(crate::verify::header::validate_block_header(
            block_header_slice,
            &validator_set,
            validator_set_for_height.height,
            commit,
        ));

        let txs = unwrap_or_error!(crate::verify::txs::validate_txs(txs_slice, &header));

        let mut message_verifier = VERIFIED_MESSAGES.lock().unwrap();
        //debug to make sure it doesn't go out of sync
        if message_verifier.remaining() != 0 {
            error!(
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

            let parsed_tx = unwrap_or_error!(tx_from_bytes(tx.as_slice()).map_err(|_| {
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

            let decrypted = unwrap_or_error!(crate::verify::random::validate_encrypted_random(
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
    }

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
