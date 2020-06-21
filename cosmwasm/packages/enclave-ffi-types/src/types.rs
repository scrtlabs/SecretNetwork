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
    pub unsafe fn unsafe_clone(&self) -> EnclaveBuffer {
        EnclaveBuffer { ptr: self.ptr }
    }
}

impl Default for EnclaveReturn {
    fn default() -> EnclaveReturn {
        EnclaveReturn::Success
    }
}

/// This enum is used to return from an ecall/ocall to represent if the operation was a success and if not then what was the error.
/// The goal is to not reveal anything sensitive
/// `#[repr(C)]` is a Rust feature which makes the struct be aligned just like C structs.
/// See [`Repr(C)`][https://doc.rust-lang.org/nomicon/other-reprs.html]
#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum EnclaveReturn {
    /// Success, the function returned without any failure.
    Success,
    /// KeysError, There's a key missing or failed to derive a key.
    KeysError,
    /// Failure in Encryption, couldn't decrypt the variable / failed to encrypt the results.
    EncryptionError,
    /// SigningError, for some reason it failed on signing the results.
    SigningError,
    /// RecoveringError, Something failed in recovering the public key.
    RecoveringError,
    ///PermissionError, Received a permission error from an ocall, (i.e. opening the signing keys file or something like that)
    PermissionError,
    /// SgxError, Error that came from the SGX specific stuff (i.e DRAND, Sealing etc.)
    SgxError,
    /// StateError, an Error in the State. (i.e. failed applying delta, failed deserializing it etc.)
    StateError,
    /// OcallError, an error from an ocall.
    OcallError,
    /// OcallDBError, an error from the Database in the untrusted part, couldn't get/save something.
    OcallDBError,
    /// Something went really wrong.
    Other,
}

/// This struct holds a pointer to memory in userspace, that contains the storage
#[repr(C)]
pub struct Ctx {
    pub data: *mut c_void,
}

impl Ctx {
    /// # Safety
    /// Very unsafe. Much careful
    pub unsafe fn unsafe_clone(&self) -> Ctx {
        Ctx { data: self.data }
    }
}

/// This type represents the possible error conditions that can be encountered in the enclave
/// cbindgen:prefix-with-name
#[repr(C)]
#[derive(Debug, Display)]
pub enum EnclaveError {
    /// This indicated failed ocalls, but ocalls during callbacks from wasm code will not currently
    /// be represented this way. This is doable by returning a `TrapKind::Host` from these callbacks,
    /// but that's a TODO at the moment.
    FailedOcall,
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
    /// Failed to seal data
    FailedSeal,
    FailedUnseal,
    /// contract key was invalid
    FailedContractAuthentication,
    FailedToDeserialize,
    FailedToSerialize,
    EncryptionError,
    DecryptionError,
    /// Unexpected Error happened, no more details available
    Unknown,
    Panic,
}

#[repr(C)]
#[derive(Debug, Display)]
pub enum CryptoError {
    /// The `DerivingKeyError` error.
    ///
    /// This error means that the ECDH process failed.
    DerivingKeyError,
    // {
    //     self_key: [u8; 64],
    //     other_key: [u8; 64],
    // },
    /// The `MissingKeyError` error.
    ///
    /// This error means that a key was missing.
    MissingKeyError,
    //  {
    //     key_type: &'static str,
    // },
    /// The `DecryptionError` error.
    ///
    /// This error means that the symmetric decryption has failed for some reason.
    DecryptionError,
    /// The `ImproperEncryption` error.
    ///
    /// This error means that the ciphertext provided was imporper.
    /// e.g. MAC wasn't valid, missing IV etc.
    ImproperEncryption,
    /// The `EncryptionError` error.
    ///
    /// This error means that the symmetric encryption has failed for some reason.
    EncryptionError,
    /// The `SigningError` error.
    ///
    /// This error means that the signing process has failed for some reason.
    SigningError,
    // {
    //     hashed_msg: [u8; 32],
    // },
    /// The `ParsingError` error.
    ///
    /// This error means that the signature couldn't be parsed correctly.
    ParsingError,
    //  {
    //     sig: [u8; 65],
    // },
    /// The `RecoveryError` error.
    ///
    /// This error means that the public key can't be recovered from that message & signature.
    RecoveryError,
    //  {
    //     sig: [u8; 65],
    // },
    /// The `KeyError` error.
    ///
    /// This error means that a key wasn't vaild.
    /// e.g. PrivateKey, PubliKey, SharedSecret.
    // #[cfg(feature = "asymmetric")]
    KeyError,
    //  {
    //     key_type: &'static str,
    //     err: Option<secp256k1::Error>,
    // },
    // #[cfg(not(feature = "asymmetric"))]
    // KeyError { key_type: &'static str, err: Option<()> },
    // /// The `RandomError` error.
    // ///
    // /// This error means that the random function had failed generating randomness.
    // #[cfg(feature = "std")]
    // RandomError {
    //     err: rand::Error,
    // },
    // #[cfg(feature = "sgx")]
    RandomError, // {
                 //     err: sgx_types::sgx_status_t,
                 // }
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
