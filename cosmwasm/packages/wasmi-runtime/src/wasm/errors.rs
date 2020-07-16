use derive_more::Display;
use enclave_ffi_types::{EnclaveError, UntrustedVmError};
use log::*;
use wasmi::{Error as InterpreterError, HostError, TrapKind};

#[derive(Debug, Display)]
#[non_exhaustive]
pub enum WasmEngineError {
    #[display(fmt = "FailedOcall")]
    FailedOcall(UntrustedVmError),
    OutOfGas,
    Panic,

    EncryptionError,
    DecryptionError,

    MemoryAllocationError,
    MemoryReadError,
    MemoryWriteError,

    NonExistentImportFunction,
    NotImplemented,
}

impl WasmEngineError {
    /// This function is unsafe because you have to make sure you do not use the `WasmEngineError`
    /// instance again after calling this function.
    unsafe fn clone(&self) -> Self {
        use WasmEngineError::*;
        match self {
            FailedOcall(UntrustedVmError { ptr }) => FailedOcall(UntrustedVmError { ptr: *ptr }),
            OutOfGas => OutOfGas,
            Panic => Panic,

            EncryptionError => EncryptionError,
            DecryptionError => DecryptionError,

            MemoryAllocationError => MemoryAllocationError,
            MemoryReadError => MemoryReadError,
            MemoryWriteError => MemoryWriteError,

            NonExistentImportFunction => NonExistentImportFunction,
            NotImplemented => NotImplemented,
        }
    }
}

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
            NotImplemented => EnclaveError::NotImplemented,
            // Unexpected WasmEngineError variant
            _other => EnclaveError::Unknown,
        }
    }
}

pub fn wasmi_error_to_enclave_error(wasmi_error: InterpreterError) -> EnclaveError {
    match wasmi_error
        .as_host_error()
        .map(|err| err.downcast_ref::<WasmEngineError>())
    {
        // Safety: This code is safe because we will not use engine_err ever again.
        // It is dropped at the end of this function.
        Some(Some(engine_err)) => EnclaveError::from(unsafe { engine_err.clone() }),
        // Unexpected HostError.
        Some(None) => EnclaveError::Unknown,
        // The error is not a HostError.
        None => {
            error!("Got an error from wasmi: {:?}", wasmi_error);
            match wasmi_error {
                InterpreterError::Trap(trap) => trap_kind_to_enclave_error(trap.kind()),
                _ => EnclaveError::FailedFunctionCall,
            }
        }
    }
}

fn trap_kind_to_enclave_error(kind: &TrapKind) -> EnclaveError {
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
