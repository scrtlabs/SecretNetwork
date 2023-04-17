use log::warn;
use serde::{Deserialize, Serialize};

#[cfg(feature = "random")]
use cw_types_v010::encoding::Binary;

use cw_types_v010::types as v010types;
use cw_types_v010::types::{Env as V010Env, HumanAddr};
use cw_types_v1::types as v1types;
use cw_types_v1::types::Env as V1Env;
use cw_types_v1::types::MessageInfo as V1MessageInfo;
use enclave_ffi_types::EnclaveError;

pub const CONTRACT_KEY_LENGTH: usize = 64;

/// CosmwasmApiVersion is used to decide how to handle contract inputs and outputs
#[derive(Serialize, Deserialize, Copy, Clone, Debug, PartialEq)]
pub enum CosmWasmApiVersion {
    /// CosmWasm v0.10 API
    V010,
    /// CosmWasm v1 API
    V1,
    /// CosmWasm version invalid
    Invalid,
}

/// features that a contract requires
#[derive(Serialize, Deserialize, Copy, Clone, Debug, PartialEq)]
pub enum ContractFeature {
    Random,
}

pub type BaseAddr = HumanAddr;
pub type BaseCoin = v010types::Coin;
pub type BaseCanoncalAddr = v010types::CanonicalAddr;

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub struct BaseEnv(pub V010Env);

impl BaseEnv {
    pub fn get_contract_key(&self) -> Result<[u8; CONTRACT_KEY_LENGTH], EnclaveError> {
        let contract_key = if let Some(b64_key) = &self.0.contract_key {
            base64::decode(b64_key).map_err(|err| {
                warn!(
                    "got an error while trying to decode contract key {:?}: {}",
                    b64_key, err
                );
                EnclaveError::FailedContractAuthentication
            })?
        } else {
            warn!("Contract execute with empty contract key");
            return Err(EnclaveError::FailedContractAuthentication);
        };

        if contract_key.len() != CONTRACT_KEY_LENGTH {
            warn!("Contract execute with empty contract key");
            return Err(EnclaveError::FailedContractAuthentication);
        }

        let mut key_as_bytes = [0u8; CONTRACT_KEY_LENGTH];
        key_as_bytes.copy_from_slice(&contract_key);

        Ok(key_as_bytes)
    }

    pub fn get_verification_params(&self) -> (&BaseAddr, &BaseAddr, u64, &Vec<BaseCoin>) {
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
            CosmWasmApiVersion::Invalid => panic!("Can't parse invalid env"),
        }
    }

    fn into_v010(self) -> CwEnv {
        // Assaf: contract_key is irrelevant inside the contract,
        // but existing v0.10 contracts might expect it to be populated :facepalm:,
        // therefore we are going to leave it populated :shrug:.

        // in secretd v1.3 the timestamp passed from Go was unix time in seconds
        // from secretd v1.4 the timestamp passed from Go is unix time in nanoseconds
        // v0.10 time is seconds since unix epoch
        // v1 time is nanoseconds since unix epoch
        // so we need to convert it from nanoseconds to seconds

        CwEnv::V010Env {
            env: V010Env {
                block: v010types::BlockInfo {
                    height: self.0.block.height,
                    // v0.10 env.block.time is seconds since unix epoch
                    time: v1types::Timestamp::from_nanos(self.0.block.time).seconds(),
                    chain_id: self.0.block.chain_id,
                    #[cfg(feature = "random")]
                    random: None,
                },
                message: v010types::MessageInfo {
                    sender: self.0.message.sender,
                    sent_funds: self.0.message.sent_funds,
                },
                contract: v010types::ContractInfo {
                    address: self.0.contract.address,
                },
                contract_key: self.0.contract_key,
                contract_code_hash: self.0.contract_code_hash,
                transaction: None,
            },
        }
    }

    /// This is the conversion function from the base to the new env. We assume that if there are
    /// any API changes that are necessary on the base level we will have to update this as well
    fn into_v1(self) -> CwEnv {
        CwEnv::V1Env {
            env: V1Env {
                block: v1types::BlockInfo {
                    height: self.0.block.height,
                    // v1 env.block.time is nanoseconds since unix epoch
                    time: v1types::Timestamp::from_nanos(self.0.block.time),
                    chain_id: self.0.block.chain_id,
                    #[cfg(feature = "random")]
                    random: self.0.block.random,
                },
                contract: v1types::ContractInfo {
                    address: v1types::Addr::unchecked(self.0.contract.address.0),
                    code_hash: self.0.contract_code_hash,
                },
                transaction: self.0.transaction,
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
    pub fn is_v1(&self) -> bool {
        matches!(self, CwEnv::V1Env { .. })
    }

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

    #[cfg(feature = "random")]
    pub fn set_random(&mut self, random: Option<Binary>) {
        match self {
            CwEnv::V010Env { .. } => {}
            CwEnv::V1Env { env, .. } => {
                env.block.random = random;
            }
        }
    }

    #[cfg(feature = "random")]
    pub fn get_random(&self) -> Option<Binary> {
        match self {
            CwEnv::V010Env { .. } => None,
            CwEnv::V1Env { env, .. } => env.block.random.clone(),
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
