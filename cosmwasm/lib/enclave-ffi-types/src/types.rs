use core::ffi::c_void;

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

/// This struct holds a pointer to memory in userspace, that contains the storage
#[repr(C)]
#[derive(Clone, Copy)]
pub struct Ctx {
    pub data: *mut c_void,
}

/// This type represents the possible error conditions that can be encountered in the enclave
/// cbindgen:prefix-with-name
#[repr(C)]
pub enum EnclaveError {
    InvalidWasm,
    WasmModuleWithStart,
    WasmModuleWithFP,
    FailedFunctionCall,
}

/// This struct is returned from ecall_init.
/// cbindgen:prefix-with-name
#[repr(C)]
pub enum InitResult {
    Success {
        /// A pointer to the output of the calculation
        output: UserSpaceBuffer,
        /// The gas used by the execution.
        used_gas: u64,
        /// A signature by the enclave on all of the results.
        signature: [u8; 65],
    },
    Failure {
        err: EnclaveError,
    },
}

/// This struct is returned from ecall_handle.
/// cbindgen:prefix-with-name
#[repr(C)]
pub enum HandleResult {
    Success {
        /// A pointer to the output of the calculation
        output: UserSpaceBuffer,
        /// The gas used by the execution.
        used_gas: u64,
        /// A signature by the enclave on all of the results.
        signature: [u8; 65],
    },
    Failure {
        err: EnclaveError,
    },
}

/// This struct is returned from ecall_query.
/// cbindgen:prefix-with-name
#[repr(C)]
pub enum QueryResult {
    Success {
        /// A pointer to the output of the calculation
        output: UserSpaceBuffer,
        /// The gas used by the execution.
        used_gas: u64,
        /// A signature by the enclave on all of the results.
        signature: [u8; 65],
    },
    Failure {
        err: EnclaveError,
    },
}
