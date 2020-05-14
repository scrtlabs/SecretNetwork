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

pub const SEED_SEALING_PATH: &str = "./.sgx_secrets/seed.sealed";
pub const NODE_SK_SEALING_PATH: &str = "./.sgx_secrets/node_sk_key.sealed";
pub const IO_KEY_SEALING_KEY_PATH: &str = "./.sgx_secrets/io_sk_key.sealed";
pub const base_state_key_PATH: &str = "./.sgx_secrets/base_state_key_sealed.sealed";

pub const IO_KEY_DERIVE_ORDER: u32 = 1;
pub const STATE_MASTER_KEY_DERIVE_ORDER: u32 = 2;
