#![allow(unused)]

use derive_more::Display;
use enclave_ffi_types::EnclaveError;
use wasmi::{Error as InterpreterError, HostError};

#[derive(Debug, Display)]
#[non_exhaustive]
pub enum WasmEngineError {
    FailedOcall,
    OutOfGas,
    EncryptionError,
    DecryptionError,
    DbError(DbError),
    MemoryAllocationError,
    MemoryReadError,
    MemoryWriteError,
    InputInvalid,
    InputEmpty,
    InputWrongPrefix,
    InputWrongLength,
    OutputWrongLength,
    NonExistentImportFunction,
}

#[derive(Debug, Display)]
#[non_exhaustive]
pub enum DbError {
    FailedRead,
    FailedRemove,
    FailedWrite,
    FailedEncryption,
    FailedDecryption,
}

impl From<DbError> for WasmEngineError {
    fn from(err: DbError) -> Self {
        WasmEngineError::DbError(err)
    }
}

impl HostError for WasmEngineError {}

