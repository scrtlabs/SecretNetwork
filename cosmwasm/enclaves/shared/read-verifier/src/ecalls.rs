use cosmos_sdk_proto::cosmos::base::kv::v1beta1::{Pair, Pairs};
use cosmos_sdk_proto::traits::Message;
use enclave_crypto::hash::sha;
use enclave_utils::validate_const_ptr;
use integer_encoding::VarInt;
use log::{debug, error};
use sgx_types::sgx_status_t;
use std::slice;
use tendermint::merkle;

use crate::READ_PROOFER;

const MAX_VARIABLE_LENGTH: u32 = 100_000;

// TODO this is the same macro as in `block-verifier` - dedup when possible
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

/// # Safety
///  This function reads buffers which must be correctly initialized by the caller,
/// see safety section of slice::[from_raw_parts](https://doc.rust-lang.org/std/slice/fn.from_raw_parts.html#safety)
///
#[no_mangle]
#[allow(unused_variables)]
pub unsafe extern "C" fn ecall_submit_store_roots(
    in_roots: *const u8,
    in_roots_len: u32,
    in_compute_root: *const u8,
    in_compute_root_len: u32,
) -> sgx_status_t {
    validate_input_length!(in_roots_len, "roots", MAX_VARIABLE_LENGTH);
    validate_const_ptr!(
        in_roots,
        in_roots_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );
    validate_input_length!(in_compute_root_len, "roots", MAX_VARIABLE_LENGTH);
    validate_const_ptr!(
        in_compute_root,
        in_compute_root_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    let store_roots_slice = slice::from_raw_parts(in_roots, in_roots_len as usize);
    let compute_root_slice = slice::from_raw_parts(in_compute_root, in_compute_root_len as usize);

    let store_roots: Pairs = Pairs::decode(store_roots_slice).unwrap();
    let mut store_roots_bytes = vec![];

    // Make sure that the provided AppHash contains the provided compute store root
    // The AppHash merkle is built using sha256(root) of every module
    let compute_h = sha::sha_256(compute_root_slice);
    if !store_roots
        .pairs
        .as_slice()
        .iter()
        .any(|p| p.value == compute_h.to_vec())
    {
        error!("could not verify compute store root!");
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    };

    // Encode all key-value pairs to bytes
    for root in store_roots.pairs {
        // debug!("TOMMM key: {:?}", String::from_utf8_lossy(&root.key));
        // debug!("TOMMM val: {:?}", root.value);
        store_roots_bytes.push(pair_to_bytes(root));
    }
    let h = merkle::simple_hash_from_byte_vectors(store_roots_bytes);

    debug!("received app_hash: {:?}", h);
    // debug!("received compute_root: {:?}", compute_root_slice);
    // debug!(
    //     "TOMMM hashed compute_root: {:?}",
    //     sha::sha_256(compute_root_slice)
    // );

    let mut rp = READ_PROOFER.lock().unwrap();
    rp.app_hash = h;
    rp.store_merkle_root = compute_root_slice.to_vec();

    sgx_status_t::SGX_SUCCESS
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

    buf
}
