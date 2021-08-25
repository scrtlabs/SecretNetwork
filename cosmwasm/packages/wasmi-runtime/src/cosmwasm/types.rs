//! must keep this file in sync with cosmwasm/packages/std/src/types.rs and cosmwasm/packages/std/src/init_handle.rs

#![allow(unused)]

/// These types are are copied over from the cosmwasm_std package, and must be kept in sync with it.
///
/// We copy these types instead of directly depending on them, because we require special versions of serde
/// inside the enclave, which are different from the versions that cosmwasm_std uses.
/// For some reason patching the dependencies didn't work, so we are forced to maintain this copy, for now :(
use std::fmt;

use log::*;

use bech32::{FromBase32, ToBase32};
use enclave_ffi_types::EnclaveError;
use serde::{Deserialize, Serialize};
use serde_json::Value;

pub use super::coins::Coin;
use super::encoding::Binary;

use super::addresses::Addr;
use crate::consts::BECH32_PREFIX_ACC_ADDR;
use crate::crypto::multisig::MultisigThresholdPubKey;
use crate::crypto::secp256k1::Secp256k1PubKey;
use crate::crypto::traits::PubKey;
use crate::crypto::CryptoError;

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
    pub fn from_canonical(canonical_addr: &CanonicalAddr) -> Result<Self, bech32::Error> {
        let human_addr_str = bech32::encode(
            BECH32_PREFIX_ACC_ADDR,
            canonical_addr.as_slice().to_base32(),
        )?;

        Ok(HumanAddr(human_addr_str))
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
    pub fn from_human(human_addr: &HumanAddr) -> Result<Self, bech32::Error> {
        let (decoded_prefix, data) = bech32::decode(human_addr.as_str())?;
        let canonical = Vec::<u8>::from_base32(&data)?;

        Ok(CanonicalAddr(Binary(canonical)))
    }
}

impl fmt::Display for CanonicalAddr {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        self.0.fmt(f)
    }
}

/// EnvV010 is Env that's used by cosmwasm v0.10 contracts
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct EnvV010 {
    pub block: BlockInfoV010,
    pub message: MessageInfoV010,
    pub contract: ContractInfoV010,
    pub contract_key: Option<String>,
    #[serde(default)]
    pub contract_code_hash: String,
}

/// BlockInfoV010 is BlockInfo that's used by cosmwasm v0.10 contracts
#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct BlockInfoV010 {
    pub height: u64,
    // time is seconds since epoch begin (Jan. 1, 1970)
    pub time: u64,
    pub chain_id: String,
}

/// MessageInfoV010 is MessageInfo that's used by cosmwasm v0.10 contracts
#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct MessageInfoV010 {
    /// The `sender` field from the wasm/MsgStoreCode, wasm/MsgInstantiateContract or wasm/MsgExecuteContract message.
    /// You can think of this as the address that initiated the action (i.e. the message). What that
    /// means exactly heavily depends on the application.
    ///
    /// The x/wasm module ensures that the sender address signed the transaction.
    /// Additional signers of the transaction that are either needed for other messages or contain unnecessary
    /// signatures are not propagated into the contract.
    ///
    /// There is a discussion to open up this field to multiple initiators, which you're welcome to join
    /// if you have a specific need for that feature: https://github.com/CosmWasm/cosmwasm/issues/293
    pub sender: HumanAddr,
    pub sent_funds: Vec<Coin>,
}

/// ContractInfoV010 is ContractInfo that's used by cosmwasm v0.10 contracts
#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct ContractInfoV010 {
    pub address: HumanAddr,
}

/// EnvV016 is Env that's used by cosmwasm v0.16 contracts
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct EnvV016 {
    pub block: BlockInfoV016,
    pub contract: ContractInfoV016,
}

/// BlockInfoV016 is BlockInfo that's used by cosmwasm v0.16 contracts
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct BlockInfoV016 {
    /// The height of a block is the number of blocks preceding it in the blockchain.
    pub height: u64,
    /// Absolute time of the block creation in seconds since the UNIX epoch (00:00:00 on 1970-01-01 UTC).
    ///
    /// The source of this is the [BFT Time in Tendermint](https://docs.tendermint.com/master/spec/consensus/bft-time.html),
    /// which has the same nanosecond precision as the `Timestamp` type.
    ///
    /// # Examples
    ///
    /// Using chrono:
    ///
    /// ```
    /// # use cosmwasm_std::{Addr, BlockInfo, ContractInfo, Env, MessageInfo, Timestamp};
    /// # let env = Env {
    /// #     block: BlockInfo {
    /// #         height: 12_345,
    /// #         time: Timestamp::from_nanos(1_571_797_419_879_305_533),
    /// #         chain_id: "cosmos-testnet-14002".to_string(),
    /// #     },
    /// #     contract: ContractInfo {
    /// #         address: Addr::unchecked("contract"),
    /// #     },
    /// # };
    /// # extern crate chrono;
    /// use chrono::NaiveDateTime;
    /// let seconds = env.block.time.seconds();
    /// let nsecs = env.block.time.subsec_nanos();
    /// let dt = NaiveDateTime::from_timestamp(seconds as i64, nsecs as u32);
    /// ```
    ///
    /// Creating a simple millisecond-precision timestamp (as used in JavaScript):
    ///
    /// ```
    /// # use cosmwasm_std::{Addr, BlockInfo, ContractInfo, Env, MessageInfo, Timestamp};
    /// # let env = Env {
    /// #     block: BlockInfo {
    /// #         height: 12_345,
    /// #         time: Timestamp::from_nanos(1_571_797_419_879_305_533),
    /// #         chain_id: "cosmos-testnet-14002".to_string(),
    /// #     },
    /// #     contract: ContractInfo {
    /// #         address: Addr::unchecked("contract"),
    /// #     },
    /// # };
    /// let millis = env.block.time.nanos() / 1_000_000;
    /// ```
    pub time: Timestamp,
    pub chain_id: String,
}

/// MessageInfoV016 is MessageInfo that's used by cosmwasm v0.16 contracts
/// Additional information from [MsgInstantiateContract] and [MsgExecuteContract], which is passed
/// along with the contract execution message into the `instantiate` and `execute` entry points.
///
/// It contains the essential info for authorization - identity of the call, and payment.
///
/// [MsgInstantiateContract]: https://github.com/CosmWasm/wasmd/blob/v0.15.0/x/wasm/internal/types/tx.proto#L47-L61
/// [MsgExecuteContract]: https://github.com/CosmWasm/wasmd/blob/v0.15.0/x/wasm/internal/types/tx.proto#L68-L78
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct MessageInfoV016 {
    /// The `sender` field from `MsgInstantiateContract` and `MsgExecuteContract`.
    /// You can think of this as the address that initiated the action (i.e. the message). What that
    /// means exactly heavily depends on the application.
    ///
    /// The x/compute module ensures that the sender address signed the transaction or
    /// is otherwise authorized to send the message.
    ///
    /// Additional signers of the transaction that are either needed for other messages or contain unnecessary
    /// signatures are not propagated into the contract.
    pub sender: Addr,
    /// The funds that are sent to the contract as part of `MsgInstantiateContract`
    /// or `MsgExecuteContract`. The transfer is processed in bank before the contract
    /// is executed such that the new balance is visible during contract execution.
    pub funds: Vec<Coin>,
}

/// ContractInfoV016 is ContractInfo that's used by cosmwasm v0.16 contracts
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct ContractInfoV016 {
    pub address: Addr,
    #[serde(default)]
    pub code_hash: String,
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
    Gov(GovMsg),
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
pub enum GovMsg {
    // Let contract vote on a governance proposal
    Vote {
        proposal: u64,
        vote_option: VoteOption,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub enum VoteOption {
    Yes,
    No,
    Abstain,
    NoWithVeto,
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
        callback_sig: Option<Vec<u8>>,
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
        /// Human-readable label for the contract
        #[serde(default)]
        label: String,
        callback_sig: Option<Vec<u8>>,
    },
}

impl<T: Clone + fmt::Debug + PartialEq> From<GovMsg> for CosmosMsg<T> {
    fn from(msg: GovMsg) -> Self {
        CosmosMsg::Gov(msg)
    }
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

/// Return true
///
/// Only used for serde annotations
fn bool_true() -> bool {
    true
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct LogAttribute {
    pub key: String,
    pub value: String,
    /// nonstandard late addition, thus optional and only used in deserialization.
    /// The contracts may return this in newer versions that support distinguishing
    /// encrypted and plaintext logs. We naturally default to encrypted logs, and
    /// don't serialize the field later so it doesn't leak up to the Go layers.
    #[serde(default = "bool_true")]
    #[serde(skip_serializing)]
    pub encrypted: bool,
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
        matches!(self, QueryResult::Err(_))
    }
}

/// A shorthand to produce a log attribute
pub fn log<K: ToString, V: ToString>(key: K, value: V) -> LogAttribute {
    LogAttribute {
        key: key.to_string(),
        value: value.to_string(),
        encrypted: true,
    }
}

/// A shorthand to produce a plaintext log attribute
pub fn plaintext_log<K: ToString, V: ToString>(key: K, value: V) -> LogAttribute {
    LogAttribute {
        key: key.to_string(),
        value: value.to_string(),
        encrypted: false,
    }
}
