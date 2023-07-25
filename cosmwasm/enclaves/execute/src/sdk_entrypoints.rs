use core::slice;
pub use enclave_contract_engine::clear_block_cache;
use enclave_ffi_types::SdkBeginBlockerResult;
use enclave_utils::validate_const_ptr;
use read_verifier::ecalls::submit_store_roots;

use log::error;

use sgx_types::sgx_status_t;

const MAX_VARIABLE_LENGTH: u32 = 100_000;

// TODO this is the same macro as in `block-verifier` - dedup when possible
macro_rules! validate_input_length {
    ($input:expr, $var_name:expr, $constant:expr) => {
        if $input > $constant {
            error!(
                "Error: {} ({}) is larger than the constant value ({})",
                $var_name, $input, $constant
            );
            return SdkBeginBlockerResult::BadVariableLength;
        }
    };
}

#[no_mangle]
pub unsafe extern "C" fn ecall_app_begin_blocker(
    in_roots: *const u8,
    in_roots_len: u32,
    in_compute_root: *const u8,
    in_compute_root_len: u32,
) -> SdkBeginBlockerResult {
    clear_block_cache();

    validate_input_length!(in_roots_len, "roots", MAX_VARIABLE_LENGTH);
    validate_const_ptr!(
        in_roots,
        in_roots_len as usize,
        SdkBeginBlockerResult::BadVariableLength
    );
    validate_input_length!(in_compute_root_len, "compute_roots", MAX_VARIABLE_LENGTH);
    validate_const_ptr!(
        in_compute_root,
        in_compute_root_len as usize,
        SdkBeginBlockerResult::BadVariableLength
    );

    let store_roots_slice = slice::from_raw_parts(in_roots, in_roots_len as usize);
    let compute_root_slice = slice::from_raw_parts(in_compute_root, in_compute_root_len as usize);

    match submit_store_roots(store_roots_slice, compute_root_slice) {
        sgx_status_t::SGX_SUCCESS => SdkBeginBlockerResult::Success,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER => SdkBeginBlockerResult::BadVariable,
        _ => SdkBeginBlockerResult::Failure,
    }
}

#[no_mangle]
pub unsafe extern "C" fn end_blocker() {}
