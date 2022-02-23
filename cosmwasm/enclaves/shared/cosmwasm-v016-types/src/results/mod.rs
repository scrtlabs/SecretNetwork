//! This module contains the messages that are sent from the contract to the VM as an execution result

mod contract_result;
mod cosmos_msg;
mod empty;
mod events;
mod response;
mod submessages;

pub use contract_result::ContractResult;
pub use cosmos_msg::{BankMsg, CosmosMsg, WasmMsg};
#[cfg(feature = "staking")]
pub use cosmos_msg::{DistributionMsg, StakingMsg};
#[cfg(feature = "stargate")]
pub use cosmos_msg::{GovMsg, VoteOption};
pub use empty::Empty;
pub use events::{Attribute, Event};
pub use response::Response;
pub use submessages::{Reply, ReplyOn, SubMsg, SubMsgExecutionResponse};
