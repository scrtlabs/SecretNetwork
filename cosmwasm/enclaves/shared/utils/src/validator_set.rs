use crate::storage::{seal, unseal};
use log::error;
use serde::{Deserialize, Serialize};
use sgx_types::{sgx_status_t, SgxResult};
use std::{env, path};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ValidatorSetForHeight {
    /// block height for which this set is valid
    pub height: u64,
    /// proto encoded validator set
    pub validator_set: Vec<u8>,
}

impl ValidatorSetForHeight {
    pub fn unseal_from(path: &str) -> SgxResult<Self> {
        let val_set_from_storage: Self =
            serde_json::from_slice(unseal(path)?.as_slice()).map_err(|e| {
                error!("Error decoding validator set from json {:?}", e);
                sgx_status_t::SGX_ERROR_UNEXPECTED
            })?;

        Ok(val_set_from_storage)
    }

    pub fn seal_to(&self, path: &str) -> SgxResult<()> {
        let encoded = serde_json::to_vec(&self).map_err(|e| {
            error!("Error encoding validator set to json: {:?}", e);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;

        seal(encoded.as_slice(), path)
    }
}
