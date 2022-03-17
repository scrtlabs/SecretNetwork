use log::*;

use sgx_trts::trts::{
    rsgx_lfence, rsgx_raw_is_outside_enclave, rsgx_sfence, rsgx_slice_is_outside_enclave,
};
use sgx_types::*;

pub fn validate_mut_ptr(ptr: *mut u8, ptr_len: usize) -> SgxResult<()> {
    if rsgx_raw_is_outside_enclave(ptr, ptr_len) {
        warn!("Tried to access memory outside enclave -- rsgx_slice_is_outside_enclave");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }
    rsgx_sfence();
    Ok(())
}

pub fn validate_const_ptr(ptr: *const u8, ptr_len: usize) -> SgxResult<()> {
    if ptr.is_null() || ptr_len == 0 {
        warn!("Tried to access an empty pointer - ptr.is_null()");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }
    rsgx_lfence();
    Ok(())
}

pub fn validate_mut_slice(mut_slice: &mut [u8]) -> SgxResult<()> {
    if rsgx_slice_is_outside_enclave(mut_slice) {
        warn!("Tried to access memory outside enclave -- rsgx_slice_is_outside_enclave");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }
    rsgx_sfence();
    Ok(())
}
