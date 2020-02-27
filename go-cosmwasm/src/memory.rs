use std::mem;
use std::slice;

// this frees memory we released earlier
#[no_mangle]
pub extern "C" fn free_rust(buf: Buffer) {
    unsafe {
        let _ = buf.consume();
    }
}

#[derive(Copy, Clone)]
#[repr(C)]
pub struct Buffer {
    pub ptr: *mut u8,
    pub len: usize,
    pub cap: usize,
}

impl Default for Buffer {
    fn default() -> Self {
        Buffer {
            ptr: std::ptr::null_mut(),
            len: 0,
            cap: 0,
        }
    }
}

impl Buffer {
    // read provides a reference to the included data to be parsed or copied elsewhere
    // data is only guaranteed to live as long as the Buffer
    // (or the scope of the extern "C" call it came from)
    pub fn read(&self) -> Option<&[u8]> {
        if self.is_empty() {
            None
        } else {
            unsafe { Some(slice::from_raw_parts(self.ptr, self.len)) }
        }
    }

    /// consume must only be used on memory previously released by from_vec
    /// when the Vec is out of scope, it will deallocate the memory previously referenced by Buffer
    ///
    /// # Safety
    ///
    /// if not empty, `ptr` must be a valid memory reference, which was previously
    /// created by `from_vec`. You may not consume a slice twice.
    /// Otherwise you risk double free panics
    pub unsafe fn consume(self) -> Vec<u8> {
        if self.is_empty() {
            return Vec::new();
        }
        let mut v = Vec::from_raw_parts(self.ptr, self.len, self.cap);
        v.shrink_to_fit();
        v
    }

    // this releases our memory to the caller
    pub fn from_vec(mut v: Vec<u8>) -> Self {
        let buf = Buffer {
            ptr: v.as_mut_ptr(),
            len: v.len(),
            cap: v.capacity(),
        };
        mem::forget(v);
        buf
    }

    pub fn is_empty(&self) -> bool {
        self.ptr.is_null() || self.len == 0 || self.cap == 0
    }
}
