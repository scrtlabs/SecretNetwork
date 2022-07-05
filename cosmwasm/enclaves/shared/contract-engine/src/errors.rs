use derive_more::Display;
use log::*;

use wasmi::{Error as InterpreterError, HostError, TrapKind};

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

pub fn wasm3_error_to_enclave_error(
    context: &mut crate::wasm3::Context,
    wasm3_error: wasm3::error::Error,
) -> EnclaveError {
    use wasm3::error::Error as Wasm3RsError;
    use wasm3::error::Trap;

    match context.take_last_error() {
        Some(wasm_engine_error) => wasm_engine_error.into(),
        None => match wasm3_error {
            Wasm3RsError::Wasm3(wasm3_error) => match Trap::from(wasm3_error) {
                Trap::OutOfBoundsMemoryAccess => EnclaveError::ContractPanicMemoryAccessOutOfBounds,
                Trap::DivisionByZero => EnclaveError::ContractPanicDivisionByZero,
                Trap::IntegerOverflow => EnclaveError::Panic,
                Trap::IntegerConversion => EnclaveError::ContractPanicInvalidConversionToInt,
                Trap::IndirectCallTypeMismatch => EnclaveError::FailedFunctionCall,
                Trap::TableIndexOutOfRange => EnclaveError::ContractPanicTableAccessOutOfBounds,
                Trap::Exit => EnclaveError::Panic,
                Trap::Abort => EnclaveError::Panic,
                Trap::Unreachable => EnclaveError::ContractPanicUnreachable,
                Trap::StackOverflow => EnclaveError::ContractPanicStackOverflow,
            },
            Wasm3RsError::InvalidFunctionSignature => EnclaveError::FailedFunctionCall,
            Wasm3RsError::FunctionNotFound => EnclaveError::FailedFunctionCall,
            Wasm3RsError::ModuleNotFound => EnclaveError::FailedFunctionCall,
            Wasm3RsError::ModuleLoadEnvMismatch => EnclaveError::FailedFunctionCall,
            Wasm3RsError::RuntimeIsActive => EnclaveError::FailedFunctionCall,
        },
    }
}

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
