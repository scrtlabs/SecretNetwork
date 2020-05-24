pub use enclave_ffi_types::ENCRYPTED_SEED_SIZE;

#[cfg(feature = "production")]
pub static SPID_FILE: &str = "spid_production.txt";
#[cfg(feature = "production")]
pub static API_KEY_FILE: &str = "key_production.txt";

#[cfg(not(feature = "production"))]
pub static SPID_FILE: &str = "spid.txt";
#[cfg(not(feature = "production"))]
pub static API_KEY_FILE: &str = "api_key.txt";

pub static RA_CERT: &str = "cert.pem";

pub const CERTEXPIRYDAYS: i64 = 90i64;

#[derive(PartialEq, Eq)]
pub enum SigningMethod {
    MRSIGNER,
    MRENCLAVE,
    NONE,
}

#[cfg(feature = "production")]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::MRENCLAVE;

#[cfg(not(feature = "production"))]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::NONE;

pub const CONSENSUS_SEED_SEALING_PATH: &str = "./.sgx_secrets/consensus_seed.sealed";
pub const CONSENSUS_BASE_STATE_KEY_SEALING_PATH: &str =
    "./.sgx_secrets/consensus_base_state_key.sealed";
pub const CONSENSUS_SEED_EXCHANGE_KEYPAIR_SEALING_PATH: &str =
    "./.sgx_secrets/consensus_seed_exchange_keypair.sealed";
pub const CONSENSUS_IO_EXCHANGE_KEYPAIR_SEALING_PATH: &str =
    "./.sgx_secrets/consensus_io_exchange_keypair.sealed";

pub const REGISTRATION_KEY_SEALING_PATH: &str =
    "./.sgx_secrets/new_node_seed_exchange_keypair.sealed";

pub const CONSENSUS_SEED_EXCHANGE_KEYPAIR_DERIVE_ORDER: u32 = 1;
pub const CONSENSUS_IO_EXCHANGE_KEYPAIR_DERIVE_ORDER: u32 = 2;
pub const CONSENSUS_BASE_STATE_KEY_DERIVE_ORDER: u32 = 3;
