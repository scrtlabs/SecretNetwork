use std::cell::Cell;

use enclave_ffi_types::EnclaveError;

const RECURSION_LIMIT: u8 = 5;

thread_local! {
    /// This counter tracks the recursion depth of queries,
    /// and effectively the amount of loaded instances of WASMI.
    ///
    /// It is incremented before each computation begins and is decremented after each computation ends.
    static RECURSION_DEPTH: Cell<u8> = Cell::new(0);
}

fn increment() -> Result<(), EnclaveError> {
    RECURSION_DEPTH.with(|depth| {
        let d = depth.get();
        if d == RECURSION_LIMIT {
            return Err(EnclaveError::ExceededRecursionLimit);
        }
        depth.set(d.saturating_add(1));
        Ok(())
    })
}

fn decrement() {
    RECURSION_DEPTH.with(|depth| {
        depth.set(depth.get().saturating_sub(1));
    })
}

/// Returns whether or not this is the last possible level of recursion
pub fn limit_reached() -> bool {
    RECURSION_DEPTH.with(|depth| depth.get()) == RECURSION_LIMIT
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
