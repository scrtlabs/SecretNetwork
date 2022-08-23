use log::warn;
use serde::{Deserialize, Serialize};

use cw_types_v010::types::{Env as V010Env, HumanAddr};
use cw_types_v1::types::Env as V1Env;
use cw_types_v1::types::MessageInfo as V1MessageInfo;

use cw_types_v010::types as v010types;
use cw_types_v1::types as v1types;

use enclave_ffi_types::EnclaveError;
use hex;

/// CosmwasmApiVersion is used to decide how to handle contract inputs and outputs
pub enum CosmWasmApiVersion {
    /// CosmWasm v0.10 API
    V010,
    /// CosmWasm v1 API
    V1,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub struct BaseEnv(pub V010Env);

impl BaseEnv {
    pub fn get_verification_params(&self) -> (&HumanAddr, &HumanAddr, u64, &Vec<v010types::Coin>) {
        (
            &self.0.message.sender,
            &self.0.contract.address,
            self.0.block.height,
            &self.0.message.sent_funds,
        )
    }

    pub fn into_versioned_env(self, api_version: &CosmWasmApiVersion) -> CwEnv {
        match api_version {
            CosmWasmApiVersion::V010 => self.into_v010(),
            CosmWasmApiVersion::V1 => self.into_v1(),
        }
    }

    fn into_v010(self) -> CwEnv {
        CwEnv::V010Env { env: self.0 }
    }

    /// This is the conversion function from the base to the new env. We assume that if there are
    /// any API changes that are necessary on the base level we will have to update this as well
    fn into_v1(self) -> CwEnv {
        CwEnv::V1Env {
            env: V1Env {
                block: v1types::BlockInfo {
                    height: self.0.block.height,
                    time: v1types::Timestamp::from_nanos(self.0.block.time),
                    chain_id: self.0.block.chain_id.clone(),
                },
                contract: v1types::ContractInfo {
                    address: v1types::Addr::unchecked(self.0.contract.address.0),
                    code_hash: self.0.contract_code_hash,
                },
            },
            msg_info: v1types::MessageInfo {
                sender: v1types::Addr::unchecked(self.0.message.sender.0),
                funds: self
                    .0
                    .message
                    .sent_funds
                    .into_iter()
                    .map(|x| x.into())
                    .collect(),
            },
        }
    }
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum CwEnv {
    V010Env { env: V010Env },
    V1Env { env: V1Env, msg_info: V1MessageInfo },
}

impl CwEnv {
    pub fn get_contract_hash(&self) -> &String {
        match self {
            CwEnv::V010Env { env } => &env.contract_code_hash,
            CwEnv::V1Env { env, .. } => &env.contract.code_hash,
        }
    }

    pub fn set_contract_hash(&mut self, contract_hash: &[u8; 32]) {
        match self {
            CwEnv::V010Env { env } => {
                env.contract_code_hash = hex::encode(contract_hash);
            }
            CwEnv::V1Env { env, .. } => {
                env.contract.code_hash = hex::encode(contract_hash);
            }
        }
    }

    pub fn get_wasm_ptrs(&self) -> Result<(Vec<u8>, Vec<u8>), EnclaveError> {
        match self {
            CwEnv::V010Env { env } => {
                let env_bytes = serde_json::to_vec(env).map_err(|err| {
                    warn!(
                    "got an error while trying to serialize env_v010 (cosmwasm v0.10) into bytes {:?}: {}",
                    env, err
                );
                    EnclaveError::FailedToSerialize
                })?;

                Ok((env_bytes, vec![]))
            }
            CwEnv::V1Env { env, msg_info } => {
                let env_bytes = serde_json::to_vec(env).map_err(|err| {
                    warn!(
                    "got an error while trying to serialize env_v010 (cosmwasm v0.10) into bytes {:?}: {}",
                    env, err
                );
                    EnclaveError::FailedToSerialize
                })?;
                let msg_bytes = serde_json::to_vec(msg_info).map_err(|err| {
                    warn!(
                    "got an error while trying to serialize env_v010 (cosmwasm v0.10) into bytes {:?}: {}",
                    msg_info, err
                );
                    EnclaveError::FailedToSerialize
                })?;

                Ok((env_bytes, msg_bytes))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn it_works() {
        assert_eq!(2 + 2, 4);
    }
}
