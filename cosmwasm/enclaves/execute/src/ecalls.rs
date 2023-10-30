use cosmos_sdk_proto::cosmos::base::kv::v1beta1::{Pair, Pairs};
use cosmos_sdk_proto::traits::Message;
use integer_encoding::VarInt;
use sgx_types::sgx_status_t;
use tendermint::merkle;

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
    #[cfg(feature = "light-client-validation")]
    {
        block_verifier::submit_block_signatures::submit_block_signatures_impl(
            in_header,
            in_header_len,
            in_commit,
            in_commit_len,
            in_txs,
            in_txs_len,
            in_encrypted_random,
            in_encrypted_random_len,
            decrypted_random,
        )
    }

    #[cfg(all(not(feature = "light-client-validation"), not(feature = "SGX_MODE_HW")))]
    {
        // not returning an error here, but instead going for a noop so tests can run without errors
        sgx_status_t::SGX_SUCCESS
    }

    #[cfg(all(not(feature = "light-client-validation"), feature = "SGX_MODE_HW"))]
    {
        // this is an error so that if we're compiling in HW mode we don't forget to enable this feature
        // if this function is being called (integration tests)
        sgx_status_t::SGX_ERROR_ECALL_NOT_ALLOWED
    }
}

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

    // Encode all key-value pairs to bytes
    for root in store_roots.pairs {
        store_roots_bytes.push(pair_to_bytes(root));
    }

    let h = merkle::simple_hash_from_byte_vectors(store_roots_bytes);
    debug!("received app_hash: {:?}", h);
    debug!("received compute_root: {:?}", compute_root_slice);

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
