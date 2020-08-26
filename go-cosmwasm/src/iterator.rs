use cosmwasm_sgx_vm::{FfiError, FfiResult, GasInfo, StorageIterator};
use cosmwasm_std::KV;

use crate::error::GoResult;
use crate::gas_meter::gas_meter_t;
use crate::memory::Buffer;

// Iterator maintains integer references to some tables on the Go side
#[repr(C)]
#[derive(Default, Copy, Clone)]
pub struct iterator_t {
    pub db_counter: u64,
    pub iterator_index: u64,
}

// These functions should return GoResult but because we don't trust them here, we treat the return value as i32
// and then check it when converting to GoResult manually
#[repr(C)]
#[derive(Default)]
pub struct Iterator_vtable {
    pub next_db: Option<
        extern "C" fn(
            iterator_t,
            *mut gas_meter_t,
            *mut u64,
            *mut Buffer,
            *mut Buffer,
            *mut Buffer,
        ) -> i32,
    >,
}

#[repr(C)]
pub struct GoIter {
    pub gas_meter: *mut gas_meter_t,
    pub state: iterator_t,
    pub vtable: Iterator_vtable,
}

impl GoIter {
    pub fn new(gas_meter: *mut gas_meter_t) -> Self {
        GoIter {
            gas_meter,
            state: iterator_t::default(),
            vtable: Iterator_vtable::default(),
        }
    }
}

impl StorageIterator for GoIter {
    fn next(&mut self) -> FfiResult<Option<KV>> {
        let next_db = match self.vtable.next_db {
            Some(f) => f,
            None => {
                let result = Err(FfiError::unknown("iterator vtable not set"));
                return (result, GasInfo::free());
            }
        };

        let mut key_buf = Buffer::default();
        let mut value_buf = Buffer::default();
        let mut err = Buffer::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (next_db)(
            self.state,
            self.gas_meter,
            &mut used_gas as *mut u64,
            &mut key_buf as *mut Buffer,
            &mut value_buf as *mut Buffer,
            &mut err as *mut Buffer,
        )
        .into();
        let gas_info = GasInfo::with_externally_used(used_gas);

        // return complete error message (reading from buffer for GoResult::Other)
        let default = || "Failed to fetch next item from iterator".to_string();
        unsafe {
            if let Err(err) = go_result.into_ffi_result(err, default) {
                return (Err(err), gas_info);
            }
        }

        let okey = unsafe { key_buf.read() };
        let result = match okey {
            Some(key) => {
                let value = unsafe { value_buf.read() };
                if let Some(value) = value {
                    Ok(Some((key.into(), value.into())))
                } else {
                    Err(FfiError::unknown(
                        "Failed to read value while reading the next key in the db",
                    ))
                }
            }
            None => Ok(None),
        };
        (result, gas_info)
    }
}
