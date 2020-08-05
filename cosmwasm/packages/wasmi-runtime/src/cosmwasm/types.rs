//! must keep this file in sync with cosmwasm/packages/std/src/types.rs and cosmwasm/packages/std/src/init_handle.rs

#![allow(unused)]

/// These types are are copied over from the cosmwasm_std package, and must be kept in sync with it.
///
/// We copy these types instead of directly depending on them, because we require special versions of serde
/// inside the enclave, which are different from the versions that cosmwasm_std uses.
/// For some reason patching the dependencies didn't work, so we are forced to maintain this copy, for now :(
use std::fmt;

use serde::{Deserialize, Serialize};

use super::encoding::Binary;
use serde_json::Value;

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct HumanAddr(pub String);

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct CanonicalAddr(pub Binary);

impl HumanAddr {
    pub fn as_str(&self) -> &str {
        &self.0
    }
    pub fn len(&self) -> usize {
        self.0.len()
    }
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }
}

impl fmt::Display for HumanAddr {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", &self.0)
    }
}

impl From<&str> for HumanAddr {
    fn from(addr: &str) -> Self {
        HumanAddr(addr.to_string())
    }
}

impl From<&HumanAddr> for HumanAddr {
    fn from(addr: &HumanAddr) -> Self {
        HumanAddr(addr.0.to_string())
    }
}

impl CanonicalAddr {
    pub fn as_slice(&self) -> &[u8] {
        &self.0.as_slice()
    }
    pub fn len(&self) -> usize {
        self.0.len()
    }
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }
}

impl fmt::Display for CanonicalAddr {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        self.0.fmt(f)
    }
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct Env {
    pub block: BlockInfo,
    pub message: MessageInfo,
    pub contract: ContractInfo,
    pub contract_key: Option<String>,
    pub contract_code_hash: Option<String>,
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct BlockInfo {
    pub height: i64,
    // time is seconds since epoch begin (Jan. 1, 1970)
    pub time: i64,
    pub chain_id: String,
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct MessageInfo {
    pub sender: CanonicalAddr,
    // go likes to return null for empty array, make sure we can parse it (use option)
    pub sent_funds: Option<Vec<Coin>>,
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct ContractInfo {
    pub address: CanonicalAddr,
    // go likes to return null for empty array, make sure we can parse it (use option)
    pub balance: Option<Vec<Coin>>,
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct Coin {
    pub denom: String,
    pub amount: String,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(untagged)]
pub enum WasmOutput {
    ErrObject {
        #[serde(rename = "Err")]
        err: Value,
    },
    OkString {
        #[serde(rename = "Ok")]
        ok: String,
    },
    OkObject {
        #[serde(rename = "Ok")]
        ok: ContractResult,
    },
}

// This should be in correlation with cosmwasm-std/init_handle's InitResponse and HandleResponse
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct ContractResult {
    pub messages: Vec<CosmosMsg>,
    pub log: Vec<LogAttribute>,
    pub data: Option<Binary>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
// This should be in correlation with cosmwasm-std/init_handle's CosmosMsg
// See https://github.com/serde-rs/serde/issues/1296 why we cannot add De-Serialize trait bounds to T
pub enum CosmosMsg<T = CustomMsg>
where
    T: Clone + fmt::Debug + PartialEq,
{
    Bank(BankMsg),
    // by default we use RawMsg, but a contract can override that
    // to call into more app-specific code (whatever they define)
    Custom(T),
    Staking(StakingMsg),
    Wasm(WasmMsg),
}

/// Added this here for reflect tests....
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
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

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum BankMsg {
    // this moves tokens in the underlying sdk
    Send {
        from_address: HumanAddr,
        to_address: HumanAddr,
        amount: Vec<Coin>,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum StakingMsg {
    Delegate {
        // delegator is automatically set to address of the calling contract
        validator: HumanAddr,
        amount: Coin,
    },
    Undelegate {
        // delegator is automatically set to address of the calling contract
        validator: HumanAddr,
        amount: Coin,
    },
    Withdraw {
        // delegator is automatically set to address of the calling contract
        validator: HumanAddr,
        /// this is the "withdraw address", the one that should receive the rewards
        /// if None, then use delegator address
        recipient: Option<HumanAddr>,
    },
    Redelegate {
        // delegator is automatically set to address of the calling contract
        src_validator: HumanAddr,
        dst_validator: HumanAddr,
        amount: Coin,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum WasmMsg {
    /// this dispatches a call to another contract at a known address (with known ABI)
    Execute {
        contract_addr: HumanAddr,
        /// callback_code_hash is the hex encoded hash of the code. This is used by Secret Network to harden against replaying the contract
        /// It is used to bind the request to a destination contract in a stronger way than just the contract address which can be faked
        callback_code_hash: String,
        /// msg is the json-encoded HandleMsg struct (as raw Binary)
        msg: Binary,
        send: Vec<Coin>,
    },
    /// this instantiates a new contracts from previously uploaded wasm code
    Instantiate {
        code_id: u64,
        /// callback_code_hash is the hex encoded hash of the code. This is used by Secret Network to harden against replaying the contract
        /// It is used to bind the request to a destination contract in a stronger way than just the contract address which can be faked
        callback_code_hash: String,
        /// msg is the json-encoded InitMsg struct (as raw Binary)
        msg: Binary,
        send: Vec<Coin>,
        /// optional human-readable label for the contract
        label: Option<String>,
    },
}

impl<T: Clone + fmt::Debug + PartialEq> From<BankMsg> for CosmosMsg<T> {
    fn from(msg: BankMsg) -> Self {
        CosmosMsg::Bank(msg)
    }
}

#[cfg(feature = "staking")]
impl<T: Clone + fmt::Debug + PartialEq> From<StakingMsg> for CosmosMsg<T> {
    fn from(msg: StakingMsg) -> Self {
        CosmosMsg::Staking(msg)
    }
}

impl<T: Clone + fmt::Debug + PartialEq> From<WasmMsg> for CosmosMsg<T> {
    fn from(msg: WasmMsg) -> Self {
        CosmosMsg::Wasm(msg)
    }
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct LogAttribute {
    pub key: String,
    pub value: String,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum QueryResult {
    Ok(Binary),
    Err(String),
}

impl QueryResult {
    // unwrap will panic on err, or give us the real data useful for tests
    pub fn unwrap(self) -> Binary {
        match self {
            QueryResult::Err(msg) => panic!("Unexpected error: {}", msg),
            QueryResult::Ok(res) => res,
        }
    }

    pub fn is_err(&self) -> bool {
        match self {
            QueryResult::Err(_) => true,
            _ => false,
        }
    }
}

// coin is a shortcut constructor for a set of one denomination of coins
pub fn coin(amount: &str, denom: &str) -> Vec<Coin> {
    vec![Coin {
        amount: amount.to_string(),
        denom: denom.to_string(),
    }]
}

// log is shorthand to produce log messages
pub fn log(key: &str, value: &str) -> LogAttribute {
    LogAttribute {
        key: key.to_string(),
        value: value.to_string(),
    }
}
