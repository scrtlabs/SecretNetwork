use serde::{Deserialize, Serialize};
use std::fmt;

use cw_types_v010::encoding::Binary;

use super::{CosmosMsg, Empty, Event};

/// Use this to define when the contract gets a response callback.
/// If you only need it for errors or success you can select just those in order
/// to save gas.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum ReplyOn {
    /// Always perform a callback after SubMsg is processed
    Always,
    /// Only callback if SubMsg returned an error, no callback on success case
    Error,
    /// Only callback if SubMsg was successful, no callback on error case
    Success,
    /// Never make a callback - this is like the original CosmosMsg semantics
    Never,
}

fn bool_false() -> bool {
    false
}

/// A submessage that will guarantee a `reply` call on success or error, depending on
/// the `reply_on` setting. If you do not need to process the result, use regular messages instead.
///
/// Note: On error the submessage execution will revert any partial state changes due to this message,
/// but not revert any state changes in the calling contract. If this is required, it must be done
/// manually in the `reply` entry point.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct SubMsg<T = Empty>
where
    T: Clone + fmt::Debug + PartialEq,
{
    /// An arbitrary ID chosen by the contract.
    /// This is typically used to match `Reply`s in the `reply` entry point to the submessage.
    pub id: u64,
    pub msg: CosmosMsg<T>,
    pub gas_limit: Option<u64>,
    pub reply_on: ReplyOn,
    // An indication that will be passed to the reply that will indicate wether the original message,
    // which is the one who create the submessages, was encrypted or not.
    // Plaintext replies will be encrypted only if the original message was.
    #[serde(default = "bool_false")]
    pub was_msg_encrypted: bool,
}

/// The information we get back from a successful sub message execution,
/// with full Cosmos SDK events.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct SubMsgResponse {
    pub events: Vec<Event>,
    pub data: Option<Binary>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum SubMsgResult {
    Ok(SubMsgResponse),
    /// An error type that every custom error created by contract developers can be converted to.
    /// This could potentially have more structure, but String is the easiest.
    #[serde(rename = "error")]
    Err(String),
}
/// The result object returned to `reply`. We always get the ID from the submessage
/// back and then must handle success and error cases ourselves.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct Reply {
    /// The ID that the contract set when emitting the `SubMsg`.
    /// Use this to identify which submessage triggered the `reply`.
    pub id: Binary,
    pub result: SubMsgResult,
    pub was_orig_msg_encrypted: bool,
    pub is_encrypted: bool,
}
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct DecryptedReply {
    /// The ID that the contract set when emitting the `SubMsg`.
    /// Use this to identify which submessage triggered the `reply`.
    pub id: u64,
    pub result: SubMsgResult,
}

/// The information we get back from a successful sub-call, with full sdk events
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct SubMsgExecutionResponse {
    pub events: Vec<Event>,
    pub data: Option<Binary>,
}
