use snafu::ResultExt;

use cosmwasm::encoding::Binary;
use cosmwasm::errors::{ContractErr, Result, Utf8Err};
use cosmwasm::traits::Api;
use cosmwasm::types::{CanonicalAddr, HumanAddr};

use crate::memory::Buffer;

// this represents something passed in from the caller side of FFI
// in this case a struct with go function pointers
#[repr(C)]
pub struct api_t {}

#[repr(C)]
#[derive(Copy, Clone)]
pub struct GoApi_vtable {
    pub humanize_address: extern "C" fn(*const api_t, Buffer, Buffer) -> i32,
    pub canonicalize_address: extern "C" fn(*const api_t, Buffer, Buffer) -> i32,
}

#[repr(C)]
#[derive(Copy, Clone)]
pub struct GoApi {
    pub state: *const api_t,
    pub vtable: GoApi_vtable,
}

// We must declare that these are safe to Send, to use in wasm.
// The known go caller passes in immutable function pointers, but this is indeed
// unsafe for possible other callers.
//
// see: https://stackoverflow.com/questions/50258359/can-a-struct-containing-a-raw-pointer-implement-send-and-be-ffi-safe
unsafe impl Send for GoApi {}

const MAX_ADDRESS_BYTES: usize = 100;

impl Api for GoApi {
    fn canonical_address(&self, human: &HumanAddr) -> Result<CanonicalAddr> {
        let human = human.as_str().as_bytes();
        let input = Buffer::from_vec(human.to_vec());
        let mut output = Buffer::from_vec(vec![0u8; MAX_ADDRESS_BYTES]);
        let read = (self.vtable.canonicalize_address)(self.state, input, output);
        if read < 0 {
            return ContractErr {
                msg: "human_address returned error",
            }
            .fail();
        }
        output.len = read as usize;
        let canon = unsafe { output.consume() };
        Ok(CanonicalAddr(Binary(canon)))
    }

    fn human_address(&self, canonical: &CanonicalAddr) -> Result<HumanAddr> {
        let canonical = canonical.as_slice();
        let input = Buffer::from_vec(canonical.to_vec());
        let mut output = Buffer::from_vec(vec![0u8; MAX_ADDRESS_BYTES]);
        let read = (self.vtable.humanize_address)(self.state, input, output);
        if read < 0 {
            return ContractErr {
                msg: "canonical_address returned error",
            }
            .fail();
        }
        output.len = read as usize;
        let result = unsafe { output.consume() };
        // TODO: let's change the Utf8Err definition in cosmwasm to avoid a copy
        //        let human = String::from_utf8(result).context(Utf8Err{})?;
        let human = std::str::from_utf8(&result)
            .context(Utf8Err {})?
            .to_string();
        Ok(HumanAddr(human))
    }
}
