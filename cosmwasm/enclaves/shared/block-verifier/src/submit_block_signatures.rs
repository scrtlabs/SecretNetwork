use core::convert::TryInto;
use std::slice;

use sgx_types::sgx_status_t;

use enclave_utils::{validate_const_ptr, validate_input_length, validate_mut_ptr, KEY_MANAGER};
use log::debug;
use log::error;
use std::convert::TryFrom;
use tendermint::signature::Ed25519Signature;
use tendermint::Hash;
use tendermint::Signature;

macro_rules! unwrap_or_return {
    ($result:expr) => {
        match $result {
            Ok(commit) => commit,
            Err(e) => return e.into(),
        }
    };
}

use crate::txs::tx_from_bytes;
use crate::wasm_messages::VERIFIED_BLOCK_MESSAGES;

use sha2::{Digest, Sha256};

const MAX_VARIABLE_LENGTH: u32 = 100_000;
const MAX_BLOCK_DATA_LENGTH: u32 = 22_020_096; // 21 MiB = max block size
const RANDOM_PROOF_LEN: u32 = 80;

#[no_mangle]
#[allow(unused_variables)]
#[allow(clippy::too_many_arguments)]
#[allow(clippy::missing_safety_doc)]
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
    next_validator_set_evidence: &mut [u8; 32],
    in_cron_msgs: *const u8,
    in_cron_msgs_len: u32,
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

    let cron_msgs_slice = if in_cron_msgs_len != 0 && !in_cron_msgs.is_null() {
        slice::from_raw_parts(in_cron_msgs, in_cron_msgs_len as usize)
    } else {
        &[]
    };

    let (validator_set, height) = {
        let extra = KEY_MANAGER.extra_data.lock().unwrap();
        let validator_set = match extra.decode_validator_set() {
            Some(set) => set,
            None => {
                error!("Error parsing validator set from proto");
                return sgx_status_t::SGX_SUCCESS;
            }
        };

        (validator_set, extra.height)
    };

    let commit = unwrap_or_return!(crate::verify::commit::decode(block_commit_slice));

    let header = unwrap_or_return!(crate::verify::header::validate_block_header(
        block_header_slice,
        &validator_set,
        height,
        commit,
    ));

    let txs = unwrap_or_return!(crate::verify::txs::validate_txs(txs_slice, &header));

    let cron_msgs = if cron_msgs_slice.len() > 0 {
        let msgs = crate::txs::txs_from_bytes(cron_msgs_slice).map_err(|e| {
            error!("Error parsing cron msgs from proto: {:?}", e);
            sgx_status_t::SGX_ERROR_INVALID_PARAMETER
        });
        if msgs.is_err() {
            error!("Error parsing cron msgs from proto: {:?}", msgs);
            return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
        }
        Some(msgs.unwrap())
    } else {
        None
    };

    let mut message_verifier = VERIFIED_BLOCK_MESSAGES.lock().unwrap();

    if message_verifier.remaining() != 0 {
        // new block, clear messages
        message_verifier.clear();
    }

    let mut hasher = Sha256::new();
    hasher.update(cron_msgs_slice);
    let hash_result = hasher.finalize();
    let hash_result: [u8; 32] = hash_result.into();

    let implicit_hash = tendermint::Hash::Sha256(hash_result);

    if implicit_hash != header.header.implicit_hash {
        error!("Implicit hash does not match header implicit hash");
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    if let Some(cron_msgs) = cron_msgs {
        for msg in cron_msgs {
            let parsed_cron_msg = unwrap_or_return!(tx_from_bytes(msg.as_slice()).map_err(|_| {
                error!("Unable to parse tx bytes from proto");
                sgx_status_t::SGX_ERROR_INVALID_PARAMETER
            }));

            message_verifier.append_msg_from_tx(parsed_cron_msg);
        }
    }

    for tx in txs.iter() {
        // doing this a different way makes the code unreadable or requires creating a copy of

        let parsed_tx = unwrap_or_return!(tx_from_bytes(tx.as_slice()).map_err(|_| {
            error!("Unable to parse tx bytes from proto");
            sgx_status_t::SGX_ERROR_INVALID_PARAMETER
        }));

        message_verifier.append_msg_from_tx(parsed_tx);
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

    // store this in the storage: header.header.next_validators_hash
    if let tendermint::Hash::Sha256(val) = header.header.next_validators_hash {
        let validator_set_evidence = KEY_MANAGER.encrypt_hash(val, height + 1);
        next_validator_set_evidence.copy_from_slice(validator_set_evidence.as_slice());
        message_verifier
            .next_validators_evidence
            .copy_from_slice(validator_set_evidence.as_slice());
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
    let failed_call = || Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER);

    validate_input_length!(in_header_len, "header", MAX_VARIABLE_LENGTH, failed_call());
    validate_input_length!(in_commit_len, "commit", MAX_VARIABLE_LENGTH, failed_call());
    validate_input_length!(in_txs_len, "txs", MAX_BLOCK_DATA_LENGTH, failed_call());
    validate_input_length!(
        in_encrypted_random_len,
        "encrypted_random",
        RANDOM_PROOF_LEN,
        failed_call()
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
