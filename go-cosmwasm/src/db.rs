use cosmwasm::traits::{ReadonlyStorage, Storage};

use crate::memory::Buffer;

// this represents something passed in from the caller side of FFI
#[repr(C)]
pub struct db_t {}

#[repr(C)]
pub struct DB_vtable {
    pub c_get: extern "C" fn(*mut db_t, Buffer, Buffer) -> i64,
    pub c_set: extern "C" fn(*mut db_t, Buffer, Buffer),
}

#[repr(C)]
pub struct DB {
    pub state: *mut db_t,
    pub vtable: DB_vtable,
}

impl ReadonlyStorage for DB {
    fn get(&self, key: &[u8]) -> Option<Vec<u8>> {
        let buf = Buffer::from_vec(key.to_vec());
        // TODO: dynamic size
        let mut buf2 = Buffer::from_vec(vec![0u8; 2000]);
        let res = (self.vtable.c_get)(self.state, buf, buf2);

        // read in the number of bytes returned
        if res < 0 {
            // TODO
            panic!("val was not big enough for data");
        }
        if res == 0 {
            return None;
        }
        buf2.len = res as usize;
        unsafe { Some(buf2.consume()) }
    }
}

impl Storage for DB {
    fn set(&mut self, key: &[u8], value: &[u8]) {
        let buf = Buffer::from_vec(key.to_vec());
        let buf2 = Buffer::from_vec(value.to_vec());
        // caller will free input
        (self.vtable.c_set)(self.state, buf, buf2);
    }
}
