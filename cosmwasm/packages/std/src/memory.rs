use std::convert::TryFrom;
use std::mem;
use std::vec::Vec;

/// Refers to some heap allocated data in Wasm.
/// A pointer to an instance of this can be returned over FFI boundaries.
///
/// This struct is crate internal since the VM defined the same type independently.
#[repr(C)]
pub struct Region {
    pub offset: u32,
    /// The number of bytes available in this region
    pub capacity: u32,
    /// The number of bytes used in this region
    pub length: u32,
}

/// Creates a memory region of capacity `size` and length 0. Returns a pointer to the Region.
/// This is the same as the `allocate` export, but designed to be called internally.
pub fn alloc(size: usize) -> *mut Region {
    let data: Vec<u8> = Vec::with_capacity(size);
    let data_ptr = data.as_ptr() as usize;

    let region = build_region_from_components(
        u32::try_from(data_ptr).expect("pointer doesn't fit in u32"),
        u32::try_from(data.capacity()).expect("capacity doesn't fit in u32"),
        0,
    );
    mem::forget(data);
    Box::into_raw(region)
}

/// Similar to alloc, but instead of creating a new vector it consumes an existing one and returns
/// a pointer to the Region (preventing the memory from being freed until explicitly called later).
///
/// The resulting Region has capacity = length, i.e. the buffer's capacity is ignored.
pub fn release_buffer(buffer: Vec<u8>) -> *mut Region {
    let region = build_region(&buffer);
    mem::forget(buffer);
    Box::into_raw(region)
}

/// Return the data referenced by the Region and
/// deallocates the Region (and the vector when finished).
/// Warning: only use this when you are sure the caller will never use (or free) the Region later
///
/// # Safety
///
/// The ptr must refer to a valid Region, which was previously returned by alloc,
/// and not yet deallocated. This call will deallocate the Region and return an owner vector
/// to the caller containing the referenced data.
///
/// Naturally, calling this function twice on the same pointer will double deallocate data
/// and lead to a crash. Make sure to call it exactly once (either consuming the input in
/// the wasm code OR deallocating the buffer from the caller).
pub unsafe fn consume_region(ptr: *mut Region) -> Vec<u8> {
    assert!(!ptr.is_null(), "Region pointer is null");
    let region = Box::from_raw(ptr);

    let region_start = region.offset as *mut u8;
    // This case is explicitely disallowed by Vec
    // "The pointer will never be null, so this type is null-pointer-optimized."
    assert!(!region_start.is_null(), "Region starts at null pointer");

    Vec::from_raw_parts(
        region_start,
        region.length as usize,
        region.capacity as usize,
    )
}

/// Returns a box of a Region, which can be sent over a call to extern
/// note that this DOES NOT take ownership of the data, and we MUST NOT consume_region
/// the resulting data.
/// The Box must be dropped (with scope), but not the data
pub fn build_region(data: &[u8]) -> Box<Region> {
    let data_ptr = data.as_ptr() as usize;
    build_region_from_components(
        u32::try_from(data_ptr).expect("pointer doesn't fit in u32"),
        u32::try_from(data.len()).expect("length doesn't fit in u32"),
        u32::try_from(data.len()).expect("length doesn't fit in u32"),
    )
}

fn build_region_from_components(offset: u32, capacity: u32, length: u32) -> Box<Region> {
    Box::new(Region {
        offset: offset,
        capacity: capacity,
        length: length,
    })
}

/// Encodes multiple sections of data into one vector.
///
/// Each section is suffixed by a section length encoded as big endian uint32.
/// Using suffixes instead of prefixes allows reading sections in reverse order,
/// such that the first element does not need to be re-allocated if the contract's
/// data structure supports truncation (such as a Rust vector).
///
/// The resulting data looks like this:
///
/// ```ignore
/// section1 || section1_len || section2 || section2_len || section3 || section3_len || …
/// ```
#[allow(dead_code)] // used in Wasm and tests only
pub fn encode_sections(sections: &[&[u8]]) -> Vec<u8> {
    let mut out_len: usize = sections.iter().map(|section| section.len()).sum();
    out_len += 4 * sections.len();
    let mut out_data = Vec::with_capacity(out_len);
    for &section in sections {
        let section_len = force_to_u32(section.len()).to_be_bytes();
        out_data.extend(section);
        out_data.extend_from_slice(&section_len);
    }
    // debug_assert_eq!(out_data.len(), out_len);
    // debug_assert_eq!(out_data.capacity(), out_len);
    out_data
}

/// Converts an input of type usize to u32.
///
/// On 32 bit platforms such as wasm32 this is just a safe cast.
/// On other platforms the conversion panics for values larger than
/// `u32::MAX`.
#[inline]
pub fn force_to_u32(input: usize) -> u32 {
    #[cfg(target_pointer_width = "32")]
    {
        // usize = u32 on this architecture
        input as u32
    }
    #[cfg(not(target_pointer_width = "32"))]
    {
        use std::convert::TryInto;
        input.try_into().expect("Input exceeds u32 range")
    }
}
