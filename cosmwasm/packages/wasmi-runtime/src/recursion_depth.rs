use std::sync::SgxMutex;

use lazy_static::lazy_static;

use enclave_ffi_types::EnclaveError;

const RECURSION_LIMIT: u8 = 5;

lazy_static! {
    /// This counter tracks the recursion depth of queries,
    /// and effectively the amount of loaded instances of WASMI.
    ///
    /// It is incremented before each computation begins and is decremented after each computation ends.
    static ref RECURSION_DEPTH: SgxMutex<u8> = SgxMutex::new(0);
}

fn increment() -> Result<(), EnclaveError> {
    let mut depth = RECURSION_DEPTH.lock().unwrap();
    if *depth == RECURSION_LIMIT {
        return Err(EnclaveError::ExceededRecursionLimit);
    }
    *depth = depth.saturating_add(1);
    Ok(())
}

fn decrement() {
    let mut depth = RECURSION_DEPTH.lock().unwrap();
    *depth = depth.saturating_sub(1);
}

/// Returns whether or not this is the last possible level of recursion
pub fn limit_reached() -> bool {
    *RECURSION_DEPTH.lock().unwrap() == RECURSION_LIMIT
}

pub struct RecursionGuard {
    _private: (), // prevent direct instantiation outside this module
}

impl RecursionGuard {
    pub fn new() -> Result<Self, EnclaveError> {
        increment()?;
        Ok(Self { _private: () })
    }
}

impl Drop for RecursionGuard {
    fn drop(&mut self) {
        decrement();
    }
}

pub fn guard() -> Result<RecursionGuard, EnclaveError> {
    RecursionGuard::new()
}
