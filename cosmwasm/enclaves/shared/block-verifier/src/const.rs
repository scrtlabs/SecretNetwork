use enclave_crypto::consts::{DEFAULT_SGX_SECRET_PATH, SCRT_SGX_STORAGE_ENV_VAR};
use lazy_static::lazy_static;
use std::{env, path};

const VALIDATOR_SET_FILE_NAME: &str = "validator_set.sealed";

fn path_from_env(file_name: &str) -> String {
    path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string()),
    )
    .join(file_name)
    .to_str()
    .unwrap_or(DEFAULT_SGX_SECRET_PATH)
    .to_string()
}

lazy_static! {
    pub static ref VALIDATOR_SET_SEALING_PATH: String = path_from_env(VALIDATOR_SET_FILE_NAME);
}
