use cosmwasm_std::{Binary, Coin};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum InstantiateMsg {
    WasmMsg {
        ty: String,
    },
    Counter {
        counter: u64,
        expires: u64,
    },
    AddAttributes {},
    AddAttributesWithSubmessage {
        id: u64,
    },
    AddPlaintextAttributes {},
    AddPlaintextAttributesWithSubmessage {
        id: u64,
    },
    AddEvents {},
    AddEventsWithSubmessage {
        id: u64,
    },
    AddMixedAttributesAndEvents {},
    AddMixedAttributesAndEventsWithSubmessage {
        id: u64,
    },
    MeasureGasForSubmessage {
        id: u64,
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
    BankMsgSend {
        amount: Vec<Coin>,
        to: String,
    },
    BankMsgBurn {
        amount: Vec<Coin>,
    },
    CosmosMsgCustom {},
    SendMultipleFundsToInitCallback {
        coins: Vec<Coin>,
        code_id: u64,
        code_hash: String,
    },
    SendMultipleFundsToExecCallback {
        coins: Vec<Coin>,
        to: String,
        code_hash: String,
    },
    GetEnv {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum ExecuteMsg {
    WasmMsg {
        ty: String,
    },
    Increment {
        addition: u64,
    },
    SendFundsWithErrorWithReply {},
    SendFundsWithReply {},
    AddAttributes {},
    AddAttributesWithSubmessage {
        id: u64,
    },
    AddMoreAttributes {},
    AddPlaintextAttributes {},
    AddPlaintextAttributesWithSubmessage {
        id: u64,
    },
    AddMorePlaintextAttributes {},
    AddEvents {},
    AddMoreEvents {},
    AddEventsWithSubmessage {
        id: u64,
    },
    AddMixedAttributesAndEvents {},
    AddMixedAttributesAndEventsWithSubmessage {
        id: u64,
    },
    AddMoreMixedAttributesAndEvents {},
    AddAttributesFromV010 {
        addr: String,
        code_hash: String,
        id: u64,
    },
    GasMeter {},
    GasMeterProxy {},
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
    MultipleSubMessages {},
    MultipleSubMessagesNoReply {},
    QuickError {},
    MultipleSubMessagesNoReplyWithError {},
    MultipleSubMessagesNoReplyWithPanic {},
    MultipleSubMessagesWithReplyWithError {},
    MultipleSubMessagesWithReplyWithPanic {},
    InitV10 {
        code_id: u64,
        code_hash: String,
        counter: u64,
    },
    ExecV10 {
        address: String,
        code_hash: String,
    },
    InitV10NoReply {
        code_id: u64,
        code_hash: String,
        counter: u64,
    },
    ExecV10NoReply {
        address: String,
        code_hash: String,
    },
    QueryV10 {
        address: String,
        code_hash: String,
    },
    InitV10WithError {
        code_id: u64,
        code_hash: String,
    },
    ExecV10WithError {
        address: String,
        code_hash: String,
    },
    InitV10NoReplyWithError {
        code_id: u64,
        code_hash: String,
    },
    ExecV10NoReplyWithError {
        address: String,
        code_hash: String,
    },
    QueryV10WithError {
        address: String,
        code_hash: String,
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
    SendMultipleFundsToInitCallback {
        coins: Vec<Coin>,
        code_id: u64,
        code_hash: String,
    },
    SendFundsToExecCallback {
        amount: u32,
        denom: String,
        to: String,
        code_hash: String,
    },
    SendMultipleFundsToExecCallback {
        coins: Vec<Coin>,
        to: String,
        code_hash: String,
    },
    ValidateAddress {
        addr: String,
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
    BankMsgSend {
        amount: Vec<Coin>,
        to: String,
    },
    BankMsgBurn {
        amount: Vec<Coin>,
    },
    CosmosMsgCustom {},
    GetEnv {},
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
    ReceiveExternalQueryV1 {
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
    GetContractVersion {},
    GetEnv {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryRes {
    Get { count: u64 },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum ExternalMessages {
    GetCountFromV1 {},
    QueryFromV1WithError {},
}
