use std::slice;
use tendermint::block::Commit;
use tendermint::block::Header;
use tendermint_proto::Protobuf;

use sgx_types::sgx_status_t;

use crate::{txs, verify_block};
use log::{debug, error};

use enclave_utils::{validate_const_ptr, validate_input_length, validate_mut_ptr};
use tendermint::block::signed_header::SignedHeader;
use tendermint::validator::Set;
use tendermint::Hash::Sha256;

#[cfg(feature = "random")]
use enclave_crypto::{SIVEncryptable, KEY_MANAGER};

use crate::txs::txs_hash;

#[cfg(feature = "light-client-validation")]
use crate::txs::tx_from_bytes;
#[cfg(feature = "light-client-validation")]
use crate::wasm_messages::VERIFIED_MESSAGES;

use crate::proof::create_proof;
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

    let block_header_slice = slice::from_raw_parts(in_header, in_header_len as usize);
    let block_commit_slice = slice::from_raw_parts(in_commit, in_commit_len as usize);

    let txs_slice = if in_txs_len != 0 && !in_txs.is_null() {
        validate_const_ptr!(
            in_txs,
            in_txs_len as usize,
            sgx_status_t::SGX_ERROR_INVALID_PARAMETER
        );
        slice::from_raw_parts(in_txs, in_txs_len as usize)
    } else {
        &[]
    };

    let mut encrypted_random_slice: [u8; 48] = [0u8; 48];
    let mut rand_proof: [u8; 32] = [0u8; 32];
    #[cfg(feature = "random")]
    {
        let random_and_proof =
            slice::from_raw_parts(in_encrypted_random, in_encrypted_random_len as usize);
        encrypted_random_slice.copy_from_slice(&random_and_proof[0..48]);
        rand_proof.copy_from_slice(&random_and_proof[48..])
    }

    let validator_set_result = ValidatorSetForHeight::unseal();
    if let Err(validator_set_error) = validator_set_result {
        return validator_set_error;
    }
    let validator_set_for_height: ValidatorSetForHeight = validator_set_result.unwrap();

    let header = if let Ok(r) = Header::decode(block_header_slice) {
        r
    } else {
        error!("Error parsing header from proto");
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    };

    let commit = if let Ok(res) = Commit::decode(block_commit_slice).map_err(|e| {
        error!("Error parsing commit from proto: {:?}", e);
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    }) {
        res
    } else {
        return sgx_status_t::SGX_SUCCESS;
    };

    // validate that we have the validator set for the current height
    if header.height.value() != validator_set_for_height.height {
        error!("Validator set height does not match stored validator set");
        // we use this error code to signal that the validator set is not synced with the current block
        return sgx_status_t::SGX_ERROR_FILE_RECOVERY_NEEDED;
    }

    // validate the tx bytes with the hash in the header
    // trace!("Got tx bytes: {:?}", hex::encode(txs_slice));

    let txs = txs::txs_from_bytes(txs_slice).unwrap();

    let calculated_tx_hash = txs_hash(&txs);
    if Some(Sha256(calculated_tx_hash)) != header.data_hash {
        error!("Error verifying data hash");
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let validator_set =
        if let Ok(r) = Set::decode(validator_set_for_height.validator_set.as_slice()) {
            r
        } else {
            error!("Error parsing header from proto");
            return sgx_status_t::SGX_SUCCESS;
        };

    #[cfg(feature = "random")]
    let validator_hash = validator_set.hash();

    let signed_header = SignedHeader::new(header, commit).unwrap();
    let untrusted_block = tendermint_light_client_verifier::types::UntrustedBlockState {
        signed_header: &signed_header,
        validators: &validator_set,
        next_validators: None,
    };

    let result = verify_block(&untrusted_block);

    if !result {
        error!("Error verifying block header!");
        return sgx_status_t::SGX_ERROR_INVALID_SIGNATURE;
    }

    #[cfg(feature = "light-client-validation")]
    {
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

            let parsed_tx = tx_from_bytes(tx.as_slice()).map_err(|_| {
                error!("Unable to parse tx bytes from proto");
                sgx_status_t::SGX_ERROR_INVALID_PARAMETER
            });

            if let Ok(result) = parsed_tx {
                message_verifier.append_wasm_from_tx(result);
            } else {
                return parsed_tx.unwrap_err();
            }
        }

        message_verifier.set_block_info(
            signed_header.header.height.value(),
            signed_header.header.time.unix_timestamp_nanos(),
        );
    }

    #[cfg(feature = "random")]
    {
        let calculated_proof = create_proof(
            signed_header.header.height.value(),
            &encrypted_random_slice,
            signed_header.header.app_hash.as_bytes(),
        );
        if calculated_proof != rand_proof {
            error!("Error validating random");
            return sgx_status_t::SGX_ERROR_INVALID_SIGNATURE;
        }

        println!(
            "Encrypted random slice len: {}",
            encrypted_random_slice.len()
        );

        let decrypted = match KEY_MANAGER
            .random_encryption_key
            .unwrap()
            .decrypt_siv(&encrypted_random_slice, Some(&[validator_hash.as_bytes()]))
        {
            Ok(res) => res,
            Err(_) => {
                error!("Error decrypting random slice");
                return sgx_status_t::SGX_ERROR_INVALID_SIGNATURE;
            }
        };

        decrypted_random.copy_from_slice(&*decrypted);
    }

    debug!(
        "Done verifying block height: {:?}",
        validator_set_for_height.height
    );

    sgx_status_t::SGX_SUCCESS
}
