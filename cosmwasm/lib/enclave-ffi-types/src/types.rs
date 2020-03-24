use std::ffi::c_void;

/// This type represents an opaque pointer to a memory address in normal user space.
#[repr(C)]
pub struct UserSpaceBuffer {
    pub ptr: *mut c_void,
}

/// This type represents an opaque pointer to a memory address inside the enclave.
#[repr(C)]
pub struct EnclaveBuffer {
    pub ptr: *mut c_void,
}

/// This struct is returned from ecall_init.
#[repr(C)]
pub struct InitResult {
    /// A pointer to the output of the calculation
    pub output: UserSpaceBuffer,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 65],
}

/// This struct is returned from ecall_handle.
#[repr(C)]
pub struct HandleResult {
    /// A pointer to the output of the calculation
    pub output: UserSpaceBuffer,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 65],
}

/// This struct is returned from ecall_query.
#[repr(C)]
pub struct QueryResult {
    /// A pointer to the output of the calculation
    pub output: UserSpaceBuffer,
    /// The gas used by the execution.
    pub used_gas: u64,
    /// A signature by the enclave on all of the results.
    pub signature: [u8; 65],
}
