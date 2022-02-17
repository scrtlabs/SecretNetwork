use std::env;
use std::path;

pub use enclave_ffi_types::ENCRYPTED_SEED_SIZE;
use lazy_static::lazy_static;

pub const CERTEXPIRYDAYS: i64 = 3652i64;

#[allow(dead_code)]
#[derive(PartialEq, Eq, Debug)]
pub enum SigningMethod {
    MRSIGNER,
    MRENCLAVE,
    NONE,
}

pub const ATTESTATION_CERTIFICATE_SAVE_PATH: &str = "attestation_cert.der";

pub const SEED_EXCH_CERTIFICATE_SAVE_PATH: &str = "node-master-cert.der";
pub const IO_CERTIFICATE_SAVE_PATH: &str = "io-master-cert.der";

pub const NODE_EXCHANGE_KEY_FILE: &str = "new_node_seed_exchange_keypair.sealed";
pub const NODE_ENCRYPTED_SEED_KEY_FILE: &str = "consensus_seed.sealed";

//todo: set this to the real value
#[cfg(feature = "production")]
pub const MRSIGNER: [u8; 32] = [
    132, 92, 243, 72, 20, 244, 85, 149, 199, 32, 248, 31, 116, 121, 77, 120, 89, 49, 72, 79, 68, 5,
    214, 229, 3, 110, 248, 135, 19, 255, 63, 166,
];

#[cfg(not(feature = "production"))]
pub const MRSIGNER: [u8; 32] = [
    131, 215, 25, 231, 125, 234, 202, 20, 112, 246, 186, 246, 42, 77, 119, 67, 3, 200, 153, 219,
    105, 2, 15, 156, 112, 238, 29, 252, 8, 199, 206, 158,
];

#[cfg(feature = "production")]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::MRENCLAVE;

#[cfg(all(not(feature = "production"), not(feature = "test")))]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::MRENCLAVE;

#[cfg(all(not(feature = "production"), feature = "test"))]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::MRSIGNER;

lazy_static! {
    pub static ref CONSENSUS_SEED_SEALING_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(NODE_ENCRYPTED_SEED_KEY_FILE)
    .to_str()
    .unwrap_or(&DEFAULT_SGX_SECRET_PATH.to_string())
    .to_string();
    pub static ref REGISTRATION_KEY_SEALING_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(NODE_EXCHANGE_KEY_FILE)
    .to_str()
    .unwrap_or(&DEFAULT_SGX_SECRET_PATH.to_string())
    .to_string();
    pub static ref ATTESTATION_CERT_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(ATTESTATION_CERTIFICATE_SAVE_PATH)
    .to_str()
    .unwrap_or(&DEFAULT_SGX_SECRET_PATH.to_string())
    .to_string();
}

pub const CONSENSUS_SEED_EXCHANGE_KEYPAIR_DERIVE_ORDER: u32 = 1;
pub const CONSENSUS_IO_EXCHANGE_KEYPAIR_DERIVE_ORDER: u32 = 2;
pub const CONSENSUS_STATE_IKM_DERIVE_ORDER: u32 = 3;
pub const CONSENSUS_CALLBACK_SECRET_DERIVE_ORDER: u32 = 4;

pub const SCRT_SGX_STORAGE_ENV_VAR: &str = "SCRT_SGX_STORAGE";

const DEFAULT_SGX_SECRET_PATH: &str = "/opt/secret/.sgx_secrets/";
