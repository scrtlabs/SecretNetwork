use serde::{Deserialize, Serialize};
use std::fmt;

/// This is the final result type that is created and serialized in a contract for
/// every init/execute call. The VM then deserializes this type to distinguish
/// between successful and failed executions.
///
/// We use a custom type here instead of Rust's Result because we want to be able to
/// define the serialization, which is a public interface. Every language that compiles
/// to Wasm and runs in the ComsWasm VM needs to create the same JSON representation.
///
/// # Examples
///
/// Success:
///
/// ```
/// # use cosmwasm_std::{to_vec, ContractResult, Response};
/// let response: Response = Response::default();
/// let result: ContractResult<Response> = ContractResult::Ok(response);
/// assert_eq!(to_vec(&result).unwrap(), br#"{"ok":{"messages":[],"attributes":[],"events":[],"data":null}}"#.to_vec());
/// ```
///
/// Failure:
///
/// ```
/// # use cosmwasm_std::{to_vec, ContractResult, Response};
/// let error_msg = String::from("Something went wrong");
/// let result: ContractResult<Response> = ContractResult::Err(error_msg);
/// assert_eq!(to_vec(&result).unwrap(), br#"{"error":"Something went wrong"}"#.to_vec());
/// ```
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub enum ContractResult<S> {
    Ok(S),
    /// An error type that every custom error created by contract developers can be converted to.
    /// This could potientially have more structure, but String is the easiest.
    Err(String),
}

// Implementations here mimic the Result API and should be implemented via a conversion to Result
// to ensure API consistency
impl<S> ContractResult<S> {
    /// Converts a `ContractResult<S>` to a `Result<S, String>` as a convenient way
    /// to access the full Result API.
    pub fn into_result(self) -> Result<S, String> {
        Result::<S, String>::from(self)
    }

    pub fn unwrap(self) -> S {
        self.into_result().unwrap()
    }

    pub fn is_ok(&self) -> bool {
        matches!(self, ContractResult::Ok(_))
    }

    pub fn is_err(&self) -> bool {
        matches!(self, ContractResult::Err(_))
    }
}

impl<S: fmt::Debug> ContractResult<S> {
    pub fn unwrap_err(self) -> String {
        self.into_result().unwrap_err()
    }
}

impl<S, E: ToString> From<Result<S, E>> for ContractResult<S> {
    fn from(original: Result<S, E>) -> ContractResult<S> {
        match original {
            Ok(value) => ContractResult::Ok(value),
            Err(err) => ContractResult::Err(err.to_string()),
        }
    }
}

impl<S> From<ContractResult<S>> for Result<S, String> {
    fn from(original: ContractResult<S>) -> Result<S, String> {
        match original {
            ContractResult::Ok(value) => Ok(value),
            ContractResult::Err(err) => Err(err),
        }
    }
}
