use std::vec::Vec;
use crate::memory::{UnmanagedVector, U8SliceView};
use crate::types::AllocationWithResult;
use crate::ocall;
use sgx_types::sgx_status_t;

#[repr(C)]
#[derive(Clone)]
pub struct GoQuerier {
    pub state: *const querier_t,
    pub vtable: Querier_vtable,
}

#[repr(C)]
#[derive(Clone)]
pub struct querier_t {
    _private: [u8; 0],
}

#[repr(C)]
#[derive(Clone)]
pub struct Querier_vtable {
    // We return errors through the return buffer, but may return non-zero error codes on panic
    pub query_external: extern "C" fn(
        *const querier_t,
        U8SliceView,
        *mut UnmanagedVector, // result output
        *mut UnmanagedVector, // error message output
    ) -> i32,
}

pub fn make_request(querier: *mut GoQuerier, request: Vec<u8>) -> Option<Vec<u8>> {
    let mut allocation = std::mem::MaybeUninit::<AllocationWithResult>::uninit();

    let result = unsafe {
        ocall::ocall_query_raw(
            allocation.as_mut_ptr(),
            querier,
            request.as_ptr(),
            request.len(),
        )
    };

    if result != sgx_status_t::SGX_SUCCESS {
        println!("Cannot call make_request: Reason: {:?}", result.as_str());
        return None;
    }

    let allocation = unsafe { allocation.assume_init() };
    if allocation.status != sgx_status_t::SGX_SUCCESS {
        println!("Error during make_request: Reason: {:?}", allocation.status);
        return None;
    }

    let result_vec = unsafe {
        Vec::from_raw_parts(
            allocation.result_ptr,
            allocation.result_len,
            allocation.result_len,
        )
    };

    Some(result_vec)
}
