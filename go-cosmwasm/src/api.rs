use snafu::ResultExt;

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
    pub c_human_address: extern "C" fn(*mut api_t, Buffer, Buffer) -> i32,
    pub c_canonical_address: extern "C" fn(*mut api_t, Buffer, Buffer) -> i32,
}

#[repr(C)]
#[derive(Copy, Clone)]
pub struct GoApi {
    pub state: *mut api_t,
    pub vtable: GoApi_vtable,
}

const MAX_ADDRESS_BYTES: usize = 100;

impl Api for GoApi {
    fn canonical_address(&self, human: &HumanAddr) -> Result<CanonicalAddr> {
        let human = human.as_str().as_bytes();
        let input = Buffer::from_vec(human.to_vec());
        let mut output = Buffer::from_vec(vec![0u8; MAX_ADDRESS_BYTES]);
        let read = (self.vtable.c_canonical_address)(self.state, input, output);
        if read < 0 {
            return ContractErr {
                msg: "human_address returned error",
            }
            .fail();
        }
        output.len = read as usize;
        let canon = unsafe { output.consume() };
        Ok(CanonicalAddr(canon))
    }

    fn human_address(&self, canonical: &CanonicalAddr) -> Result<HumanAddr> {
        let canonical = canonical.as_bytes();
        let input = Buffer::from_vec(canonical.to_vec());
        let mut output = Buffer::from_vec(vec![0u8; MAX_ADDRESS_BYTES]);
        let read = (self.vtable.c_human_address)(self.state, input, output);
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
