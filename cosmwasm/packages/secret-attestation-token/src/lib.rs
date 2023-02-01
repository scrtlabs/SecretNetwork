// Apache Teaclave (incubating)
// Copyright 2019-2020 The Apache Software Foundation
//
// This product includes software developed at
// The Apache Software Foundation (http://www.apache.org/).
//! Types that contain information about attestation report.
//! The implementation is based on Attestation Service API version 4.
//! https://api.trustedservices.intel.com/documents/sgx-attestation-api-spec.pdf

use std::array::TryFromSliceError;

use serde::{Deserialize, Deserializer, Serialize, Serializer};

pub type UserData = [u8; 32];
pub type AttestationNonce = [u8; 16];
pub type NodeAuthPublicKey = [u8; 32];

// ------------------------ types --------------------- //

#[derive(Serialize, Deserialize)]
pub enum AttestationType {
    SgxEpid = 0,
    SgxDcap = 1,
    SgxSw = 2,
    Other = 3,
}

#[derive(Debug)]
pub enum Error {
    ReportParseError,
    ReportValidationError,
}

/// todo: replace this with a more generic access token that can be signed by an external service
/// AttestationReport can be signed by either the Intel Attestation Service
/// using EPID or Data Center Attestation Service (platform dependent) using ECDSA.
/// Or even something non-SGX
#[derive(Default, Serialize, Deserialize)]
pub struct SecretAttestationToken {
    /// Attestation type - could be SGX or non SGX
    pub attestation_type: AttestationType,
    /// Attestation data of whatever type this is. Keeping this generic for now without an
    /// enum or defined types to keep dependencies separate (enclave types vs non-enclave, etc.)
    /// i.e. make decoding in go or without having to have a single type with a ton of dependencies
    #[serde(serialize_with = "as_base64", deserialize_with = "from_base64")]
    pub data: Vec<u8>,

    /// `node_key` specifies the attesting node's public key. This is here to avoid having to parse
    /// the exact type of the signed attestation token
    #[serde(
        serialize_with = "as_base64_array",
        deserialize_with = "from_base64_array"
    )]
    pub node_key: NodeAuthPublicKey,
    /// block info contains data that we will use to validate the timestamp in the token
    pub block_info: BlockInfo,
    /// signature of the validation service (if in use)
    #[serde(serialize_with = "as_base64", deserialize_with = "from_base64")]
    pub signature: Vec<u8>,
    /// certificate of the validation service that signed the request
    #[serde(serialize_with = "as_base64", deserialize_with = "from_base64")]
    pub signing_cert: Vec<u8>,
}

#[derive(Default, Serialize, Deserialize)]
// not final - will depend on exact algorithm
pub struct BlockInfo {
    /// block height
    pub height: u64,
    /// block hash for this height
    pub hash: [u8; 32],
    /// time in seconds from epoch as it appears in the block
    pub time: u64,
}

pub enum VerificationError {
    ErrorGeneric,
}

pub enum GenerationError {
    ErrorGeneric,
}

// -------------------- traits ------------------- //
pub trait AsAttestationToken {
    fn as_attestation_token(&self) -> SecretAttestationToken;
}

pub trait FromAttestationToken<T: Sized> {
    fn from_attestation_token(other: &SecretAttestationToken) -> T;
}

pub trait AuthenticationMaterialVerify {
    fn verify(&self) -> Result<NodeAuthPublicKey, VerificationError>;
}

pub trait AuthenticationMaterialGenerate {
    fn generate(&self) -> Result<Self, GenerationError>
    where
        Self: Sized;
}

// ------------------------ impls --------------------- //

impl From<std::array::TryFromSliceError> for Error {
    fn from(_: TryFromSliceError) -> Self {
        Error::ReportParseError
    }
}

#[cfg(feature = "sgx")]
impl From<serde_json::error::Error> for Error {
    fn from(_: serde_json::error::Error) -> Self {
        Error::ReportParseError
    }
}

impl Default for AttestationType {
    fn default() -> Self {
        Self::Other
    }
}

impl From<u8> for AttestationType {
    fn from(other: u8) -> Self {
        match other {
            0 => AttestationType::SgxEpid,
            1 => AttestationType::SgxDcap,
            2 => AttestationType::SgxSw,
            3 => AttestationType::Other,
            _ => AttestationType::Other,
        }
    }
}

impl Into<u8> for AttestationType {
    fn into(self) -> u8 {
        match self {
            AttestationType::SgxEpid => 0u8,
            AttestationType::SgxDcap => 1u8,
            AttestationType::SgxSw => 2u8,
            AttestationType::Other => 3u8,
        }
    }
}

// ------------------------ serialize/deserialize helper functions --------------------- //

fn as_base64_array<S>(key: &[u8; 32], serializer: S) -> Result<S::Ok, S::Error>
where
    S: Serializer,
{
    serializer.serialize_str(&base64::encode(key))
}

fn from_base64_array<'de, D>(deserializer: D) -> Result<NodeAuthPublicKey, D::Error>
where
    D: Deserializer<'de>,
{
    struct Base64Visitor;

    impl<'de> serde::de::Visitor<'de> for Base64Visitor {
        type Value = NodeAuthPublicKey;

        fn expecting(&self, formatter: &mut ::std::fmt::Formatter) -> std::fmt::Result {
            write!(formatter, "base64 ASCII text")
        }

        fn visit_str<E>(self, v: &str) -> Result<Self::Value, E>
        where
            E: serde::de::Error,
        {
            let res = base64::decode(v).map_err(E::custom)?;
            if res.len() != 32 {
                return Err(E::custom("Node key length invalid"));
            }
            let mut node_key = NodeAuthPublicKey::default();
            node_key.copy_from_slice(&res);
            Ok(node_key)
        }
    }
    deserializer.deserialize_str(Base64Visitor)
}

fn as_base64<S>(key: &[u8], serializer: S) -> Result<S::Ok, S::Error>
where
    S: Serializer,
{
    serializer.serialize_str(&base64::encode(key))
}

fn from_base64<'de, D>(deserializer: D) -> Result<Vec<u8>, D::Error>
where
    D: Deserializer<'de>,
{
    struct Base64Visitor;

    impl<'de> serde::de::Visitor<'de> for Base64Visitor {
        type Value = Vec<u8>;

        fn expecting(&self, formatter: &mut ::std::fmt::Formatter) -> std::fmt::Result {
            write!(formatter, "base64 ASCII text")
        }

        fn visit_str<E>(self, v: &str) -> Result<Self::Value, E>
        where
            E: serde::de::Error,
        {
            base64::decode(v).map_err(E::custom)
        }
    }
    deserializer.deserialize_str(Base64Visitor)
}

#[cfg(test)]
mod tests {
    #[test]
    fn it_works() {
        let result = 2 + 2;
        assert_eq!(result, 4);
    }
}
