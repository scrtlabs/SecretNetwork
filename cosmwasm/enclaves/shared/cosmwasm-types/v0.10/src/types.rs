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

pub use super::coins::Coin;
use super::encoding::Binary;

use crate::consts::BECH32_PREFIX_ACC_ADDR;

pub const CONTRACT_KEY_LENGTH: usize = 64;
pub const CONTRACT_KEY_PROOF_LENGTH: usize = 32;

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq, Eq)]
pub struct HumanAddr(pub String);

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq, Eq)]
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
        if canonical_addr.is_empty() {
            return Ok(HumanAddr::from(""));
        }

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
        self.0.as_slice()
    }
    pub fn len(&self) -> usize {
        self.0.len()
    }
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }
    pub fn from_human(human_addr: &HumanAddr) -> Result<Self, bech32::Error> {
        if human_addr.is_empty() {
            return Ok(CanonicalAddr(Binary(vec![])));
        }

        let (decoded_prefix, data) = bech32::decode(human_addr.as_str())?;
        let canonical = Vec::<u8>::from_base32(&data)?;

        Ok(CanonicalAddr(Binary(canonical)))
    }

    pub fn from_vec(vec: Vec<u8>) -> Self {
        Self(Binary(vec))
    }
}

impl fmt::Display for CanonicalAddr {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        self.0.fmt(f)
    }
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct ContractKey {
    #[serde(default)]
    pub og_contract_key: Option<Binary>,
    #[serde(default)]
    pub current_contract_key: Option<Binary>,
    #[serde(default)]
    pub current_contract_key_proof: Option<Binary>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct Env {
    pub block: BlockInfo,
    pub message: MessageInfo,
    pub contract: ContractInfo,
    pub contract_key: Option<ContractKey>,
    #[serde(default)]
    pub contract_code_hash: String,
    #[serde(default)]
    pub transaction: Option<TransactionInfo>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct TransactionInfo {
    /// The position of this transaction in the block. The first
    /// transaction has index 0.
    ///
    /// This allows you to get a unique transaction indentifier in this chain
    /// using the pair (`env.block.height`, `env.transaction.index`).
    ///
    pub index: u32,
    /// The hash of the current transaction bytes.
    /// aka txhash or transaction_id
    /// hash = sha256(tx_bytes)
    #[serde(default)]
    pub hash: String,
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct BlockInfo {
    pub height: u64,
    /// Absolute time of the block creation in nanoseconds since the UNIX epoch (00:00:00 on 1970-01-01 UTC).
    /// Note: when passed down to the enclave from Go this field is in nanoseconds, when passed down to a v0.10 contract this field is in seconds
    /// For more context: https://github.com/scrtlabs/SecretNetwork/pull/1331#discussion_r1113526524
    pub time: u64,
    pub chain_id: String,
    #[cfg(feature = "random")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub random: Option<Binary>,
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct MessageInfo {
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

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct ContractInfo {
    pub address: HumanAddr,
}

// This should be in correlation with cosmwasm-std/init_handle's InitResponse and HandleResponse
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ContractResult {
    pub messages: Vec<CosmosMsg>,
    pub log: Vec<LogAttribute>,
    pub data: Option<Binary>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
// This should be in correlation with cosmwasm-std/init_handle's CosmosMsg
// See https://github.com/serde-rs/serde/issues/1296 why we cannot add De-Serialize trait bounds to T
pub enum CosmosMsg<T = Empty>
where
    T: Clone + fmt::Debug + PartialEq,
{
    Bank(BankMsg),
    Custom(T),
    Staking(StakingMsg),
    Wasm(WasmMsg),
    Gov(GovMsg),
}

/// An empty struct that serves as a placeholder in different places,
/// such as contracts that don't set a custom message.
///
/// It is designed to be expressable in correct JSON and JSON Schema but
/// contains no meaningful data. Previously we used enums without cases,
/// but those cannot represented as valid JSON Schema (https://github.com/CosmWasm/cosmwasm/issues/451)
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct Empty {}

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
        /// callback_sig is used only inside the enclave to validate messages
        /// that are originating from other contracts
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
        /// callback_sig is used only inside the enclave to validate messages
        /// that are originating from other contracts
        callback_sig: Option<Vec<u8>>,
    },
    /// Migrates a given contracts to use new wasm code. Passes a MigrateMsg to allow us to
    /// customize behavior.
    ///
    /// Only the contract admin (as defined in wasmd), if any, is able to make this call.
    ///
    /// This is translated to a [MsgMigrateContract](https://github.com/CosmWasm/wasmd/blob/v0.14.0/x/wasm/internal/types/tx.proto#L86-L96).
    /// `sender` is automatically filled with the current contract's address.
    Migrate {
        contract_addr: String,
        /// callback_code_hash is the hex encoded hash of the **new** code. This is used by Secret Network to harden against replaying the contract
        /// It is used to bind the request to a destination contract in a stronger way than just the contract address which can be faked
        callback_code_hash: String,
        /// the code_id of the **new** logic to place in the given contract
        code_id: u64,
        /// msg is the json-encoded MigrateMsg struct that will be passed to the new code
        msg: Binary,
        /// callback_sig is used only inside the enclave to validate messages
        /// that are originating from other contracts
        callback_sig: Option<Vec<u8>>,
    },
    /// Sets a new admin (for migrate) on the given contract.
    /// Fails if this contract is not currently admin of the target contract.
    UpdateAdmin {
        contract_addr: String,
        admin: String,
        /// callback_sig is used only inside the enclave to validate messages
        /// that are originating from other contracts
        callback_sig: Option<Vec<u8>>,
    },
    /// Clears the admin on the given contract, so no more migration possible.
    /// Fails if this contract is not currently admin of the target contract.
    ClearAdmin {
        contract_addr: String,
        /// callback_sig is used only inside the enclave to validate messages
        /// that are originating from other contracts
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
