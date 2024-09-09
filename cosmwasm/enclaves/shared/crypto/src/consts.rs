use std::env;
use std::path;

pub use enclave_ffi_types::{INPUT_ENCRYPTED_SEED_SIZE, OUTPUT_ENCRYPTED_SEED_SIZE};
use lazy_static::lazy_static;
use sgx_types::sgx_quote_sign_type_t;

pub const CERTEXPIRYDAYS: i64 = 3652i64;

#[allow(dead_code)]
#[derive(PartialEq, Eq, Debug)]
pub enum SigningMethod {
    MRSIGNER,
    MRENCLAVE,
    NONE,
}

pub const ATTESTATION_CERTIFICATE_SAVE_PATH: &str = "attestation_cert.der";
pub const ATTESTATION_DCAP_SAVE_PATH: &str = "attestation_dcap.quote";
pub const COLLATERAL_DCAP_SAVE_PATH: &str = "attestation_dcap.collateral";
pub const CERT_COMBINED_SAVE_PATH: &str = "attestation_combined.bin";
pub const MIGRATION_CERT_SAVE_PATH: &str = "migration_report.bin";
pub const PUBKEY_SAVE_PATH: &str = "pubkey.bin";

pub const SEED_EXCH_KEY_SAVE_PATH: &str = "node-master-key.txt";
pub const IO_KEY_SAVE_PATH: &str = "io-master-key.txt";
pub const SEED_UPDATE_SAVE_PATH: &str = "seed.txt";

pub const NODE_EXCHANGE_KEY_FILE: &str = "new_node_seed_exchange_keypair.sealed";
pub const NODE_ENCRYPTED_SEED_KEY_GENESIS_FILE: &str = "consensus_seed.sealed";
pub const NODE_ENCRYPTED_SEED_KEY_CURRENT_FILE: &str = "consensus_seed_current.sealed";

pub const MIGRATION_APPROVAL_SAVE_PATH: &str = "migration_trg.sealed";
pub const MIGRATION_CONSENSUS_SAVE_PATH: &str = "migration_consensus.bin";

#[cfg(feature = "random")]
pub const REK_SEALED_FILE_NAME: &str = "rek.sealed";
#[cfg(feature = "random")]
pub const IRS_SEALED_FILE_NAME: &str = "irs.sealed";

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
pub const SIGNATURE_TYPE: sgx_quote_sign_type_t = sgx_quote_sign_type_t::SGX_LINKABLE_SIGNATURE;

#[cfg(not(feature = "production"))]
pub const SIGNATURE_TYPE: sgx_quote_sign_type_t = sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE;

#[cfg(feature = "production")]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::MRENCLAVE;

#[cfg(all(not(feature = "production"), not(feature = "test")))]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::MRSIGNER;

#[cfg(all(not(feature = "production"), feature = "test"))]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::MRSIGNER;

lazy_static! {
    pub static ref GENESIS_CONSENSUS_SEED_SEALING_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(NODE_ENCRYPTED_SEED_KEY_GENESIS_FILE)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref CURRENT_CONSENSUS_SEED_SEALING_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(NODE_ENCRYPTED_SEED_KEY_CURRENT_FILE)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref REGISTRATION_KEY_SEALING_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(NODE_EXCHANGE_KEY_FILE)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref ATTESTATION_CERT_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(ATTESTATION_CERTIFICATE_SAVE_PATH)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref ATTESTATION_DCAP_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(ATTESTATION_DCAP_SAVE_PATH)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref COLLATERAL_DCAP_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(COLLATERAL_DCAP_SAVE_PATH)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref CERT_COMBINED_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(CERT_COMBINED_SAVE_PATH)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref PUBKEY_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(PUBKEY_SAVE_PATH)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref MIGRATION_CERT_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(MIGRATION_CERT_SAVE_PATH)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref MIGRATION_APPROVAL_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(MIGRATION_APPROVAL_SAVE_PATH)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref MIGRATION_CONSENSUS_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(MIGRATION_CONSENSUS_SAVE_PATH)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
}

#[cfg(feature = "random")]
lazy_static! {
    pub static ref REK_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(REK_SEALED_FILE_NAME)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
    pub static ref IRS_PATH: String = path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string())
    )
    .join(IRS_SEALED_FILE_NAME)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string();
}

pub const CONSENSUS_SEED_EXCHANGE_KEYPAIR_DERIVE_ORDER: u32 = 1;
pub const CONSENSUS_IO_EXCHANGE_KEYPAIR_DERIVE_ORDER: u32 = 2;
pub const CONSENSUS_STATE_IKM_DERIVE_ORDER: u32 = 3;
pub const CONSENSUS_CALLBACK_SECRET_DERIVE_ORDER: u32 = 4;
pub const RANDOMNESS_ENCRYPTION_KEY_SECRET_DERIVE_ORDER: u32 = 5;
pub const INITIAL_RANDOMNESS_SEED_SECRET_DERIVE_ORDER: u32 = 6;
pub const ADMIN_PROOF_SECRET_DERIVE_ORDER: u32 = 7;
pub const CONTRACT_KEY_PROOF_SECRET_DERIVE_ORDER: u32 = 8;

pub const ENCRYPTED_KEY_MAGIC_BYTES: &[u8; 6] = b"secret";
pub const CONSENSUS_SEED_VERSION: u16 = 2;
/// STATE_ENCRYPTION_VERSION is bumped every time we change anything in the state encryption protocol
pub const STATE_ENCRYPTION_VERSION: u32 = 3;

pub const SCRT_SGX_STORAGE_ENV_VAR: &str = "SCRT_SGX_STORAGE";

pub const DEFAULT_SGX_SECRET_PATH: &str = "/opt/secret/.sgx_secrets/";
