use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

use cosmwasm::types::{CosmosMsg, HumanAddr};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum InitMsg {
       Nop { },
       Callback {
        contract_addr: HumanAddr,
       },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum HandleMsg {
    A {
        contract_addr: HumanAddr,
        x: u8,
        y: u8,
    },
    B {
        contract_addr: HumanAddr,
        x: u8,
        y: u8,
    },
    C {
        x: u8,
        y: u8,
    },
    EmptyLogKeyValue { },
    EmptyData { },
    NoData { },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum QueryMsg {
    Owner {},
}

// We define a custom struct for each query response
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct OwnerResponse {
    pub owner: HumanAddr,
}
