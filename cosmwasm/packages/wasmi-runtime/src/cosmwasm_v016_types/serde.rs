// This file simply re-exports some methods from serde_json
// The reason is two fold:
// 1. To easily ensure that all calling libraries use the same version (minimize code size)
// 2. To allow us to switch out to eg. serde-json-core more easily
use serde::{de::DeserializeOwned, Serialize};
use std::any::type_name;

use crate::cosmwasm_v016_types::binary::Binary;
use crate::cosmwasm_v016_types::errors::{StdError, StdResult};

pub fn from_slice<T: DeserializeOwned>(value: &[u8]) -> StdResult<T> {
    serde_json_wasm::from_slice(value).map_err(|e| StdError::parse_err(type_name::<T>(), e))
}

pub fn from_binary<T: DeserializeOwned>(value: &Binary) -> StdResult<T> {
    from_slice(value.as_slice())
}

pub fn to_vec<T>(data: &T) -> StdResult<Vec<u8>>
where
    T: Serialize + ?Sized,
{
    serde_json_wasm::to_vec(data).map_err(|e| StdError::serialize_err(type_name::<T>(), e))
}

pub fn to_binary<T>(data: &T) -> StdResult<Binary>
where
    T: Serialize + ?Sized,
{
    to_vec(data).map(Binary)
}
