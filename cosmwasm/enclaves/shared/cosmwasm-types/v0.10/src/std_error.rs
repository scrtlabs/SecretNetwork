//! must keep this file in sync with cosmwasm/packages/std/src/errors/std_error.rs

use serde::{Deserialize, Serialize};

/// Structured error type for init, handle and query.
///
/// This can be serialized and passed over the Wasm/VM boundary, which allows us to use structured
/// error types in e.g. integration tests. In that process backtraces are stripped off.
///
/// The prefix "Std" means "the standard error within the standard library". This is not the only
/// result/error type in cosmwasm-std.
///
/// When new cases are added, they should describe the problem rather than what was attempted (e.g.
/// InvalidBase64 is preferred over Base64DecodingErr). In the long run this allows us to get rid of
/// the duplication in "StdError::FooErr".
///
/// Checklist for adding a new error:
/// - Add enum case
/// - Add to PartialEq implementation
/// - Add serialize/deserialize test
/// - Add creator function in std_error_helpers.rs
/// - Regenerate schemas
#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
#[non_exhaustive]
pub enum StdError {
    /// Whenever there is no specific error type available
    GenericErr {
        msg: String,
    },
    InvalidBase64 {
        msg: String,
    },
    /// Whenever UTF-8 bytes cannot be decoded into a unicode string, e.g. in String::from_utf8 or str::from_utf8.
    InvalidUtf8 {
        msg: String,
    },
    NotFound {
        kind: String,
    },
    NullPointer {},
    ParseErr {
        /// the target type that was attempted
        target: String,
        msg: String,
    },
    SerializeErr {
        /// the source type that was attempted
        source: String,
        msg: String,
    },
    Unauthorized {},
    Underflow {
        minuend: String,
        subtrahend: String,
    },
}

/// The return type for init, handle and query. Since the error type cannot be serialized to JSON,
/// this is only available within the contract and its unit tests.
///
/// The prefix "Std" means "the standard result within the standard library". This is not the only
/// result/error type in cosmwasm-std.
pub type StdResult<T> = core::result::Result<T, StdError>;
