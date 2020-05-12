pub const MASTER_STATE_KEY_SEALED_KEY_FILE: &str = "master_state_key_sealed.bin";
pub const MASTER_IO_SEALED_KEY_FILE: &str = "master_io_sealed.bin";
pub const MASTER_RAND_SEED_KEY_FILE: &str = "master_rand_seed_sealed.bin";
pub const SECRET_KEY_SEALED_KEY_FILE: &str = "private_key_sealed.bin";
pub const PUBLIC_KEY_SEALED_KEY_FILE: &str = "public_key_sealed.bin";

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
    NONE
}

#[cfg(feature = "production")]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::MRENCLAVE;

#[cfg(not(feature = "production"))]
pub const SIGNING_METHOD: SigningMethod = SigningMethod::NONE;

pub const SEED_SEALING_PATH: &str = "./.sgx_secrets/seed.sealed";
pub const NODE_SK_SEALING_PATH: &str = "./.sgx_secrets/node_sk_key.sealed";
