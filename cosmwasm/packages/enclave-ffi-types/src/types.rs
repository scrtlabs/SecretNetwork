#![allow(unused)]

use core::ffi::c_void;
use derive_more::Display;

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

impl EnclaveBuffer {
    pub fn null() -> Self {
        Self {
            ptr: core::ptr::null_mut(),
        }
    }

    /// # Safety
    /// Very unsafe. Much careful
    pub unsafe fn unsafe_clone(&self) -> Self {
        EnclaveBuffer { ptr: self.ptr }
    }
}

/// This struct holds a pointer to memory in userspace, that contains the storage
#[repr(C)]
pub struct Ctx {
    pub data: *mut c_void,
}

impl Ctx {
    /// # Safety
    /// Very unsafe. Much careful
    pub unsafe fn unsafe_clone(&self) -> Self {
        Self { data: self.data }
    }
}

/// This type represents the possible error conditions that can be encountered in the enclave
/// cbindgen:prefix-with-name
#[repr(C)]
#[derive(Debug, Display)]
pub enum EnclaveError {
    /// An ocall failed to execute. This can happen because of three scenarios:
    /// 1. A VmError was thrown during the execution of the ocall. In this case, `vm_error` will be non-null.
    /// 2. An error happened that prevented the ocall from running correctly. This can happen because of
    ///    caught memory-handling issues, or a failed ecall during an ocall. `vm_error` will be null.
    /// 3. We failed to call the ocall due to an SGX fault. `vm_error` will be null.
    // TODO should we split these three cases for better diagnostics?
    #[display(fmt = "FailedOcall")]
    FailedOcall {
        vm_error: UntrustedVmError,
    },
    /// The WASM code was invalid and could not be loaded.
    InvalidWasm,
    /// The WASM module contained a start section, which is not allowed.
    WasmModuleWithStart,
    /// The WASM module contained floating point operations, which is not allowed.
    WasmModuleWithFP,
    /// Calling a function in the contract failed.
    FailedFunctionCall,
    /// Fail to inject gas metering
    FailedGasMeteringInjection,
    /// Ran out of gas
    OutOfGas,
    // Errors in contract ABI:
    /// Failed to seal data
    FailedSeal,
    FailedUnseal,
    /// contract key was invalid
    FailedContractAuthentication,
    FailedToDeserialize,
    FailedToSerialize,
    EncryptionError,
    DecryptionError,
    Panic,
    /// Unexpected Error happened, no more details available
    Unknown,
}

/// This type holds a pointer to a VmError that is boxed on the untrusted side
#[repr(C)]
#[derive(Debug, Display)]
#[display(fmt = "VmError")]
pub struct UntrustedVmError {
    pub ptr: *mut c_void,
}

impl UntrustedVmError {
    pub fn new(ptr: *mut c_void) -> Self {
        Self { ptr }
    }
    pub fn null() -> Self {
        Self {
            ptr: core::ptr::null_mut(),
        }
    }
}

// These implementations are safe because we know that it will only ever be a Box<VmError>,
// which also has these traits.
unsafe impl Send for UntrustedVmError {}
unsafe impl Sync for UntrustedVmError {}

/// This type represent return statuses from ocalls.
///
/// cbindgen:prefix-with-name
#[repr(C)]
#[derive(Debug, Display)]
pub enum OcallReturn {
    /// Ocall returned successfully.
    Success,
    /// Ocall failed for some reason.
    /// error parameters may be passed as out parameters.
    Failure,
    /// A panic happened during the ocall.
    Panic,
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
        signature: [u8; 64],
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
        signature: [u8; 64],
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
        signature: [u8; 64],
    },
    Failure {
        err: EnclaveError,
    },
}
