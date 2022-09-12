//! must keep this file in sync with cosmwasm/packages/std/src/errors/system_error.rs

use serde::{Deserialize, Serialize};

use super::encoding::Binary;
use super::types::HumanAddr;

/// SystemError is used for errors inside the VM and is API friendly (i.e. serializable).
///
/// This is used on return values for Querier as a nested result: Result<StdResult<T>, SystemError>
/// The first wrap (SystemError) will trigger if the contract address doesn't exist,
/// the QueryRequest is malformated, etc. The second wrap will be an error message from
/// the contract itself.
///
/// Such errors are only created by the VM. The error type is defined in the standard library, to ensure
/// the contract understands the error format without creating a dependency on cosmwasm-vm.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
#[non_exhaustive]
pub enum SystemError {
    InvalidRequest { error: String, request: Binary },
    InvalidResponse { error: String, response: Binary },
    NoSuchContract { addr: HumanAddr },
    Unknown {},
    UnsupportedRequest { kind: String },
    ExceededRecursionLimit {},
}

pub type SystemResult<T> = Result<T, SystemError>;
