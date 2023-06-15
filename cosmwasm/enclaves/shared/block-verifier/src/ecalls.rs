use cosmos_sdk_proto::cosmos::base::kv::v1beta1::{Pair, Pairs};
use cosmos_sdk_proto::traits::Message;
use integer_encoding::VarInt;
use std::slice;
use tendermint::block::Commit;
use tendermint::block::Header;
use tendermint::merkle;
use tendermint_proto::Protobuf;

use sgx_types::sgx_status_t;

use crate::{txs, verify_block};
use log::{debug, error};

use enclave_utils::{validate_const_ptr, validate_mut_ptr};
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

use enclave_utils::validator_set::ValidatorSetForHeight;

const MAX_VARIABLE_LENGTH: u32 = 100_000;
const MAX_TXS_LENGTH: u32 = 10 * 1024 * 1024;

#[cfg(feature = "light-client-validation")]
const TX_THRESHOLD: usize = 100_000;

macro_rules! validate_input_length {
    ($input:expr, $var_name:expr, $constant:expr) => {
        if $input > $constant {
            error!(
                "Error: {} ({}) is larger than the constant value ({})",
                $var_name, $input, $constant
            );
            return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
        }
    };
}

#[no_mangle]
#[allow(unused_variables)]
pub unsafe extern "C" fn ecall_submit_store_roots(
    in_roots: *const u8,
    in_roots_len: u32,
) -> sgx_status_t {
    validate_input_length!(in_roots_len, "roots", MAX_VARIABLE_LENGTH);
    validate_const_ptr!(
        in_roots,
        in_roots_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    let store_roots_slice = slice::from_raw_parts(in_roots, in_roots_len as usize);

    let store_roots: Pairs = Pairs::decode(store_roots_slice).unwrap();
    let mut store_roots_bytes = vec![];

    // Encode all key-value pairs to bytes
    for root in store_roots.pairs {
        store_roots_bytes.push(pair_to_bytes(root));
    }

    let h = merkle::simple_hash_from_byte_vectors(store_roots_bytes);
    debug!("received app_hash: {:?}", h);

    return sgx_status_t::SGX_SUCCESS;
}

// This is a copy of a cosmos-sdk function: https://github.com/scrtlabs/cosmos-sdk/blob/1b9278476b3ac897d8ebb90241008476850bf212/store/internal/maps/maps.go#LL152C1-L152C1
// Returns key || value, with both the key and value length prefixed.
fn pair_to_bytes(kv: Pair) -> Vec<u8> {
    // In the worst case:
    // * 8 bytes to Uvarint encode the length of the key
    // * 8 bytes to Uvarint encode the length of the value
    // So preallocate for the worst case, which will in total
    // be a maximum of 14 bytes wasted, if len(key)=1, len(value)=1,
    // but that's going to rare.
    let mut buf = vec![];

    // Encode the key, prefixed with its length.
    buf.extend_from_slice(&(kv.key.len()).encode_var_vec());
    buf.extend_from_slice(&kv.key);

    // Encode the value, prefixing with its length.
    buf.extend_from_slice(&(kv.value.len()).encode_var_vec());
    buf.extend_from_slice(&kv.value);

    return buf;
}

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
        MAX_VARIABLE_LENGTH
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

    #[cfg(feature = "random")]
    let encrypted_random_slice =
        slice::from_raw_parts(in_encrypted_random, in_encrypted_random_len as usize);

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
    }

    debug!(
        "Done verifying block height: {:?}",
        validator_set_for_height.height
    );

    sgx_status_t::SGX_SUCCESS
}
