use cosmwasm_std::{Binary, Coin};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum InstantiateMsg {
    Counter {
        counter: u64,
        expires: u64,
    },

    // These were ported from the v0.10 test-contract:
    Nop {},
    Callback {
        contract_addr: String,
        code_hash: String,
    },
    CallbackContractError {
        contract_addr: String,
        code_hash: String,
    },
    ContractError {
        error_type: String,
    },
    NoLogs {},
    CallbackToInit {
        code_id: u64,
        code_hash: String,
    },
    CallbackBadParams {
        contract_addr: String,
        code_hash: String,
    },
    Panic {},
    SendExternalQueryDepthCounter {
        to: String,
        depth: u8,
        code_hash: String,
    },
    SendExternalQueryRecursionLimit {
        to: String,
        depth: u8,
        code_hash: String,
    },
    CallToInit {
        code_id: u64,
        code_hash: String,
        label: String,
        msg: String,
    },
    CallToExec {
        addr: String,
        code_hash: String,
        msg: String,
    },
    CallToQuery {
        addr: String,
        code_hash: String,
        msg: String,
    },
    BankMsg {
        amount: Vec<Coin>,
        to: String,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum ExecuteMsg {
    Increment {
        addition: u64,
    },
    TransferMoney {
        amount: u64,
    },
    RecursiveReply {},
    RecursiveReplyFail {},
    InitNewContract {},
    InitNewContractWithError {},
    SubMsgLoop {
        iter: u64,
    },
    SubMsgLoopIner {
        iter: u64,
    },

    // These were ported from the v0.10 test-contract:
    A {
        contract_addr: String,
        code_hash: String,
        x: u8,
        y: u8,
    },
    B {
        contract_addr: String,
        code_hash: String,
        x: u8,
        y: u8,
    },
    C {
        x: u8,
        y: u8,
    },
    UnicodeData {},
    EmptyLogKeyValue {},
    EmptyData {},
    NoData {},
    ContractError {
        error_type: String,
    },
    NoLogs {},
    CallbackToInit {
        code_id: u64,
        code_hash: String,
    },
    CallbackContractError {
        contract_addr: String,
        code_hash: String,
    },
    CallbackBadParams {
        contract_addr: String,
        code_hash: String,
    },
    SetState {
        key: String,
        value: String,
    },
    GetState {
        key: String,
    },
    RemoveState {
        key: String,
    },
    TestCanonicalizeAddressErrors {},
    Panic {},
    AllocateOnHeap {
        bytes: u32,
    },
    PassNullPointerToImportsShouldThrow {
        pass_type: String,
    },
    SendExternalQuery {
        to: String,
        code_hash: String,
    },
    SendExternalQueryPanic {
        to: String,
        code_hash: String,
    },
    SendExternalQueryError {
        to: String,
        code_hash: String,
    },
    SendExternalQueryBadAbi {
        to: String,
        code_hash: String,
    },
    SendExternalQueryBadAbiReceiver {
        to: String,
        code_hash: String,
    },
    LogMsgSender {},
    CallbackToLogMsgSender {
        to: String,
        code_hash: String,
    },
    DepositToContract {},
    SendFunds {
        amount: u32,
        denom: String,
        to: String,
        from: String,
    },
    SendFundsToInitCallback {
        amount: u32,
        denom: String,
        code_id: u64,
        code_hash: String,
    },
    SendFundsToExecCallback {
        amount: u32,
        denom: String,
        to: String,
        code_hash: String,
    },
    Sleep {
        ms: u64,
    },
    SendExternalQueryDepthCounter {
        to: String,
        code_hash: String,
        depth: u8,
    },
    SendExternalQueryRecursionLimit {
        to: String,
        code_hash: String,
        depth: u8,
    },
    WithFloats {
        x: u8,
        y: u8,
    },
    CallToInit {
        code_id: u64,
        code_hash: String,
        label: String,
        msg: String,
    },
    CallToExec {
        addr: String,
        code_hash: String,
        msg: String,
    },
    CallToQuery {
        addr: String,
        code_hash: String,
        msg: String,
    },
    StoreReallyLongKey {},
    StoreReallyShortKey {},
    StoreReallyLongValue {},
    Secp256k1Verify {
        pubkey: Binary,
        sig: Binary,
        msg_hash: Binary,
        iterations: u32,
    },
    Secp256k1VerifyFromCrate {
        pubkey: Binary,
        sig: Binary,
        msg_hash: Binary,
        iterations: u32,
    },
    Ed25519Verify {
        pubkey: Binary,
        sig: Binary,
        msg: Binary,
        iterations: u32,
    },
    Ed25519BatchVerify {
        pubkeys: Vec<Binary>,
        sigs: Vec<Binary>,
        msgs: Vec<Binary>,
        iterations: u32,
    },
    Secp256k1RecoverPubkey {
        msg_hash: Binary,
        sig: Binary,
        recovery_param: u8,
        iterations: u32,
    },
    Secp256k1Sign {
        msg: Binary,
        privkey: Binary,
        iterations: u32,
    },
    Ed25519Sign {
        msg: Binary,
        privkey: Binary,
        iterations: u32,
    },
    BankMsg {
        amount: Vec<Coin>,
        to: String,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryMsg {
    Get {},

    // These were ported from the v0.10 test-contract:
    ContractError {
        error_type: String,
    },
    Panic {},
    ReceiveExternalQuery {
        num: u8,
    },
    SendExternalQueryInfiniteLoop {
        to: String,
        code_hash: String,
    },
    WriteToStorage {},
    RemoveFromStorage {},
    SendExternalQueryDepthCounter {
        to: String,
        depth: u8,
        code_hash: String,
    },
    SendExternalQueryRecursionLimit {
        to: String,
        depth: u8,
        code_hash: String,
    },
    CallToQuery {
        addr: String,
        code_hash: String,
        msg: String,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryRes {
    Get { count: u64 },
}
