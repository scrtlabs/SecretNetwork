use std::fmt;
use std::ops::Deref;

use enclave_ffi_types::EnclaveError;
use log::warn;
use serde::{de, ser, Deserialize, Deserializer, Serialize};

use super::errors::{StdError, StdResult};

/// Binary is a wrapper around Vec<u8> to add base64 de/serialization
/// with serde. It also adds some helper methods to help encode inline.
///
/// This is only needed as serde-json-{core,wasm} has a horrible encoding for Vec<u8>
#[derive(Clone, Default, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct Binary(pub Vec<u8>);

impl Binary {
    /// take an (untrusted) string and decode it into bytes.
    /// fails if it is not valid base64
    pub fn from_base64(encoded: &str) -> Result<Self, EnclaveError> {
        let binary = base64::decode(encoded).map_err(|err| {
            warn!("Failed to decode base64 string: {:?}", err.to_string());
            EnclaveError::FailedToDeserialize
        })?;
        Ok(Binary(binary))
    }

    /// encode to base64 string (guaranteed to be success as we control the data inside).
    /// this returns normalized form (with trailing = if needed)
    pub fn to_base64(&self) -> String {
        base64::encode(&self.0)
    }

    pub fn as_slice(&self) -> &[u8] {
        self.0.as_slice()
    }

    /// Copies content into fixed-sized array.
    /// The result type `A: ByteArray` is a workaround for
    /// the missing [const-generics](https://rust-lang.github.io/rfcs/2000-const-generics.html).
    /// `A` is a fixed-sized array like `[u8; 8]`.
    ///
    /// ByteArray is implemented for `[u8; 0]` to `[u8; 64]`, such that
    /// we are limited to 64 bytes for now.
    ///
    /// # Examples
    ///
    /// Copy to array of explicit length
    ///
    /// ```
    /// # use cosmwasm_std::Binary;
    /// let binary = Binary::from(&[0xfb, 0x1f, 0x37]);
    /// let array: [u8; 3] = binary.to_array().unwrap();
    /// assert_eq!(array, [0xfb, 0x1f, 0x37]);
    /// ```
    ///
    /// Copy to integer
    ///
    /// ```
    /// # use cosmwasm_std::Binary;
    /// let binary = Binary::from(&[0x8b, 0x67, 0x64, 0x84, 0xb5, 0xfb, 0x1f, 0x37]);
    /// let num = u64::from_be_bytes(binary.to_array().unwrap());
    /// assert_eq!(num, 10045108015024774967);
    /// ```
    pub fn to_array<const LENGTH: usize>(&self) -> StdResult<[u8; LENGTH]> {
        if self.len() != LENGTH {
            return Err(StdError::invalid_data_size(LENGTH, self.len()));
        }

        let mut out: [u8; LENGTH] = [0; LENGTH];
        out.copy_from_slice(&self.0);
        Ok(out)
    }
}

impl fmt::Display for Binary {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.to_base64())
    }
}

impl From<&[u8]> for Binary {
    fn from(binary: &[u8]) -> Self {
        Self(binary.to_vec())
    }
}

/// Just like Vec<u8>, Binary is a smart pointer to [u8].
/// This implements `*binary` for us and allows us to
/// do `&*binary`, returning a `&[u8]` from a `&Binary`.
/// With [deref coercions](https://doc.rust-lang.org/1.22.1/book/first-edition/deref-coercions.html#deref-coercions),
/// this allows us to use `&binary` whenever a `&[u8]` is required.
impl Deref for Binary {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        self.as_slice()
    }
}

// Reference
impl<const LENGTH: usize> From<&[u8; LENGTH]> for Binary {
    fn from(source: &[u8; LENGTH]) -> Self {
        Self(source.to_vec())
    }
}

// Owned
impl<const LENGTH: usize> From<[u8; LENGTH]> for Binary {
    fn from(source: [u8; LENGTH]) -> Self {
        Self(source.into())
    }
}

impl From<Vec<u8>> for Binary {
    fn from(vec: Vec<u8>) -> Self {
        Self(vec)
    }
}

impl From<Binary> for Vec<u8> {
    fn from(original: Binary) -> Vec<u8> {
        original.0
    }
}

/// Implement `encoding::Binary == std::vec::Vec<u8>`
impl PartialEq<Vec<u8>> for Binary {
    fn eq(&self, rhs: &Vec<u8>) -> bool {
        // Use Vec<u8> == Vec<u8>
        self.0 == *rhs
    }
}

/// Implement `std::vec::Vec<u8> == encoding::Binary`
impl PartialEq<Binary> for Vec<u8> {
    fn eq(&self, rhs: &Binary) -> bool {
        // Use Vec<u8> == Vec<u8>
        *self == rhs.0
    }
}

/// Implement `Binary == &[u8]`
impl PartialEq<&[u8]> for Binary {
    fn eq(&self, rhs: &&[u8]) -> bool {
        // Use &[u8] == &[u8]
        self.as_slice() == *rhs
    }
}

/// Implement `&[u8] == Binary`
impl PartialEq<Binary> for &[u8] {
    fn eq(&self, rhs: &Binary) -> bool {
        // Use &[u8] == &[u8]
        *self == rhs.as_slice()
    }
}

/// Serializes as a base64 string
impl Serialize for Binary {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: ser::Serializer,
    {
        serializer.serialize_str(&self.to_base64())
    }
}

/// Deserializes as a base64 string
impl<'de> Deserialize<'de> for Binary {
    fn deserialize<D>(deserializer: D) -> Result<Binary, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_str(Base64Visitor)
    }
}

struct Base64Visitor;

impl<'de> de::Visitor<'de> for Base64Visitor {
    type Value = Binary;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("valid base64 encoded string")
    }

    fn visit_str<E>(self, v: &str) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        match Binary::from_base64(v) {
            Ok(binary) => Ok(binary),
            Err(_) => Err(E::custom(format!("invalid base64: {}", v))),
        }
    }
}
