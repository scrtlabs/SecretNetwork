use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
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

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum ExecuteMsg {
    Noop {},
    BenchCPU {},
    BenchReadStorage {},
    BenchWriteStorage {},
    BenchReadStorageMultipleKeys {},
    BenchAllocate {},
    BenchReadLargeItemFromStorage {},
    BenchWriteLargeItemToStorage {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryMsg {}
