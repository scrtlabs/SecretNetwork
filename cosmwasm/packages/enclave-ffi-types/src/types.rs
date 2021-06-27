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
    /// # Safety
    /// Very unsafe. Much careful
    pub unsafe fn unsafe_clone(&self) -> Self {
        EnclaveBuffer { ptr: self.ptr }
    }
}

/// This is safe because `Vec<u8>`s are `Send`
unsafe impl Send for EnclaveBuffer {}

impl Default for EnclaveBuffer {
    fn default() -> Self {
        Self {
            ptr: core::ptr::null_mut(),
        }
    }
}

/// This type describes parameters of the runtime that the node can configure for itself.
#[repr(C)]
pub struct RuntimeConfiguration {
    /// The amount of wasmi modules cached in an LRU cache inside the enclave.
    /// This speeds up the execution of recently used modules, but has a significant
    /// memory overhead.
    pub module_cache_size: u8,
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
    #[display(fmt = "failed to execute ocall")]
    FailedOcall { vm_error: UntrustedVmError },
    #[display(fmt = "failed to validate transaction")]
    ValidationFailure,
    // Problems with the module binary
    /// The WASM code was invalid and could not be loaded.
    #[display(fmt = "tried to load invalid wasm code")]
    InvalidWasm,
    #[display(fmt = "failed to initialize wasm memory")]
    CannotInitializeWasmMemory,
    /// The WASM module contained a start section, which is not allowed.
    WasmModuleWithStart,
    /// The WASM module contained floating point operations, which is not allowed.
    #[display(fmt = "found floating point operation in module code")]
    WasmModuleWithFP,
    /// Fail to inject gas metering
    #[display(fmt = "failed to inject gas metering")]
    FailedGasMeteringInjection,
    #[display(fmt = "internal error during execution")]
    InternalError,
    // runtime issues with the module
    /// Ran out of gas
    #[display(fmt = "execution ran out of gas")]
    OutOfGas,
    /// Calling a function in the contract failed.
    #[display(fmt = "calling a function in the contract failed for an unexpected reason")]
    FailedFunctionCall,
    // These variants mimic the variants of `wasmi::TrapKind`
    /// The contract panicked during execution.
    #[display(fmt = "the contract panicked")]
    ContractPanicUnreachable,
    /// The contract tried to access memory out of bounds.
    #[display(fmt = "the contract tried to access memory out of bounds")]
    ContractPanicMemoryAccessOutOfBounds,
    /// The contract tried to access a nonexistent resource.
    #[display(fmt = "the contract tried to access a nonexistent resource")]
    ContractPanicTableAccessOutOfBounds,
    /// The contract tried to access an uninitialized resource.
    #[display(fmt = "the contract tried to access an uninitialized resource")]
    ContractPanicElemUninitialized,
    /// The contract tried to divide by zero.
    #[display(fmt = "the contract tried to divide by zero")]
    ContractPanicDivisionByZero,
    /// The contract tried to perform an invalid conversion to an integer.
    #[display(fmt = "the contract tried to perform an invalid conversion to an integer")]
    ContractPanicInvalidConversionToInt,
    /// The contract has run out of space on the stack.
    #[display(fmt = "the contract has run out of space on the stack")]
    ContractPanicStackOverflow,
    /// The contract tried to call a function but expected an incorrect function signature.
    #[display(
        fmt = "the contract tried to call a function but expected an incorrect function signature"
    )]
    ContractPanicUnexpectedSignature,

    // Errors in contract ABI:
    /// Failed to seal data
    #[display(fmt = "failed to seal data")]
    FailedSeal,
    #[display(fmt = "failed to unseal data")]
    FailedUnseal,
    #[display(fmt = "failed to authenticate secret contract")]
    FailedContractAuthentication,
    #[display(fmt = "failed to deserialize data")]
    FailedToDeserialize,
    #[display(fmt = "failed to serialize data")]
    FailedToSerialize,
    #[display(fmt = "failed to encrypt data")]
    EncryptionError,
    #[display(fmt = "failed to decrypt data")]
    DecryptionError,
    #[display(fmt = "failed to allocate memory")]
    MemoryAllocationError,
    #[display(fmt = "failed to allocate minimal safety buffer")]
    MemorySafetyAllocationError,
    #[display(fmt = "failed to read memory")]
    MemoryReadError,
    #[display(fmt = "failed to write memory")]
    MemoryWriteError,
    #[display(fmt = "function not implemented")]
    NotImplemented,
    #[display(fmt = "failed to verify transaction signature")]
    FailedTxVerification,
    #[display(fmt = "contract tried to write to storage during a query")]
    UnauthorizedWrite,

    // serious issues
    /// The host was caught trying to disrupt the enclave.
    /// This can happen if e.g. the host provides invalid pointers as responses from ocalls.
    #[display(fmt = "communication with the enclave's host failed")]
    HostMisbehavior,
    #[display(fmt = "panicked due to unexpected behavior")]
    Panic,
    #[display(fmt = "enclave ran out of heap memory")]
    OutOfMemory,
    #[display(fmt = "depth of nested contract calls exceeded")]
    ExceededRecursionLimit,
    /// Unexpected Error happened, no more details available
    #[display(fmt = "unknown error")]
    Unknown,
}

/// This type represents the possible error conditions that can be encountered in the
/// enclave while authenticating a new node in the network.
/// cbindgen:prefix-with-name
#[repr(C)]
#[derive(Debug, Display, PartialEq, Eq)]
pub enum NodeAuthResult {
    #[display(fmt = "Enclave quote is valid")]
    Success,
    #[display(fmt = "Enclave quote status was GROUP_OUT_OF_DATE which is not allowed")]
    GroupOutOfDate,
    #[display(fmt = "Enclave quote status was SIGNATURE_INVALID which is not allowed")]
    SignatureInvalid,
    #[display(fmt = "Enclave quote status was SIGNATURE_REVOKED which is not allowed")]
    SignatureRevoked,
    #[display(fmt = "Enclave quote status was GROUP_REVOKED which is not allowed")]
    GroupRevoked,
    #[display(fmt = "Enclave quote status was KEY_REVOKED which is not allowed")]
    KeyRevoked,
    #[display(fmt = "Enclave quote status was SIGRL_VERSION_MISMATCH which is not allowed")]
    SigrlVersionMismatch,
    #[display(fmt = "Enclave quote status was CONFIGURATION_NEEDED which is not allowed")]
    ConfigurationNeeded,
    #[display(
        fmt = "Enclave quote status was CONFIGURATION_AND_SW_HARDENING_NEEDED which is not allowed"
    )]
    SwHardeningAndConfigurationNeeded,
    #[display(fmt = "Enclave quote status invalid")]
    BadQuoteStatus,
    #[display(fmt = "Enclave version mismatch. Registering enclave had different code signature")]
    MrEnclaveMismatch,
    #[display(fmt = "Enclave version mismatch. Registering enclave had different signer")]
    MrSignerMismatch,
    #[display(fmt = "Enclave received invalid inputs")]
    InvalidInput,
    #[display(fmt = "The provided certificate was invalid")]
    InvalidCert,
    #[display(fmt = "Writing to file system from the enclave failed")]
    CantWriteToStorage,
    #[display(fmt = "The public key in the certificate appears to be malformed")]
    MalformedPublicKey,
    #[display(fmt = "Encrypting the seed failed")]
    SeedEncryptionFailed,
    #[display(fmt = "failed to allocate minimal safety buffer")]
    MemorySafetyAllocationError,
    #[display(
        fmt = "Unexpected panic during node authentication. Certificate may be malformed or invalid"
    )]
    Panic,
}

/// This type represents the possible error conditions that can be encountered in the
/// enclave while authenticating a new node in the network.
/// cbindgen:prefix-with-name
#[repr(C)]
#[derive(Debug, Display, PartialEq, Eq)]
pub enum HealthCheckResult {
    Success,
}

impl Default for HealthCheckResult {
    fn default() -> Self {
        HealthCheckResult::Success
    }
}

/// This type holds a pointer to a VmError that is boxed on the untrusted side
// `VmError` is the standard error type for the `cosmwasm-sgx-vm` layer.
// During an ocall, we call into the original implementation of `db_read`, `db_write`, and `db_remove`.
// These call out all the way to the Go side. They return `VmError` when something goes wrong in this process.
// These errors need to be propagated back into and out of the enclave, and then bacl into the `cosmwasm-sgx-vm` layer.
// There is never anything we can do with these errors inside the enclave, so instead of converting `VmError`
// to a type that the enclave can understand, we just box it bedore returning from the enclave, store the heap pointer
// in an instance of `UntrustedVmError`, propagate this error all the way back to the point that called
// into the enclave, and then finally unwrap the `VmError`, which gets propagated up the normal stack.
//
// For a more detailed discussion, see:
// https://github.com/enigmampc/SecretNetwork/pull/307#issuecomment-651157410
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
}

impl Default for UntrustedVmError {
    fn default() -> Self {
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
        /// The contract_key for this contract.
        contract_key: [u8; 64],
    },
    Failure {
        /// The error that happened in the enclave
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
    },
    Failure {
        /// The error that happened in the enclave
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
    },
    Failure {
        /// The error that happened in the enclave
        err: EnclaveError,
    },
}
