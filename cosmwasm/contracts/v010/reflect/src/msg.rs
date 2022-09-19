use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

use cosmwasm_std::{Binary, CosmosMsg, HumanAddr, QueryRequest};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct InitMsg {}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    ReflectMsg { msgs: Vec<CosmosMsg<CustomMsg>> },
    ChangeOwner { owner: HumanAddr },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryMsg {
    Owner {},
    // this will call out to CustomQuery::Capitalize
    ReflectCustom { text: String },
}

// We define a custom struct for each query response
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct OwnerResponse {
    pub owner: HumanAddr,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
/// CustomMsg is an override of CosmosMsg::Custom to show this works and can be extended in the contract
pub enum CustomMsg {
    Debug(String),
    Raw(Binary),
}

impl Into<CosmosMsg<CustomMsg>> for CustomMsg {
    fn into(self) -> CosmosMsg<CustomMsg> {
        CosmosMsg::Custom(self)
    }
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
/// CustomQuery is an override of QueryRequest::Custom to show this works and can be extended in the contract
pub enum CustomQuery {
    Ping {},
    Capital { text: String },
}

// TODO: do we want to standardize this somehow for all?
impl Into<QueryRequest<CustomQuery>> for CustomQuery {
    fn into(self) -> QueryRequest<CustomQuery> {
        QueryRequest::Custom(self)
    }
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
// All return values of CustomQuery are CustomResponse
pub struct CustomResponse {
    pub msg: String,
}
