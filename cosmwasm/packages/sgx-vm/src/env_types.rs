use cosmwasm_std::{Coin, HumanAddr};
use serde::{Deserialize, Serialize};

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
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
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
