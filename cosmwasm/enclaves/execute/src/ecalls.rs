use sgx_types::sgx_status_t;

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

    #[cfg(not(feature = "light-client-validation"))]
    {
        sgx_status_t::SGX_ERROR_ECALL_NOT_ALLOWED
    }
}
