use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::fmt;

use crate::addresses::HumanAddr;
use crate::coins::Coin;
use crate::encoding::Binary;
use crate::errors::StdResult;
use crate::math::Decimal;

pub type QueryResponse = Binary;

pub type QueryResult = StdResult<QueryResponse>;

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryRequest<T> {
    Bank(BankQuery),
    Custom(T),
    Staking(StakingQuery),
    Wasm(WasmQuery),
    Dist(DistQuery),
    Mint(MintQuery),
    Gov(GovQuery),
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
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

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum GovQuery {
    /// Returns all the currently active proposals. Might be useful to filter out invalid votes, and trigger
    /// in-contract voting periods
    Proposals {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum DistQuery {
    /// This calls into the native bank module for all denominations.
    /// Note that this may be much more expensive than Balance and should be avoided if possible.
    /// Return value is AllBalanceResponse.
    Rewards { delegator: HumanAddr },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum MintQuery {
    /// This calls into the native bank module for all denominations.
    /// Note that this may be much more expensive than Balance and should be avoided if possible.
    /// Return value is AllBalanceResponse.
    Inflation {},
    BondedRatio {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum WasmQuery {
    /// this queries the public API of another contract at a known address (with known ABI)
    /// return value is whatever the contract returns (caller should know)
    Smart {
        contract_addr: HumanAddr,
        /// callback_code_hash is the hex encoded hash of the code. This is used by Secret Network to harden against replaying the contract
        /// It is used to bind the request to a destination contract in a stronger way than just the contract address which can be faked
        callback_code_hash: String,
        /// msg is the json-encoded QueryMsg struct
        msg: Binary,
    },
    /// this queries the raw kv-store of the contract.
    /// returns the raw, unparsed data stored at that key (or `Ok(Err(StdError:NotFound{}))` if missing)
    Raw {
        contract_addr: HumanAddr,
        /// Key is the raw key used in the contracts Storage
        key: Binary,
        /// callback_code_hash is the hex encoded hash of the code. This is used by Secret Network to harden against replaying the contract
        /// It is used to bind the request to a destination contract in a stronger way than just the contract address which can be faked
        callback_code_hash: String,
    },
}

impl<T: Clone + fmt::Debug + PartialEq + JsonSchema> From<GovQuery> for QueryRequest<T> {
    fn from(msg: GovQuery) -> Self {
        QueryRequest::Gov(msg)
    }
}

impl<T: Clone + fmt::Debug + PartialEq + JsonSchema> From<MintQuery> for QueryRequest<T> {
    fn from(msg: MintQuery) -> Self {
        QueryRequest::Mint(msg)
    }
}

impl<T: Clone + fmt::Debug + PartialEq + JsonSchema> From<DistQuery> for QueryRequest<T> {
    fn from(msg: DistQuery) -> Self {
        QueryRequest::Dist(msg)
    }
}

impl<T: Clone + fmt::Debug + PartialEq + JsonSchema> From<BankQuery> for QueryRequest<T> {
    fn from(msg: BankQuery) -> Self {
        QueryRequest::Bank(msg)
    }
}

#[cfg(feature = "staking")]
impl<T: Clone + fmt::Debug + PartialEq + JsonSchema> From<StakingQuery> for QueryRequest<T> {
    fn from(msg: StakingQuery) -> Self {
        QueryRequest::Staking(msg)
    }
}

impl<T: Clone + fmt::Debug + PartialEq + JsonSchema> From<WasmQuery> for QueryRequest<T> {
    fn from(msg: WasmQuery) -> Self {
        QueryRequest::Wasm(msg)
    }
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub struct BalanceResponse {
    /// Always returns a Coin with the requested denom.
    /// This may be of 0 amount if no such funds.
    pub amount: Coin,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub struct AllBalanceResponse {
    /// Returns all non-zero coins held by this account.
    pub amount: Vec<Coin>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
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
    /// Returns all the unbonding delegations by the delegator
    UnbondingDelegations { delegator: HumanAddr },
}

/// ProposalsResponse is data format returned from GovQuery::Proposals query
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub struct ProposalsResponse {
    pub proposals: Vec<Proposal>,
}

/// ProposalsResponse is data format returned from GovQuery::Proposals query
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub struct Proposal {
    pub id: u64,
    /// Time of the block where MinDeposit was reached. -1 if MinDeposit is not reached
    pub voting_start_time: u64,
    /// Time that the VotingPeriod for this proposal will end and votes will be tallied
    pub voting_end_time: u64,
}

/// BondedDenomResponse is data format returned from StakingRequest::BondedDenom query
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub struct BondedDenomResponse {
    pub denom: String,
}

/// UnbondingDelegationsResponse is data format returned from StakingRequest::UnbondingDelegations query
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub struct UnbondingDelegationsResponse {
    pub delegations: Vec<Delegation>,
}

/// DelegationsResponse is data format returned from StakingRequest::AllDelegations query
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub struct AllDelegationsResponse {
    pub delegations: Vec<Delegation>,
}

/// Delegation is basic (cheap to query) data about a delegation
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
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

/// DelegationResponse is data format returned from StakingRequest::Delegation query
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub struct DelegationResponse {
    pub delegation: Option<FullDelegation>,
}

/// FullDelegation is all the info on the delegation, some (like accumulated_reward and can_redelegate)
/// is expensive to query
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
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

/// ValidatorsResponse is data format returned from StakingRequest::Validators query
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct ValidatorsResponse {
    pub validators: Vec<Validator>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct Validator {
    pub address: HumanAddr,
    pub commission: Decimal,
    pub max_commission: Decimal,
    /// TODO: what units are these (in terms of time)?
    pub max_change_rate: Decimal,
}

/// Rewards response
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct RewardsResponse {
    pub rewards: Vec<ValidatorRewards>,
    pub total: Vec<Coin>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct ValidatorRewards {
    pub validator_address: HumanAddr,
    pub reward: Vec<Coin>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct RewardCoin {
    pub coin: String,
    pub demon: String,
}

/// Inflation response
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct InflationResponse {
    pub inflation_rate: String,
}

/// Bonded Ratio response
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct BondedRatioResponse {
    pub bonded_ratio: String,
}
