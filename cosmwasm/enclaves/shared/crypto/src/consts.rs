use std::borrow::ToOwned;
use std::env;
use std::path;
use std::string::ToString;

use lazy_static::lazy_static;
use log::*;
use sgx_types::{sgx_quote_sign_type_t, sgx_report_body_t, sgx_self_report};

pub const CERTEXPIRYDAYS: i64 = 3652i64;

#[allow(dead_code)]
#[derive(PartialEq, Eq, Debug)]
pub enum SigningMethod {
    MRSIGNER,
    MRENCLAVE,
    NONE,
}

pub const SCRT_SGX_STORAGE_ENV_VAR: &str = "SCRT_SGX_STORAGE";
pub const DEFAULT_SGX_SECRET_PATH: &str = "/opt/secret/.sgx_secrets/";

lazy_static! {
    pub static ref SELF_REPORT_BODY: sgx_report_body_t = {
        let report_body = unsafe {
            let p_report = sgx_self_report();
            (*p_report).body
        };
        trace!(
            "self mr_enclave = {}",
            hex::encode(report_body.mr_enclave.m)
        );
        trace!("self mr_signer  = {}", hex::encode(report_body.mr_signer.m));

        report_body
    };
    pub static ref RESOLVED_SGX_SECRET_PATH: String = {
        if let Ok(env_var) = env::var(SCRT_SGX_STORAGE_ENV_VAR) {
            env_var
        } else {
            DEFAULT_SGX_SECRET_PATH.to_string()
        }
    };
    pub static ref SEALED_FILE_UNITED: String =
        "data-".to_owned() + &hex::encode(SELF_REPORT_BODY.mr_enclave.m) + ".bin";
}

pub fn make_sgx_secret_path(file_name: &str) -> String {
    let sgx_path: &String = &RESOLVED_SGX_SECRET_PATH;
    path::Path::new(sgx_path)
        .join(file_name)
        .to_string_lossy()
        .into_owned()
}

pub const FILE_ATTESTATION_CERTIFICATE: &str = "attestation_cert.der";
pub const FILE_CERT_COMBINED: &str = "attestation_combined.bin";
pub const FILE_MIGRATION_CERT_LOCAL: &str = "migration_report_local.bin";
pub const FILE_MIGRATION_CERT_REMOTE: &str = "migration_report_remote.bin";
pub const FILE_MIGRATION_TARGET_INFO: &str = "migration_target_info.bin";
pub const FILE_MIGRATION_DATA: &str = "migration_data.bin";
pub const FILE_PUBKEY: &str = "pubkey.bin";
pub const FILE_MIGRATION_CONSENSUS: &str = "migration_consensus.json";

pub const SEED_EXCH_KEY_SAVE_PATH: &str = "node-master-key.txt";
pub const IO_KEY_SAVE_PATH: &str = "io-master-key.txt";
pub const SEED_UPDATE_SAVE_PATH: &str = "seed.txt";

pub const SEALED_FILE_ENCRYPTED_SEED_KEY_GENESIS: &str = "consensus_seed.sealed";
pub const SEALED_FILE_ENCRYPTED_SEED_KEY_CURRENT: &str = "consensus_seed_current.sealed";
pub const SEALED_FILE_TX_BYTES: &str = "tx_bytes.sealed";
pub const SEALED_FILE_REGISTRATION_KEY: &str = "new_node_seed_exchange_keypair.sealed";
pub const SEALED_FILE_REK: &str = "rek.sealed";
pub const SEALED_FILE_IRS: &str = "irs.sealed";
pub const SEALED_FILE_VALIDATOR_SET: &str = "validator_set.sealed";

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
