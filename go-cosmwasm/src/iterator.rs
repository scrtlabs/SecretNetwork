use cosmwasm_sgx_vm::{FfiError, FfiResult, NextItem, StorageIterator};
use std::convert::TryInto;

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
        extern "C" fn(iterator_t, *mut gas_meter_t, *mut u64, *mut Buffer, *mut Buffer) -> i32,
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
    fn next(&mut self) -> FfiResult<NextItem> {
        let next_db = match self.vtable.next_db {
            Some(f) => f,
            None => return Err(FfiError::other("iterator vtable not set")),
        };

        let mut key_buf = Buffer::default();
        let mut value_buf = Buffer::default();
        let mut used_gas = 0_u64;
        let go_result: GoResult = (next_db)(
            self.state,
            self.gas_meter,
            &mut used_gas as *mut u64,
            &mut key_buf as *mut Buffer,
            &mut value_buf as *mut Buffer,
        )
        .into();
        let go_result: FfiResult<()> = go_result
            .try_into()
            .unwrap_or_else(|_| Err(FfiError::other("Failed to fetch next item from iterator")));
        go_result?;

        let okey = unsafe { key_buf.read() };
        match okey {
            Some(key) => {
                let value = unsafe { value_buf.read() };
                if let Some(value) = value {
                    let kv = (key.to_vec(), value.to_vec());
                    Ok((Some(kv), used_gas))
                } else {
                    Err(FfiError::other(
                        "Failed to read value while reading the next key in the db",
                    ))
                }
            }
            None => Ok((None, used_gas)),
        }
    }
}
