use crate::storage::{seal, unseal, DEFAULT_SGX_SECRET_PATH, SCRT_SGX_STORAGE_ENV_VAR};
use log::error;
use serde::{Deserialize, Serialize};
use sgx_types::{sgx_status_t, SgxResult};
use std::{env, path};

const TX_BYTES_FILE_NAME: &str = "tx_bytes.sealed";

fn path_from_env(file_name: &str) -> String {
    path::Path::new(
        &env::var(SCRT_SGX_STORAGE_ENV_VAR).unwrap_or_else(|_| DEFAULT_SGX_SECRET_PATH.to_string()),
    )
        .join(file_name)
        .to_str()
        .unwrap_or(DEFAULT_SGX_SECRET_PATH)
        .to_string()
}

lazy_static::lazy_static! {
    pub static ref TX_BYTES_SEALING_PATH: String = path_from_env(TX_BYTES_FILE_NAME);
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct TxBytesForHeight {
    /// block height for which this set is valid
    pub height: u64,
    /// proto encoded validator set
    pub txs: Vec<Tx>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Tx {
    pub  tx: Vec<u8>
}

impl SignedBlockForHeight {
    pub fn unseal() -> SgxResult<Self> {
        let val_set_from_storage: Self = serde_json::from_slice(
            unseal(&TX_BYTES_SEALING_PATH)?.as_slice(),
        )
            .map_err(|e| {
                error!("Error decoding tx bytes from json {:?}", e);
                sgx_status_t::SGX_ERROR_UNEXPECTED
            })?;

        Ok(val_set_from_storage)
    }

    pub fn seal(&self) -> SgxResult<()> {
        let encoded = serde_json::to_vec(&self).map_err(|e| {
            error!("Error encoding tx bytes to json: {:?}", e);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;

        seal(encoded.as_slice(), &TX_BYTES_SEALING_PATH)
    }
}
