//! Copied and modified from the `wasmer-runtime-core` crate.

use serde::{Deserialize, Serialize};

#[derive(Debug)]
pub struct DeserializeError(String);

impl DeserializeError {
    fn new(msg: String) -> Self {
        Self(msg)
    }
}

impl std::error::Error for DeserializeError {}

impl std::fmt::Display for DeserializeError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(formatter, "Deserialization error: {:?}", self.0)
    }
}

type Result<T> = core::result::Result<T, DeserializeError>;

/// The hash of a wasm module.
///
/// Used as a key when loading and storing modules in a [`Cache`].
///
/// [`Cache`]: trait.Cache.html
#[derive(Debug, Copy, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
// WasmHash is made up of a 32 byte array
pub struct WasmHash([u8; 32]);
use sha2::{Digest, Sha256};

impl WasmHash {
    /// Hash a wasm module.
    ///
    /// # Note:
    /// This does no verification that the supplied data
    /// is, in fact, a wasm module.
    pub fn generate(wasm: &[u8]) -> Self {
        let hash = Sha256::digest(wasm);
        WasmHash(hash.into())
    }

    /// Create the hexadecimal representation of the
    /// stored hash.
    #[allow(dead_code)]
    pub fn encode(self) -> String {
        hex::encode(&self.into_array() as &[u8])
    }

    /// Create hash from hexadecimal representation
    #[allow(dead_code)]
    pub fn decode(hex_str: &str) -> Result<Self> {
        let bytes = hex::decode(hex_str).map_err(|e| {
            DeserializeError::new(format!(
                "Could not decode prehashed key as hexadecimal: {}",
                e
            ))
        })?;
        if bytes.len() != 32 {
            return Err(DeserializeError::new(
                "Prehashed keys must deserialze into exactly 32 bytes".to_string(),
            ));
        }
        use std::convert::TryInto;
        Ok(WasmHash(bytes[0..32].try_into().map_err(|e| {
            DeserializeError::new(format!("Could not get first 32 bytes: {}", e))
        })?))
    }

    pub(crate) fn into_array(self) -> [u8; 32] {
        let mut total = [0u8; 32];
        total[0..32].copy_from_slice(&self.0);
        total
    }
}
