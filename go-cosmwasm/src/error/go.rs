use std::convert::TryFrom;
use std::fmt;

use cosmwasm_sgx_vm::FfiError;

/// This enum gives names to the status codes returned from Go callbacks to Rust.
///
/// The go code will return one of these variants when returning.
///
/// cbindgen:prefix-with-name
// NOTE TO DEVS: If you change the values assigned to the variants of this enum, You must also
//               update the match statement in the From conversion below.
//               Otherwise all hell may break loose.
//               You have been warned.
//
#[repr(i32)] // This makes it so the enum looks like a simple i32 to Go
pub enum GoResult {
    Ok = 0,
    /// Go panicked for an unexpected reason.
    Panic = 1,
    /// Go received a bad argument from Rust
    BadArgument = 2,
    /// Ran out of gas while using the SDK (e.g. storage)
    OutOfGas = 3,
    /// An error happened during normal operation of a Go callback
    Other = 4,
}

impl TryFrom<GoResult> for Result<(), FfiError> {
    type Error = &'static str;

    fn try_from(other: GoResult) -> Result<Self, Self::Error> {
        match other {
            GoResult::Ok => Ok(Ok(())),
            GoResult::Panic => Ok(Err(FfiError::foreign_panic())),
            GoResult::BadArgument => Ok(Err(FfiError::bad_argument())),
            GoResult::OutOfGas => Ok(Err(FfiError::out_of_gas())),
            GoResult::Other => Err("Unspecified error in Go code"), // no conversion possible due to missing error message
        }
    }
}

impl From<i32> for GoResult {
    fn from(n: i32) -> Self {
        use GoResult::*;
        // This conversion treats any number that is not otherwise an expected value as `GoError::Other`
        match n {
            0 => Ok,
            1 => Panic,
            2 => BadArgument,
            3 => OutOfGas,
            _ => Other,
        }
    }
}

impl fmt::Display for GoResult {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            GoResult::Ok => write!(f, "Ok"),
            GoResult::Panic => write!(f, "Panic"),
            GoResult::BadArgument => write!(f, "BadArgument"),
            GoResult::OutOfGas => write!(f, "OutOfGas"),
            GoResult::Other => write!(f, "Other Error"),
        }
    }
}
