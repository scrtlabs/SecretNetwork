use std::mem;
use std::slice;

#[no_mangle]
pub extern "C" fn allocate_rust(ptr: *const u8, length: usize) -> Buffer {
    // Go doesn't store empty buffers the same way Rust stores empty slices (with NonNull  pointers
    // equal to the offset of the type, which would be equal to 1 in this case)
    // so when it wants to represent an empty buffer, it passes a null pointer with 0 length here.
    if length == 0 {
        Buffer::from_vec(Vec::new())
    } else {
        Buffer::from_vec(Vec::from(unsafe { slice::from_raw_parts(ptr, length) }))
    }
}

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

impl Buffer {
    /// `read` provides a reference to the included data to be parsed or copied elsewhere
    ///
    /// # Safety
    ///
    /// The caller must make sure that the `Buffer` points to valid and initialized memory
    pub unsafe fn read(&self) -> Option<&[u8]> {
        if self.ptr.is_null() {
            None
        } else {
            Some(slice::from_raw_parts(self.ptr, self.len))
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
        if self.ptr.is_null() {
            vec![]
        } else {
            Vec::from_raw_parts(self.ptr, self.len, self.cap)
        }
    }

    /// Creates a new zero length Buffer with the given capacity
    pub fn with_capacity(capacity: usize) -> Self {
        Buffer::from_vec(Vec::<u8>::with_capacity(capacity))
    }

    // this releases our memory to the caller
    pub fn from_vec(v: Vec<u8>) -> Self {
        let mut v = mem::ManuallyDrop::new(v);
        Buffer {
            ptr: v.as_mut_ptr(),
            len: v.len(),
            cap: v.capacity(),
        }
    }
}

impl Default for Buffer {
    fn default() -> Self {
        Buffer {
            ptr: std::ptr::null_mut::<u8>(),
            len: 0,
            cap: 0,
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;

    #[test]
    fn read_works() {
        let buffer1 = Buffer::from_vec(vec![0xAA]);
        assert_eq!(unsafe { buffer1.read() }, Some(&[0xAAu8] as &[u8]));

        let buffer2 = Buffer::from_vec(vec![0xAA, 0xBB, 0xCC]);
        assert_eq!(
            unsafe { buffer2.read() },
            Some(&[0xAAu8, 0xBBu8, 0xCCu8] as &[u8])
        );

        let empty: &[u8] = b"";

        let buffer3 = Buffer::from_vec(Vec::new());
        assert_eq!(unsafe { buffer3.read() }, Some(empty));

        let buffer4 = Buffer::with_capacity(7);
        assert_eq!(unsafe { buffer4.read() }, Some(empty));

        // Cleanup
        unsafe { buffer1.consume() };
        unsafe { buffer2.consume() };
        unsafe { buffer3.consume() };
        unsafe { buffer4.consume() };
    }

    #[test]
    fn with_capacity_works() {
        let buffer = Buffer::with_capacity(7);
        assert_eq!(buffer.ptr.is_null(), false);
        assert_eq!(buffer.len, 0);
        assert_eq!(buffer.cap, 7);

        // Cleanup
        unsafe { buffer.consume() };
    }

    #[test]
    fn from_vec_and_consume_work() {
        let mut original: Vec<u8> = vec![0x00, 0xaa, 0x76];
        original.reserve_exact(2);
        let original_ptr = original.as_ptr();

        let buffer = Buffer::from_vec(original);
        assert_eq!(buffer.ptr.is_null(), false);
        assert_eq!(buffer.len, 3);
        assert_eq!(buffer.cap, 5);

        let restored = unsafe { buffer.consume() };
        assert_eq!(restored.as_ptr(), original_ptr);
        assert_eq!(restored.len(), 3);
        assert_eq!(restored.capacity(), 5);
        assert_eq!(&restored, &[0x00, 0xaa, 0x76]);
    }

    #[test]
    fn from_vec_and_consume_work_for_zero_len() {
        let mut original: Vec<u8> = vec![];
        original.reserve_exact(2);
        let original_ptr = original.as_ptr();

        let buffer = Buffer::from_vec(original);
        assert_eq!(buffer.ptr.is_null(), false);
        assert_eq!(buffer.len, 0);
        assert_eq!(buffer.cap, 2);

        let restored = unsafe { buffer.consume() };
        assert_eq!(restored.as_ptr(), original_ptr);
        assert_eq!(restored.len(), 0);
        assert_eq!(restored.capacity(), 2);
    }

    #[test]
    fn from_vec_and_consume_work_for_zero_capacity() {
        let original: Vec<u8> = vec![];
        let original_ptr = original.as_ptr();

        let buffer = Buffer::from_vec(original);
        // Skip ptr test here. Since Vec does not allocate memory when capacity is 0, this could be anything
        assert_eq!(buffer.len, 0);
        assert_eq!(buffer.cap, 0);

        let restored = unsafe { buffer.consume() };
        assert_eq!(restored.as_ptr(), original_ptr);
        assert_eq!(restored.len(), 0);
        assert_eq!(restored.capacity(), 0);
    }
}
