use crate::viewing_key_obj::ViewingKeyObj;
use schemars::JsonSchema;
use secret_toolkit::permit::Permit;
use serde::{Deserialize, Serialize};

use cosmwasm_std::Uint128;

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum InstantiateMsg {
    Init {},
}

// #[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
// #[serde(rename_all = "snake_case")]
// pub struct BenchResponse {
//     /// benchmark name
//     name: String,
//     /// time in nanos to run the test
//     time: u64,
// }

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ExecuteMsg {
    Noop {},
    BenchCPU {},
    BenchReadStorage {},
    BenchWriteStorage {},
    BenchReadStorageMultipleKeys {},
    BenchAllocate {},
    BenchReadLargeItemFromStorage {},
    BenchReadLargeItemsFromStorage {},
    BenchWriteLargeItemToStorage {
        chunks: String,
    },
    BenchCreateViewingKey {},
    BenchSetViewingKey {
        key: String,
        padding: Option<String>,
    },
    SetupReadLargeItem {},
    SetupReadLargeItems {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, Eq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryWithPermit {
    Balance {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, Eq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryMsg {
    NoopQuery {},
    BenchGetBalanceWithViewingKey {
        address: String,
        key: String,
    },
    BenchGetBalanceWithPermit {
        permit: Permit,
        query: QueryWithPermit,
    },
}

impl QueryMsg {
    pub fn get_validation_params(&self) -> (Vec<&String>, ViewingKeyObj) {
        match self {
            Self::BenchGetBalanceWithViewingKey { address, key } => {
                (vec![address], ViewingKeyObj(key.clone()))
            }
            _ => panic!("This query type does not require authentication"),
        }
    }
}

#[derive(Serialize, Deserialize, JsonSchema, Debug)]
#[serde(rename_all = "snake_case")]
pub enum QueryAnswer {
    Balance { amount: Uint128 },
    ViewingKeyError { msg: String },
}
