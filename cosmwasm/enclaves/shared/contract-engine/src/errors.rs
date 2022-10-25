use derive_more::Display;
use log::*;

#[cfg(feature = "wasmi-engine")]
use wasmi::{Error as InterpreterError, HostError, TrapKind};

use wasm3::Error as Wasm3RsError;

use enclave_ffi_types::{EnclaveError, UntrustedVmError};

use crate::external::ecalls::BufferRecoveryError;

#[derive(Debug, Display)]
#[non_exhaustive]
pub enum WasmEngineError {
    #[display(fmt = "FailedOcall")]
    FailedOcall(UntrustedVmError),
    /// The untrusted host seems to be misbehaving
    HostMisbehavior,
    OutOfGas,
    Panic,

    EncryptionError,
    DecryptionError,
    SerializationError,
    DeserializationError,
    // This is for unexpected error while processing base32 data.
    Base32Error,

    MemoryAllocationError,
    MemoryReadError,
    MemoryWriteError,
    /// The contract attempted to write to storage during a query
    UnauthorizedWrite,

    /// The contract tried calling an unrecognized function
    NonExistentImportFunction,
}

pub type WasmEngineResult<T> = Result<T, WasmEngineError>;

#[cfg(feature = "wasmi-engine")]
impl HostError for WasmEngineError {}

impl From<WasmEngineError> for EnclaveError {
    fn from(engine_err: WasmEngineError) -> Self {
        use WasmEngineError::*;
        match engine_err {
            FailedOcall(vm_error) => EnclaveError::FailedOcall {
                vm_error: UntrustedVmError { ptr: vm_error.ptr },
            },
            OutOfGas => EnclaveError::OutOfGas,
            EncryptionError => EnclaveError::EncryptionError,
            DecryptionError => EnclaveError::DecryptionError,
            MemoryAllocationError => EnclaveError::MemoryAllocationError,
            MemoryReadError => EnclaveError::MemoryReadError,
            MemoryWriteError => EnclaveError::MemoryWriteError,
            UnauthorizedWrite => EnclaveError::UnauthorizedWrite,
            HostMisbehavior => EnclaveError::HostMisbehavior,
            // Unexpected WasmEngineError variant
            _other => EnclaveError::Unknown,
        }
    }
}

impl From<BufferRecoveryError> for WasmEngineError {
    fn from(_err: BufferRecoveryError) -> Self {
        WasmEngineError::HostMisbehavior
    }
}

// This is implemented just to make a `Result::map` invocation below nicer.
// All this does is unbox the `WasmEngineError` and call the `From` implementation above.
impl From<Box<WasmEngineError>> for EnclaveError {
    fn from(engine_err: Box<WasmEngineError>) -> Self {
        Self::from(*engine_err)
    }
}

/// This trait is used to convert foreign error types to EnclaveError
pub trait ToEnclaveError {
    fn to_enclave_error(self) -> EnclaveError;
}

impl ToEnclaveError for Wasm3RsError {
    fn to_enclave_error(self) -> EnclaveError {
        match self {
            Wasm3RsError::MemoryAllocationFailure => EnclaveError::MemoryAllocationError,
            Wasm3RsError::ParseError(_parse_error) => EnclaveError::InvalidWasm,
            Wasm3RsError::ModuleAlreadyLinked => EnclaveError::CannotInitializeWasmMemory,
            Wasm3RsError::ModuleTooLarge => EnclaveError::InvalidWasm,
            Wasm3RsError::MalformedModuleName => EnclaveError::InvalidWasm,
            Wasm3RsError::MalformedFunctionName => EnclaveError::InvalidWasm,
            Wasm3RsError::MalformedFunctionSignature => EnclaveError::InvalidWasm,
            Wasm3RsError::MalformedGlobalName => EnclaveError::InvalidWasm,
            Wasm3RsError::GlobalNotFound => EnclaveError::InvalidWasm,
            Wasm3RsError::FunctionNotFound => EnclaveError::FailedFunctionCall,
            Wasm3RsError::FunctionImportMissing => EnclaveError::FailedFunctionCall,
            Wasm3RsError::ArgumentCountMismatch => EnclaveError::FailedFunctionCall,
            Wasm3RsError::ArgumentTypeMismatch => EnclaveError::FailedFunctionCall,
            Wasm3RsError::MemoryInUse => EnclaveError::MemoryReadError,
            Wasm3RsError::OutOfMemory => EnclaveError::OutOfMemory,

            // Traps.
            Wasm3RsError::OutOfBoundsMemoryAccess => {
                EnclaveError::ContractPanicMemoryAccessOutOfBounds
            }
            Wasm3RsError::DivisionByZero => EnclaveError::ContractPanicDivisionByZero,
            Wasm3RsError::IntegerOverflow => EnclaveError::ContractPanicUnreachable,
            Wasm3RsError::InvalidIntegerConversion => {
                EnclaveError::ContractPanicInvalidConversionToInt
            }
            Wasm3RsError::IndirectCallTypeMismatch => {
                EnclaveError::ContractPanicUnexpectedSignature
            }
            Wasm3RsError::UndefinedTableElement => {
                EnclaveError::ContractPanicTableAccessOutOfBounds
            }
            Wasm3RsError::NullTableElement => EnclaveError::ContractPanicTableAccessOutOfBounds,
            Wasm3RsError::ExitCalled => EnclaveError::ContractPanicUnreachable,
            Wasm3RsError::AbortCalled => EnclaveError::ContractPanicUnreachable,
            Wasm3RsError::UnreachableExecuted => EnclaveError::ContractPanicUnreachable,
            Wasm3RsError::StackOverflow => EnclaveError::ContractPanicStackOverflow,

            // Other errors.
            Wasm3RsError::Unknown(_string) => EnclaveError::Unknown,
        }
    }
}

/// This trait is used to convert foreign result types to EnclaveError
pub trait ToEnclaveResult<T> {
    fn to_enclave_result(self) -> Result<T, EnclaveError>;
}

impl<T, E> ToEnclaveResult<T> for Result<T, E>
where
    E: ToEnclaveError,
{
    fn to_enclave_result(self) -> Result<T, EnclaveError> {
        self.map_err(|err| err.to_enclave_error())
    }
}

#[cfg(feature = "wasmi-engine")]
pub fn wasmi_error_to_enclave_error(wasmi_error: InterpreterError) -> EnclaveError {
    wasmi_error
        .try_into_host_error()
        .map(|host_error| {
            host_error
                .downcast::<WasmEngineError>()
                .map_or(EnclaveError::Unknown, EnclaveError::from)
        })
        .unwrap_or_else(|wasmi_error| {
            warn!("Got an error from wasmi: {:?}", wasmi_error);
            match wasmi_error {
                InterpreterError::Trap(trap) => trap_kind_to_enclave_error(trap.into_kind()),
                _ => EnclaveError::FailedFunctionCall,
            }
        })
}

#[cfg(feature = "wasmi-engine")]
fn trap_kind_to_enclave_error(kind: TrapKind) -> EnclaveError {
    match kind {
        TrapKind::Unreachable => EnclaveError::ContractPanicUnreachable,
        TrapKind::MemoryAccessOutOfBounds => EnclaveError::ContractPanicMemoryAccessOutOfBounds,
        TrapKind::TableAccessOutOfBounds => EnclaveError::ContractPanicTableAccessOutOfBounds,
        TrapKind::ElemUninitialized => EnclaveError::ContractPanicElemUninitialized,
        TrapKind::DivisionByZero => EnclaveError::ContractPanicDivisionByZero,
        TrapKind::InvalidConversionToInt => EnclaveError::ContractPanicInvalidConversionToInt,
        TrapKind::StackOverflow => EnclaveError::ContractPanicStackOverflow,
        TrapKind::UnexpectedSignature => EnclaveError::ContractPanicUnexpectedSignature,
        // This is for cases that we don't care to represent, or were added in later versions of wasmi.
        // Specifically `TrapKind::Host` should be handled in `wasmi_error_to_enclave_error` by calling
        // `.as_host_error()` on the top-level error.
        _ => EnclaveError::FailedFunctionCall,
    }
}
