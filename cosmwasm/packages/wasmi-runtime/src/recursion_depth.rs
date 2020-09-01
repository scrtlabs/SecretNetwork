use std::sync::SgxMutex;

use lazy_static::lazy_static;

use enclave_ffi_types::EnclaveError;

const RECURSION_LIMIT: i32 = 5;

lazy_static! {
    /// This counter tracks the recursion depth of queries,
    /// and effectively the amount of loaded instances of WASMI.
    ///
    /// It is incremented before each computation begins and is decremented after each computation ends.
    static ref RECURSION_DEPTH: SgxMutex<i32> = SgxMutex::new(0);
}

pub fn increment() -> Result<(), EnclaveError> {
    let mut depth = RECURSION_DEPTH.lock().unwrap();
    if *depth == RECURSION_LIMIT {
        return Err(EnclaveError::ExceededRecursionLimit);
    }
    *depth += 1;
    Ok(())
}

pub fn decrement() {
    let mut depth = RECURSION_DEPTH.lock().unwrap();
    *depth -= 1;
}

/// Returns whether or not this is the last possible level of recursion
pub fn limit_reached() -> bool {
    *RECURSION_DEPTH.lock().unwrap() == RECURSION_LIMIT
}
