//! must keep this file in sync with cosmwasm/packages/std/src/query.rs

use serde::{Deserialize, Serialize};

use super::coins::Coin;
use super::encoding::Binary;
use super::math::Decimal;
use super::types::HumanAddr;

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum QueryRequest {
    Bank(BankQuery),
    Custom(serde_json::Value),
    Staking(StakingQuery),
    Wasm(WasmQuery),
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum BankQuery {
    /// This calls into the native bank module for one denomination
    /// Return value is BalanceResponse
    Balance { address: HumanAddr, denom: String },
    /// This calls into the native bank module for all denominations.
    /// Note that this may be much more expensive than Balance and should be avoided if possible.
    /// Return value is AllBalanceResponse.
    AllBalances { address: HumanAddr },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum WasmQuery {
    /// this queries the public API of another contract at a known address (with known ABI)
    /// return value is whatever the contract returns (caller should know)
    Smart {
        contract_addr: HumanAddr,
        /// This field is used to construct a callback message to another contract
        callback_code_hash: String,
        /// msg is the json-encoded QueryMsg struct
        msg: Binary,
    },
    /// this queries the raw kv-store of the contract.
    /// returns the raw, unparsed data stored at that key (or `Ok(Err(StdError:NotFound{}))` if missing)
    Raw {
        contract_addr: HumanAddr,
        /// This field is used to construct a callback message to another contract
        callback_code_hash: String,
        /// Key is the raw key used in the contracts Storage
        key: Binary,
    },
}

impl From<BankQuery> for QueryRequest {
    fn from(msg: BankQuery) -> Self {
        QueryRequest::Bank(msg)
    }
}

#[cfg(feature = "staking")]
impl From<StakingQuery> for QueryRequest {
    fn from(msg: StakingQuery) -> Self {
        QueryRequest::Staking(msg)
    }
}

impl From<WasmQuery> for QueryRequest {
    fn from(msg: WasmQuery) -> Self {
        QueryRequest::Wasm(msg)
    }
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum StakingQuery {
    /// Returns the denomination that can be bonded (if there are multiple native tokens on the chain)
    BondedDenom {},
    /// AllDelegations will return all delegations by the delegator
    AllDelegations { delegator: HumanAddr },
    /// Delegation will return more detailed info on a particular
    /// delegation, defined by delegator/validator pair
    Delegation {
        delegator: HumanAddr,
        validator: HumanAddr,
    },
    /// Returns all registered Validators on the system
    Validators {},
}

/// Delegation is basic (cheap to query) data about a delegation
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct Delegation {
    pub delegator: HumanAddr,
    pub validator: HumanAddr,
    /// How much we have locked in the delegation
    pub amount: Coin,
}

impl From<FullDelegation> for Delegation {
    fn from(full: FullDelegation) -> Self {
        Delegation {
            delegator: full.delegator,
            validator: full.validator,
            amount: full.amount,
        }
    }
}

/// FullDelegation is all the info on the delegation, some (like accumulated_reward and can_redelegate)
/// is expensive to query
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct FullDelegation {
    pub delegator: HumanAddr,
    pub validator: HumanAddr,
    /// How much we have locked in the delegation
    pub amount: Coin,
    /// can_redelegate captures how much can be immediately redelegated.
    /// 0 is no redelegation and can_redelegate == amount is redelegate all
    /// but there are many places between the two
    pub can_redelegate: Coin,
    /// How much we can currently withdraw
    pub accumulated_rewards: Coin,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct Validator {
    pub address: HumanAddr,
    pub commission: Decimal,
    pub max_commission: Decimal,
    /// TODO: what units are these (in terms of time)?
    pub max_change_rate: Decimal,
}
