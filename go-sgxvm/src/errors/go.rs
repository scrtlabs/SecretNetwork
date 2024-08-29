/// This enum gives names to the status codes returned from Go callbacks to Rust.
/// The Go code will return one of these variants when returning.
///
/// 0 means no error, all the other cases are some sort of error.
///
/// cbindgen:prefix-with-name
// NOTE TO DEVS: If you change the values assigned to the variants of this enum, You must also
//               update the match statement in the From conversion below.
//               Otherwise all hell may break loose.
//               You have been warned.
//
#[repr(i32)] // This makes it so the enum looks like a simple i32 to Go
#[derive(PartialEq, Debug)]
pub enum GoError {
    None = 0,
    /// Go panicked for an unexpected reason.
    Panic = 1,
    /// Go received a bad argument from Rust
    BadArgument = 2,
    /// Ran out of gas while using the SDK (e.g. storage). This can come from the Cosmos SDK gas meter
    /// (https://github.com/cosmos/cosmos-sdk/blob/v0.45.4/store/types/gas.go#L29-L32).
    OutOfGas = 3,
    /// Error while trying to serialize data in Go code (typically json.Marshal)
    CannotSerialize = 4,
    /// An error happened during normal operation of a Go callback, which should be fed back to the contract
    User = 5,
    /// An error happend during interacting with DataQuerier (failed to apply some changes / failed to create contract / etc. )
    QuerierError = 6,
    /// An error type that should never be created by us. It only serves as a fallback for the i32 to GoError conversion.
    Other = -1,
}

impl From<i32> for GoError {
    fn from(n: i32) -> Self {
        // This conversion treats any number that is not otherwise an expected value as `GoError::Other`
        match n {
            0 => GoError::None,
            1 => GoError::Panic,
            2 => GoError::BadArgument,
            3 => GoError::OutOfGas,
            4 => GoError::CannotSerialize,
            5 => GoError::User,
            6 => GoError::QuerierError,
            _ => GoError::Other,
        }
    }
}
