use cosmwasm_sgx_vm::{FfiError, FfiResult, Querier, QuerierResult};
use cosmwasm_std::{Binary, SystemError};
use std::convert::TryInto;

use crate::error::GoResult;
use crate::memory::Buffer;

// this represents something passed in from the caller side of FFI
#[repr(C)]
#[derive(Clone)]
pub struct querier_t {
    _private: [u8; 0],
}

#[repr(C)]
#[derive(Clone)]
pub struct Querier_vtable {
    // We return errors through the return buffer, but may return non-zero error codes on panic
    pub query_external: extern "C" fn(*const querier_t, *mut u64, Buffer, *mut Buffer) -> i32,
}

#[repr(C)]
#[derive(Clone)]
pub struct GoQuerier {
    pub state: *const querier_t,
    pub vtable: Querier_vtable,
}

// TODO: check if we can do this safer...
unsafe impl Send for GoQuerier {}

impl Querier for GoQuerier {
    fn raw_query(&self, request: &[u8]) -> QuerierResult {
        let request_buf = Buffer::from_vec(request.to_vec());
        let mut result_buf = Buffer::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (self.vtable.query_external)(
            self.state,
            &mut used_gas as *mut u64,
            request_buf,
            &mut result_buf as *mut Buffer,
        )
        .into();
        let _request = unsafe { request_buf.consume() };
        let go_result: FfiResult<()> = go_result.try_into().unwrap_or_else(|_| {
            Err(FfiError::other(format!(
                "Failed to query another contract with this request: {}",
                String::from_utf8_lossy(request),
            )))
        });
        go_result?;

        let bin_result = unsafe { result_buf.consume() };
        match serde_json::from_slice(&bin_result) {
            Ok(system_result) => Ok((system_result, used_gas)),
            Err(e) => Ok((
                Err(SystemError::InvalidResponse {
                    error: format!("Parsing Go response: {}", e),
                    response: Binary(bin_result),
                }),
                used_gas,
            )),
        }
    }
}
