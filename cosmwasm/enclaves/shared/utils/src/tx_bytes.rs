use crate::storage::{seal, unseal};
use enclave_crypto::consts::{make_sgx_secret_path, SEALED_FILE_TX_BYTES};
use log::error;
use serde::{Deserialize, Serialize};
use sgx_types::{sgx_status_t, SgxResult};
use std::{env, path};

lazy_static::lazy_static! {
    pub static ref TX_BYTES_SEALING_PATH: String = make_sgx_secret_path(SEALED_FILE_TX_BYTES);
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
    pub tx: Vec<u8>,
}

impl TxBytesForHeight {
    pub fn unseal() -> SgxResult<Self> {
        let val_set_from_storage: Self =
            serde_json::from_slice(unseal(&TX_BYTES_SEALING_PATH)?.as_slice()).map_err(|e| {
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
