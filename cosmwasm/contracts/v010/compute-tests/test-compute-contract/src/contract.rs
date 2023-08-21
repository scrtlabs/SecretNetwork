use cosmwasm_storage::{PrefixedStorage, ReadonlySingleton, Singleton};

use cosmwasm_std::{
    log, plaintext_log, to_binary, Api, BankMsg, Binary, CanonicalAddr, Coin, CosmosMsg, Empty,
    Env, Extern, HandleResponse, HandleResult, HumanAddr, InitResponse, InitResult, Querier,
    QueryRequest, QueryResult, ReadonlyStorage, StdError, StdResult, Storage, Uint128, WasmMsg,
    WasmQuery,
};
use secp256k1::Secp256k1;

/////////////////////////////// Messages ///////////////////////////////

use core::time;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::ptr::null;
use std::{mem, thread};

//// consts

const REALLY_LONG: &[u8] = b"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum InitMsg {
    WasmMsg {
        ty: String,
    },
    Nop {},
    Callback {
        contract_addr: HumanAddr,
        code_hash: String,
    },
    CallbackContractError {
        contract_addr: HumanAddr,
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
        contract_addr: HumanAddr,
        code_hash: String,
    },
    Panic {},
    SendExternalQueryDepthCounter {
        to: HumanAddr,
        depth: u8,
        code_hash: String,
    },
    SendExternalQueryRecursionLimit {
        to: HumanAddr,
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
        addr: HumanAddr,
        code_hash: String,
        msg: String,
    },
    CallToQuery {
        addr: HumanAddr,
        code_hash: String,
        msg: String,
    },
    InitFromV1 {
        counter: u64,
    },
    Counter {
        counter: u64,
    },
    AddAttributes {},
    AddAttributesWithSubmessage {},
    AddPlaintextAttributes {},
    AddPlaintextAttributesWithSubmessage {},
    AddMixedEventsAndAttributesFromV1 {
        addr: HumanAddr,
        code_hash: String,
    },
    BankMsgSend {
        amount: Vec<Coin>,
        to: HumanAddr,
        from: Option<HumanAddr>,
    },
    CosmosMsgCustom {},
    SendMultipleFundsToInitCallback {
        coins: Vec<Coin>,
        code_id: u64,
        code_hash: String,
    },
    SendMultipleFundsToExecCallback {
        coins: Vec<Coin>,
        to: HumanAddr,
        code_hash: String,
    },
    GetEnv {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    WasmMsg {
        ty: String,
    },
    A {
        contract_addr: HumanAddr,
        code_hash: String,
        x: u8,
        y: u8,
    },
    B {
        contract_addr: HumanAddr,
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
        contract_addr: HumanAddr,
        code_hash: String,
    },
    CallbackBadParams {
        contract_addr: HumanAddr,
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
        to: HumanAddr,
        code_hash: String,
    },
    SendExternalQueryPanic {
        to: HumanAddr,
        code_hash: String,
    },
    SendExternalQueryError {
        to: HumanAddr,
        code_hash: String,
    },
    SendExternalQueryBadAbi {
        to: HumanAddr,
        code_hash: String,
    },
    SendExternalQueryBadAbiReceiver {
        to: HumanAddr,
        code_hash: String,
    },
    LogMsgSender {},
    CallbackToLogMsgSender {
        to: HumanAddr,
        code_hash: String,
    },
    DepositToContract {},
    SendFunds {
        amount: u32,
        denom: String,
        to: HumanAddr,
        from: HumanAddr,
    },
    BankMsgSend {
        amount: Vec<Coin>,
        to: HumanAddr,
        from: Option<HumanAddr>,
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
        to: HumanAddr,
        code_hash: String,
    },
    SendMultipleFundsToExecCallback {
        coins: Vec<Coin>,
        to: HumanAddr,
        code_hash: String,
    },
    Sleep {
        ms: u64,
    },
    SendExternalQueryDepthCounter {
        to: HumanAddr,
        code_hash: String,
        depth: u8,
    },
    SendExternalQueryRecursionLimit {
        to: HumanAddr,
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
        addr: HumanAddr,
        code_hash: String,
        msg: String,
    },
    CallToQuery {
        addr: HumanAddr,
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
    ExecuteFromV1 {
        counter: u64,
    },
    IncrementFromV1 {
        addition: u64,
    },
    AddAttributes {},
    AddAttributesWithSubmessage {},
    AddMoreAttributes {},
    AddPlaintextAttributes {},
    AddPlaintextAttributesWithSubmessage {},
    AddMorePlaintextAttributes {},
    AddMixedEventsAndAttributesFromV1 {
        addr: HumanAddr,
        code_hash: String,
    },
    CosmosMsgCustom {},
    InitNewContract {},
    GetEnv {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryMsg {
    ContractError {
        error_type: String,
    },
    Panic {},
    ReceiveExternalQuery {
        num: u8,
    },
    SendExternalQueryInfiniteLoop {
        to: HumanAddr,
        code_hash: String,
    },
    WriteToStorage {},
    RemoveFromStorage {},
    SendExternalQueryDepthCounter {
        to: HumanAddr,
        depth: u8,
        code_hash: String,
    },
    SendExternalQueryRecursionLimit {
        to: HumanAddr,
        depth: u8,
        code_hash: String,
    },
    CallToQuery {
        addr: HumanAddr,
        code_hash: String,
        msg: String,
    },
    GetCountFromV1 {},
    Get {},
    GetContractVersion {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryRes {
    Get { count: u64 },
}

/////////////////////////////// Init ///////////////////////////////

pub fn init<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: InitMsg,
) -> InitResult {
    match msg {
        InitMsg::WasmMsg { ty } => {
            if ty == "success" {
                return Ok(InitResponse::default());
            } else if ty == "err" {
                return Err(StdError::generic_err("custom error"));
            } else if ty == "panic" {
                panic!()
            }

            return Err(StdError::generic_err("custom error"));
        }
        InitMsg::Nop {} => Ok(InitResponse {
            messages: vec![],
            log: vec![log("init", "🌈")],
        }),
        InitMsg::Callback {
            contract_addr,
            code_hash,
        } => Ok(init_with_callback(deps, env, contract_addr, code_hash)),
        InitMsg::ContractError { error_type } => Err(map_string_to_error(error_type)),
        InitMsg::NoLogs {} => Ok(InitResponse::default()),
        InitMsg::CallbackToInit { code_id, code_hash } => {
            Ok(init_callback_to_init(deps, env, code_id, code_hash))
        }
        InitMsg::CallbackContractError {
            contract_addr,
            code_hash,
        } => Ok(init_with_callback_contract_error(contract_addr, code_hash)),
        InitMsg::CallbackBadParams {
            contract_addr,
            code_hash,
        } => Ok(init_callback_bad_params(contract_addr, code_hash)),
        InitMsg::Panic {} => panic!("panic in init"),
        InitMsg::SendExternalQueryDepthCounter {
            to,
            depth,
            code_hash,
        } => Ok(InitResponse {
            messages: vec![],
            log: vec![log(
                format!(
                    "{}",
                    send_external_query_depth_counter(deps, to, depth, code_hash)
                ),
                "",
            )],
        }),
        InitMsg::SendExternalQueryRecursionLimit {
            to,
            depth,
            code_hash,
        } => Ok(InitResponse {
            messages: vec![],
            log: vec![log(
                "message",
                send_external_query_recursion_limit(deps, to, depth, code_hash)?,
            )],
        }),
        InitMsg::CallToInit {
            code_id,
            code_hash,
            label,
            msg,
        } => Ok(InitResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                callback_code_hash: code_hash,
                msg: Binary(msg.as_bytes().into()),
                send: vec![],
                label: label,
            })],
            log: vec![log("a", "a")],
        }),
        InitMsg::CallToExec {
            addr,
            code_hash,
            msg,
        } => Ok(InitResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: addr,
                callback_code_hash: code_hash,
                msg: Binary(msg.as_bytes().into()),
                send: vec![],
            })],
            log: vec![log("b", "b")],
        }),
        InitMsg::CallToQuery {
            addr,
            code_hash,
            msg,
        } => {
            let answer: u32 = deps
                .querier
                .query(&QueryRequest::Wasm(WasmQuery::Smart {
                    contract_addr: addr,
                    callback_code_hash: code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                }))
                .map_err(|err| {
                    StdError::generic_err(format!("Got an error from query: {:?}", err))
                })?;

            Ok(InitResponse {
                messages: vec![],
                log: vec![log("c", format!("{}", answer))],
            })
        }
        InitMsg::InitFromV1 { counter } => {
            count(&mut deps.storage).save(&counter)?;

            Ok(InitResponse {
                messages: vec![],
                log: vec![],
            })
        }
        InitMsg::Counter { counter } => {
            count(&mut deps.storage).save(&counter)?;

            Ok(InitResponse {
                messages: vec![],
                log: vec![],
            })
        }
        InitMsg::AddAttributes {} => Ok(InitResponse {
            messages: vec![],
            log: vec![log("attr1", "🦄"), log("attr2", "🌈")],
        }),
        InitMsg::AddAttributesWithSubmessage {} => Ok(InitResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: env.contract.address,
                callback_code_hash: env.contract_code_hash,
                msg: Binary::from(r#"{"add_more_attributes":{}}"#.as_bytes().to_vec()),
                send: vec![],
            })],
            log: vec![log("attr1", "🦄"), log("attr2", "🌈")],
        }),
        InitMsg::AddPlaintextAttributes {} => Ok(InitResponse {
            messages: vec![],
            log: vec![plaintext_log("attr1", "🦄"), plaintext_log("attr2", "🌈")],
        }),
        InitMsg::AddPlaintextAttributesWithSubmessage {} => Ok(InitResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: env.contract.address,
                callback_code_hash: env.contract_code_hash,
                msg: Binary::from(r#"{"add_more_plaintext_attributes":{}}"#.as_bytes().to_vec()),
                send: vec![],
            })],
            log: vec![plaintext_log("attr1", "🦄"), plaintext_log("attr2", "🌈")],
        }),
        InitMsg::AddMixedEventsAndAttributesFromV1 { addr, code_hash } => Ok(InitResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: addr,
                callback_code_hash: code_hash,
                msg: Binary::from(
                    r#"{"add_more_mixed_attributes_and_events":{}}"#.as_bytes().to_vec(),
                ),
                send: vec![],
            })],
            log: vec![plaintext_log("attr1", "🦄"), plaintext_log("attr2", "🌈")],
        }),
        InitMsg::BankMsgSend {
            to,
            amount: coins,
            from,
        } => Ok(InitResponse {
            messages: vec![CosmosMsg::Bank(BankMsg::Send {
                from_address: from.unwrap_or(env.contract.address),
                to_address: to,
                amount: coins,
            })],
            log: vec![],
        }),
        InitMsg::CosmosMsgCustom {} => Ok(InitResponse {
            messages: vec![CosmosMsg::Custom(Empty {})],
            log: vec![],
        }),
        InitMsg::SendMultipleFundsToExecCallback {
            coins,
            to,
            code_hash,
        } => Ok(InitResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: to,
                msg: Binary::from("{\"no_data\":{}}".as_bytes().to_vec()),
                callback_code_hash: code_hash,
                send: coins,
            })],
            log: vec![],
        }),
        InitMsg::SendMultipleFundsToInitCallback {
            coins,
            code_id,
            code_hash,
        } => Ok(InitResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                msg: Binary::from("{\"nop\":{}}".as_bytes().to_vec()),
                callback_code_hash: code_hash,
                send: coins,
                label: "test".to_string(),
            })],
            log: vec![],
        }),
        InitMsg::GetEnv {} => Ok(InitResponse {
            log: vec![log("env", serde_json_wasm::to_string(&env).unwrap())],
            messages: vec![],
        }),
    }
}

pub const COUNT_KEY: &[u8] = b"count";

pub fn count<S: Storage>(storage: &mut S) -> Singleton<S, u64> {
    Singleton::new(storage, COUNT_KEY)
}

pub fn count_read<S: Storage>(storage: &S) -> ReadonlySingleton<S, u64> {
    ReadonlySingleton::new(storage, COUNT_KEY)
}

fn map_string_to_error(error_type: String) -> StdError {
    let as_str: &str = &error_type[..];
    match as_str {
        "generic_err" => StdError::generic_err("la la 🤯"),
        "invalid_base64" => StdError::invalid_base64("ra ra 🤯"),
        "invalid_utf8" => StdError::invalid_utf8("ka ka 🤯"),
        "not_found" => StdError::not_found("za za 🤯"),
        "parse_err" => StdError::parse_err("na na 🤯", "pa pa 🤯"),
        "serialize_err" => StdError::serialize_err("ba ba 🤯", "ga ga 🤯"),
        "unauthorized" => StdError::unauthorized(),
        "underflow" => StdError::underflow("minuend 🤯", "subtrahend 🤯"),
        _ => StdError::generic_err("catch-all 🤯"),
    }
}

fn init_with_callback_contract_error(contract_addr: HumanAddr, code_hash: String) -> InitResponse {
    InitResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            callback_code_hash: code_hash,
            msg: Binary::from(r#"{"contract_error":{"error_type":"generic_err"}}"#.as_bytes()),
            send: vec![],
        })],
        log: vec![log("init with a callback with contract error", "🤷‍♀️")],
    }
}

fn init_callback_bad_params(contract_addr: HumanAddr, code_hash: String) -> InitResponse {
    InitResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            callback_code_hash: code_hash,
            msg: Binary::from(r#"{"c":{"x":"banana","y":3}}"#.as_bytes().to_vec()),
            send: vec![],
        })],
        log: vec![],
    }
}

fn init_with_callback<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    contract_addr: HumanAddr,
    code_hash: String,
) -> InitResponse {
    InitResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            callback_code_hash: code_hash,
            contract_addr: contract_addr.clone(),
            msg: Binary::from("{\"c\":{\"x\":0,\"y\":13}}".as_bytes().to_vec()),
            send: vec![],
        })],
        log: vec![log("init with a callback", "🦄")],
    }
}

pub fn init_callback_to_init<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    code_id: u64,
    code_hash: String,
) -> InitResponse {
    InitResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_id,
            msg: Binary::from("{\"nop\":{}}".as_bytes().to_vec()),
            callback_code_hash: code_hash,
            send: vec![],
            label: String::from("fi"),
        })],
        log: vec![log("instantiating a new contract from init!", "🐙")],
    }
}

/////////////////////////////// Handle ///////////////////////////////

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::WasmMsg { ty } => {
            if ty == "success" {
                return Ok(HandleResponse::default());
            } else if ty == "err" {
                return Err(StdError::generic_err("custom error"));
            } else if ty == "panic" {
                panic!()
            }

            return Err(StdError::generic_err("custom error"));
        }
        HandleMsg::A {
            contract_addr,
            code_hash,
            x,
            y,
        } => Ok(a(deps, env, contract_addr, code_hash, x, y)),
        HandleMsg::B {
            contract_addr,
            code_hash,
            x,
            y,
        } => Ok(b(deps, env, contract_addr, code_hash, x, y)),
        HandleMsg::C { x, y } => Ok(c(deps, env, x, y)),
        HandleMsg::UnicodeData {} => Ok(unicode_data(deps, env)),
        HandleMsg::EmptyLogKeyValue {} => Ok(empty_log_key_value(deps, env)),
        HandleMsg::EmptyData {} => Ok(empty_data(deps, env)),
        HandleMsg::NoData {} => Ok(no_data(deps, env)),
        HandleMsg::ContractError { error_type } => Err(map_string_to_error(error_type)),
        HandleMsg::NoLogs {} => Ok(HandleResponse::default()),
        HandleMsg::CallbackToInit { code_id, code_hash } => {
            Ok(exec_callback_to_init(deps, env, code_id, code_hash))
        }
        HandleMsg::CallbackBadParams {
            contract_addr,
            code_hash,
        } => Ok(exec_callback_bad_params(contract_addr, code_hash)),
        HandleMsg::CallbackContractError {
            contract_addr,
            code_hash,
        } => Ok(exec_with_callback_contract_error(contract_addr, code_hash)),
        HandleMsg::SetState { key, value } => Ok(set_state(deps, key, value)),
        HandleMsg::GetState { key } => Ok(get_state(deps, key)),
        HandleMsg::RemoveState { key } => Ok(remove_state(deps, key)),
        HandleMsg::TestCanonicalizeAddressErrors {} => test_canonicalize_address_errors(deps),
        HandleMsg::Panic {} => panic!("panic in exec"),
        HandleMsg::AllocateOnHeap { bytes } => Ok(allocate_on_heap(bytes as usize)),
        HandleMsg::PassNullPointerToImportsShouldThrow { pass_type } => {
            Ok(pass_null_pointer_to_imports_should_throw(deps, pass_type))
        }
        HandleMsg::SendExternalQuery { to, code_hash } => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(vec![send_external_query(deps, to, code_hash)].into()),
        }),
        HandleMsg::SendExternalQueryDepthCounter {
            to,
            code_hash,
            depth,
        } => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(
                vec![send_external_query_depth_counter(
                    deps, to, depth, code_hash,
                )]
                .into(),
            ),
        }),
        HandleMsg::SendExternalQueryRecursionLimit {
            to,
            code_hash,
            depth,
        } => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(to_binary(&send_external_query_recursion_limit(
                deps, to, depth, code_hash,
            )?)?),
        }),
        HandleMsg::SendExternalQueryPanic { to, code_hash } => {
            send_external_query_panic(deps, to, code_hash)
        }
        HandleMsg::SendExternalQueryError { to, code_hash } => {
            send_external_query_stderror(deps, to, code_hash)
        }
        HandleMsg::SendExternalQueryBadAbi { to, code_hash } => {
            send_external_query_bad_abi(deps, to, code_hash)
        }
        HandleMsg::SendExternalQueryBadAbiReceiver { to, code_hash } => {
            send_external_query_bad_abi_receiver(deps, to, code_hash)
        }
        HandleMsg::LogMsgSender {} => Ok(HandleResponse {
            messages: vec![],
            log: vec![log("msg.sender", env.message.sender.to_string())],
            data: None,
        }),
        HandleMsg::CallbackToLogMsgSender { to, code_hash } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: to.clone(),
                callback_code_hash: code_hash,
                msg: Binary::from(r#"{"log_msg_sender":{}}"#.as_bytes().to_vec()),
                send: vec![],
            })],
            log: vec![log("hi", "hey")],
            data: None,
        }),
        HandleMsg::DepositToContract {} => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(to_binary(&env.message.sent_funds).unwrap()),
        }),
        HandleMsg::SendFunds {
            amount,
            from,
            to,
            denom,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Bank(BankMsg::Send {
                from_address: from,
                to_address: to,
                amount: vec![Coin {
                    amount: Uint128(amount as u128),
                    denom: denom,
                }],
            })],
            log: vec![],
            data: None,
        }),
        HandleMsg::BankMsgSend { to, amount, from } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Bank(BankMsg::Send {
                from_address: from.unwrap_or(env.contract.address),
                to_address: to,
                amount,
            })],
            log: vec![],
            data: None,
        }),
        HandleMsg::CosmosMsgCustom {} => Ok(HandleResponse {
            messages: vec![CosmosMsg::Custom(Empty {})],
            log: vec![],
            data: None,
        }),
        HandleMsg::SendFundsToInitCallback {
            amount,
            denom,
            code_id,
            code_hash,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
                msg: Binary("{\"nop\":{}}".as_bytes().to_vec()),
                code_id: code_id,
                callback_code_hash: code_hash,
                label: String::from("yo"),
                send: vec![Coin {
                    amount: Uint128(amount as u128),
                    denom: denom,
                }],
            })],
            log: vec![],
            data: None,
        }),
        HandleMsg::SendFundsToExecCallback {
            amount,
            denom,
            to,
            code_hash,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                msg: Binary("{\"no_data\":{}}".as_bytes().to_vec()),
                contract_addr: to,
                callback_code_hash: code_hash,
                send: vec![Coin {
                    amount: Uint128(amount as u128),
                    denom: denom,
                }],
            })],
            log: vec![],
            data: None,
        }),
        HandleMsg::Sleep { ms } => {
            thread::sleep(time::Duration::from_millis(ms));

            Ok(HandleResponse {
                messages: vec![],
                log: vec![],
                data: None,
            })
        }
        HandleMsg::WithFloats { x, y } => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(use_floats(x, y)),
        }),
        HandleMsg::CallToInit {
            code_id,
            code_hash,
            label,
            msg,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                callback_code_hash: code_hash,
                msg: Binary(msg.as_bytes().into()),
                send: vec![],
                label: label,
            })],
            log: vec![log("a", "a")],
            data: None,
        }),
        HandleMsg::CallToExec {
            addr,
            code_hash,
            msg,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: addr,
                callback_code_hash: code_hash,
                msg: Binary(msg.as_bytes().into()),
                send: vec![],
            })],
            log: vec![log("b", "b")],
            data: None,
        }),
        HandleMsg::CallToQuery {
            addr,
            code_hash,
            msg,
        } => {
            let answer: u32 = deps
                .querier
                .query(&QueryRequest::Wasm(WasmQuery::Smart {
                    contract_addr: addr,
                    callback_code_hash: code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                }))
                .map_err(|err| {
                    StdError::generic_err(format!("Got an error from query: {:?}", err))
                })?;

            Ok(HandleResponse {
                messages: vec![],
                log: vec![log("c", format!("{}", answer))],
                data: None,
            })
        }
        HandleMsg::StoreReallyLongKey {} => {
            let mut store = PrefixedStorage::new(b"my_prefix", &mut deps.storage);
            store.set(REALLY_LONG, b"hello");
            Ok(HandleResponse::default())
        }
        HandleMsg::StoreReallyShortKey {} => {
            let mut store = PrefixedStorage::new(b"my_prefix", &mut deps.storage);
            store.set(b"a", b"hello");
            Ok(HandleResponse::default())
        }
        HandleMsg::StoreReallyLongValue {} => {
            let mut store = PrefixedStorage::new(b"my_prefix", &mut deps.storage);
            store.set(b"hello", REALLY_LONG);
            Ok(HandleResponse::default())
        }
        HandleMsg::Secp256k1Verify {
            pubkey,
            sig,
            msg_hash,
            iterations,
        } => {
            let mut res: HandleResult = Ok(HandleResponse {
                messages: vec![],
                log: vec![],
                data: None,
            });

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.secp256k1_verify(
                    msg_hash.as_slice(),
                    sig.as_slice(),
                    pubkey.as_slice(),
                ) {
                    Ok(result) => Ok(HandleResponse {
                        messages: vec![],
                        log: vec![log("result", format!("{}", result))],
                        data: None,
                    }),
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        HandleMsg::Secp256k1VerifyFromCrate {
            pubkey,
            sig,
            msg_hash,
            iterations,
        } => {
            let mut res: HandleResult = Ok(HandleResponse {
                messages: vec![],
                log: vec![],
                data: None,
            });

            // loop for benchmarking
            for _ in 0..iterations {
                let secp256k1_verifier = Secp256k1::verification_only();

                let secp256k1_signature =
                    secp256k1::Signature::from_compact(&sig.0).map_err(|err| {
                        StdError::generic_err(format!("Malformed signature: {:?}", err))
                    })?;
                let secp256k1_pubkey = secp256k1::PublicKey::from_slice(pubkey.0.as_slice())
                    .map_err(|err| StdError::generic_err(format!("Malformed pubkey: {:?}", err)))?;
                let secp256k1_msg =
                    secp256k1::Message::from_slice(&msg_hash.as_slice()).map_err(|err| {
                        StdError::generic_err(format!(
                            "Failed to create a secp256k1 message from signed_bytes: {:?}",
                            err
                        ))
                    })?;

                res = match secp256k1_verifier.verify(
                    &secp256k1_msg,
                    &secp256k1_signature,
                    &secp256k1_pubkey,
                ) {
                    Ok(()) => Ok(HandleResponse {
                        messages: vec![],
                        log: vec![log("result", "true")],
                        data: None,
                    }),
                    Err(_err) => Ok(HandleResponse {
                        messages: vec![],
                        log: vec![log("result", "false")],
                        data: None,
                    }),
                };
            }

            return res;
        }
        HandleMsg::Ed25519Verify {
            pubkey,
            sig,
            msg,
            iterations,
        } => {
            let mut res: HandleResult = Ok(HandleResponse {
                messages: vec![],
                log: vec![],
                data: None,
            });

            // loop for benchmarking
            for _ in 0..iterations {
                res =
                    match deps
                        .api
                        .ed25519_verify(msg.as_slice(), sig.as_slice(), pubkey.as_slice())
                    {
                        Ok(result) => Ok(HandleResponse {
                            messages: vec![],
                            log: vec![log("result", format!("{}", result))],
                            data: None,
                        }),
                        Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                    };
            }

            return res;
        }
        HandleMsg::Ed25519BatchVerify {
            pubkeys,
            sigs,
            msgs,
            iterations,
        } => {
            let mut res: HandleResult = Ok(HandleResponse {
                messages: vec![],
                log: vec![],
                data: None,
            });

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.ed25519_batch_verify(
                    msgs.iter()
                        .map(|m| m.as_slice())
                        .collect::<Vec<&[u8]>>()
                        .as_slice(),
                    sigs.iter()
                        .map(|s| s.as_slice())
                        .collect::<Vec<&[u8]>>()
                        .as_slice(),
                    pubkeys
                        .iter()
                        .map(|p| p.as_slice())
                        .collect::<Vec<&[u8]>>()
                        .as_slice(),
                ) {
                    Ok(result) => Ok(HandleResponse {
                        messages: vec![],
                        log: vec![log("result", format!("{}", result))],
                        data: None,
                    }),
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        HandleMsg::Secp256k1RecoverPubkey {
            msg_hash,
            sig,
            recovery_param,
            iterations,
        } => {
            let mut res: HandleResult = Ok(HandleResponse {
                messages: vec![],
                log: vec![],
                data: None,
            });

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.secp256k1_recover_pubkey(
                    msg_hash.as_slice(),
                    sig.as_slice(),
                    recovery_param,
                ) {
                    Ok(result) => Ok(HandleResponse {
                        messages: vec![],
                        log: vec![log("result", format!("{}", Binary(result).to_base64()))],
                        data: None,
                    }),
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        HandleMsg::Secp256k1Sign {
            msg,
            privkey,
            iterations,
        } => {
            let mut res: HandleResult = Ok(HandleResponse {
                messages: vec![],
                log: vec![],
                data: None,
            });

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.secp256k1_sign(msg.as_slice(), privkey.as_slice()) {
                    Ok(result) => Ok(HandleResponse {
                        messages: vec![],
                        log: vec![log("result", format!("{}", Binary(result).to_base64()))],
                        data: None,
                    }),
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        HandleMsg::Ed25519Sign {
            msg,
            privkey,
            iterations,
        } => {
            let mut res: HandleResult = Ok(HandleResponse {
                messages: vec![],
                log: vec![],
                data: None,
            });

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.ed25519_sign(msg.as_slice(), privkey.as_slice()) {
                    Ok(result) => Ok(HandleResponse {
                        messages: vec![],
                        log: vec![log("result", format!("{}", Binary(result).to_base64()))],
                        data: None,
                    }),
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        HandleMsg::ExecuteFromV1 { counter } => {
            count(&mut deps.storage).save(&counter)?;

            let mut resp = HandleResponse::default();
            resp.data = Some(
                (count_read(&deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }
        HandleMsg::IncrementFromV1 { addition } => {
            if addition == 0 {
                return Err(StdError::generic_err("got wrong counter"));
            }

            let new_count = count(&mut deps.storage).load()? + addition;
            count(&mut deps.storage).save(&new_count)?;

            let mut resp = HandleResponse::default();
            resp.data = Some((new_count as u32).to_be_bytes().into());

            Ok(resp)
        }
        HandleMsg::AddAttributes {} => Ok(HandleResponse {
            messages: vec![],
            log: vec![log("attr1", "🦄"), log("attr2", "🌈")],
            data: None,
        }),
        HandleMsg::AddMoreAttributes {} => Ok(HandleResponse {
            messages: vec![],
            log: vec![log("attr3", "🍉"), log("attr4", "🥝")],
            data: None,
        }),
        HandleMsg::AddAttributesWithSubmessage {} => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: env.contract.address,
                callback_code_hash: env.contract_code_hash,
                msg: Binary::from(r#"{"add_more_attributes":{}}"#.as_bytes().to_vec()),
                send: vec![],
            })],
            log: vec![log("attr1", "🦄"), log("attr2", "🌈")],
            data: None,
        }),
        HandleMsg::AddPlaintextAttributes {} => Ok(HandleResponse {
            messages: vec![],
            log: vec![plaintext_log("attr1", "🦄"), plaintext_log("attr2", "🌈")],
            data: None,
        }),
        HandleMsg::AddMorePlaintextAttributes {} => Ok(HandleResponse {
            messages: vec![],
            log: vec![plaintext_log("attr3", "🍉"), plaintext_log("attr4", "🥝")],
            data: None,
        }),
        HandleMsg::AddPlaintextAttributesWithSubmessage {} => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: env.contract.address,
                callback_code_hash: env.contract_code_hash,
                msg: Binary::from(r#"{"add_more_plaintext_attributes":{}}"#.as_bytes().to_vec()),
                send: vec![],
            })],
            log: vec![plaintext_log("attr1", "🦄"), plaintext_log("attr2", "🌈")],
            data: None,
        }),
        HandleMsg::AddMixedEventsAndAttributesFromV1 { addr, code_hash } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: addr,
                callback_code_hash: code_hash,
                msg: Binary::from(
                    r#"{"add_more_mixed_attributes_and_events":{}}"#.as_bytes().to_vec(),
                ),
                send: vec![],
            })],
            log: vec![plaintext_log("attr1", "🦄"), plaintext_log("attr2", "🌈")],
            data: None,
        }),
        HandleMsg::InitNewContract {} => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id: 1,
                msg: Binary::from(
                    "{\"counter\":{\"counter\":150, \"expires\":100}}"
                        .as_bytes()
                        .to_vec(),
                ),
                callback_code_hash: env.contract_code_hash,
                send: vec![],
                label: String::from("fi"),
            })],
            log: vec![],
            data: None,
        }),
        HandleMsg::SendMultipleFundsToExecCallback {
            coins,
            to,
            code_hash,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: to,
                msg: Binary::from("{\"no_data\":{}}".as_bytes().to_vec()),
                callback_code_hash: code_hash,
                send: coins,
            })],
            log: vec![],
            data: None,
        }),
        HandleMsg::SendMultipleFundsToInitCallback {
            coins,
            code_id,
            code_hash,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                msg: Binary::from("{\"nop\":{}}".as_bytes().to_vec()),
                callback_code_hash: code_hash,
                send: coins,
                label: "test".to_string(),
            })],
            log: vec![],
            data: None,
        }),
        HandleMsg::GetEnv {} => Ok(HandleResponse {
            log: vec![log("env", serde_json_wasm::to_string(&env).unwrap())],
            data: None,
            messages: vec![],
        }),
    }
}

#[cfg(feature = "with_floats")]
fn use_floats(x: u8, y: u8) -> Binary {
    let res: f64 = (x as f64) / (y as f64);
    to_binary(&format!("{}", res)).unwrap()
}

#[cfg(not(feature = "with_floats"))]
fn use_floats(x: u8, y: u8) -> Binary {
    Binary(vec![x, y])
}

fn send_external_query<S: Storage, A: Api, Q: Querier>(
    deps: &Extern<S, A, Q>,
    contract_addr: HumanAddr,
    code_hash: String,
) -> u8 {
    let answer: u8 = deps
        .querier
        .query(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            callback_code_hash: code_hash,
            msg: Binary::from(r#"{"receive_external_query":{"num":2}}"#.as_bytes().to_vec()),
        }))
        .unwrap();
    answer
}

fn send_external_query_depth_counter<S: Storage, A: Api, Q: Querier>(
    deps: &Extern<S, A, Q>,
    contract_addr: HumanAddr,
    depth: u8,
    code_hash: String,
) -> u8 {
    if depth == 0 {
        return 0;
    }

    let answer: u8 = deps
        .querier
        .query(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr: contract_addr.clone(),
            callback_code_hash: code_hash.clone(),
            msg: Binary(
                format!(
                    r#"{{"send_external_query_depth_counter":{{"to":"{}","code_hash":"{}","depth":{}}}}}"#,
                    contract_addr.clone().to_string(),
                    code_hash.clone().to_string(),
                    depth - 1
                )
                .into(),
            ),
        }))
        .unwrap();

    answer + 1
}

fn send_external_query_recursion_limit<S: Storage, A: Api, Q: Querier>(
    deps: &Extern<S, A, Q>,
    contract_addr: HumanAddr,
    depth: u8,
    code_hash: String,
) -> StdResult<String> {
    let result = deps
        .querier
        .query(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr: contract_addr.clone(),
            callback_code_hash: code_hash.clone(),
            msg: Binary(
                format!(
                    r#"{{"send_external_query_recursion_limit":{{"to":"{}","code_hash":"{}","depth":{}}}}}"#,
                    contract_addr.clone().to_string(),
                    code_hash.clone().to_string(),
                    depth + 1
                )
                .into_bytes(),
            ),
        }));

    // 10 is the current recursion limit.
    if depth != 10 {
        result
    } else {
        match result {
            Err(StdError::GenericErr { msg, .. })
                if msg == "Querier system error: Query recursion limit exceeded" =>
            {
                Ok(String::from("Recursion limit was correctly enforced"))
            }
            _ => Err(StdError::generic_err(
                "Recursion limit was bypassed! this is a bug!",
            )),
        }
    }
}

fn send_external_query_panic<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    contract_addr: HumanAddr,
    code_hash: String,
) -> HandleResult {
    let err = deps
        .querier
        .query::<u8>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            msg: Binary::from(r#"{"panic":{}}"#.as_bytes().to_vec()),
            callback_code_hash: code_hash,
        }))
        .unwrap_err();

    Err(err)
}

fn send_external_query_stderror<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    contract_addr: HumanAddr,
    code_hash: String,
) -> HandleResult {
    let answer = deps
        .querier
        .query::<Binary>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            msg: Binary::from(
                r#"{"contract_error":{"error_type":"generic_err"}}"#
                    .as_bytes()
                    .to_vec(),
            ),
            callback_code_hash: code_hash,
        }));

    match answer {
        Ok(wtf) => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(wtf),
        }),
        Err(e) => Err(e),
    }
}

fn send_external_query_bad_abi<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    contract_addr: HumanAddr,
    code_hash: String,
) -> HandleResult {
    let answer = deps
        .querier
        .query::<Binary>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            callback_code_hash: code_hash,
            msg: Binary::from(
                r#""contract_error":{"error_type":"generic_err"}}"#.as_bytes().to_vec(),
            ),
        }));

    match answer {
        Ok(wtf) => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(wtf),
        }),
        Err(e) => Err(e),
    }
}

fn send_external_query_bad_abi_receiver<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    contract_addr: HumanAddr,
    code_hash: String,
) -> HandleResult {
    let answer = deps
        .querier
        .query::<String>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            msg: Binary::from(r#"{"receive_external_query":{"num":25}}"#.as_bytes().to_vec()),
            callback_code_hash: code_hash,
        }));

    match answer {
        Ok(wtf) => Ok(HandleResponse {
            messages: vec![],
            log: vec![log("wtf", wtf)],
            data: None,
        }),
        Err(e) => Err(e),
    }
}

fn exec_callback_bad_params(contract_addr: HumanAddr, code_hash: String) -> HandleResponse {
    HandleResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            callback_code_hash: code_hash,
            msg: Binary::from(r#"{"c":{"x":"banana","y":3}}"#.as_bytes().to_vec()),
            send: vec![],
        })],
        log: vec![],
        data: None,
    }
}

pub fn a<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    contract_addr: HumanAddr,
    code_hash: String,
    x: u8,
    y: u8,
) -> HandleResponse {
    HandleResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            callback_code_hash: code_hash.clone(),
            msg: Binary::from(format!(
                "{{\"b\":{{\"x\":{} ,\"y\": {},\"contract_addr\": \"{}\",\"code_hash\": \"{}\" }}}}",
                x,
                y,
                contract_addr.as_str(),
                &code_hash
            )
                .as_bytes()
                .to_vec()),
            send: vec![],
        })],
        log: vec![log("banana", "🍌")],
        data: Some(Binary(vec![x, y])),
    }
}

pub fn b<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    contract_addr: HumanAddr,
    code_hash: String,
    x: u8,
    y: u8,
) -> HandleResponse {
    HandleResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            callback_code_hash: code_hash,
            msg: Binary::from(
                format!("{{\"c\":{{\"x\":{} ,\"y\": {} }}}}", x + 1, y + 1)
                    .as_bytes()
                    .to_vec(),
            ),
            send: vec![],
        })],
        log: vec![log("kiwi", "🥝")],
        data: Some(Binary(vec![x + y])),
    }
}

pub fn c<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    x: u8,
    y: u8,
) -> HandleResponse {
    HandleResponse {
        messages: vec![],
        log: vec![log("watermelon", "🍉")],
        data: Some(Binary(vec![x + y])),
    }
}

pub fn empty_log_key_value<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
) -> HandleResponse {
    HandleResponse {
        messages: vec![],
        log: vec![log("my value is empty", ""), log("", "my key is empty")],
        data: None,
    }
}

pub fn empty_data<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
) -> HandleResponse {
    HandleResponse {
        messages: vec![],
        log: vec![],
        data: Some(Binary(vec![])),
    }
}

pub fn unicode_data<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
) -> HandleResponse {
    HandleResponse {
        messages: vec![],
        log: vec![],
        data: Some(Binary("🍆🥑🍄".as_bytes().to_vec())),
    }
}

pub fn no_data<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
) -> HandleResponse {
    HandleResponse {
        messages: vec![],
        log: vec![],
        data: None,
    }
}

pub fn exec_callback_to_init<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    code_id: u64,
    code_hash: String,
) -> HandleResponse {
    HandleResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_id,
            msg: Binary::from("{\"nop\":{}}".as_bytes().to_vec()),
            callback_code_hash: code_hash,
            send: vec![],
            label: String::from("hi"),
        })],
        log: vec![log("instantiating a new contract", "🪂")],
        data: None,
    }
}

fn exec_with_callback_contract_error(
    contract_addr: HumanAddr,
    code_hash: String,
) -> HandleResponse {
    HandleResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            callback_code_hash: code_hash,
            msg: Binary::from(
                r#"{"contract_error":{"error_type":"generic_err"}}"#
                    .as_bytes()
                    .to_vec(),
            ),
            send: vec![],
        })],
        log: vec![log("exec with a callback with contract error", "🤷‍♂️")],
        data: None,
    }
}

fn allocate_on_heap(bytes: usize) -> HandleResponse {
    let mut values: Box<Vec<u8>> = Box::new(vec![0; bytes]);

    values[bytes - 1] = 1;

    HandleResponse {
        data: Some(Binary("😅".as_bytes().to_vec())),
        log: vec![log("zero", format!("{}", values[bytes / 2]))],
        messages: vec![],
    }
}

fn get_state<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    key: String,
) -> HandleResponse {
    let store = PrefixedStorage::new(b"my_prefix", &mut deps.storage);

    match store.get(key.as_bytes()) {
        Some(value) => HandleResponse {
            data: Some(Binary(value)),
            log: vec![],
            messages: vec![],
        },
        None => HandleResponse::default(),
    }
}

fn set_state<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    key: String,
    value: String,
) -> HandleResponse {
    let mut store = PrefixedStorage::new(b"my_prefix", &mut deps.storage);
    store.set(key.as_bytes(), value.as_bytes());
    HandleResponse::default()
}

fn remove_state<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    key: String,
) -> HandleResponse {
    let mut store = PrefixedStorage::new(b"my_prefix", &mut deps.storage);
    store.remove(key.as_bytes());
    HandleResponse::default()
}

#[allow(invalid_value)]
#[allow(unused_must_use)]
fn pass_null_pointer_to_imports_should_throw<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    pass_type: String,
) -> HandleResponse {
    let null_ptr_slice: &[u8] = unsafe { std::slice::from_raw_parts(std::ptr::null(), 0) };

    let null_ptr: *const HumanAddr = std::ptr::null();
    let null_human_addr: &HumanAddr = unsafe { &*null_ptr };

    let null_ptr: *const CanonicalAddr = std::ptr::null();
    let null_canon_addr: &CanonicalAddr = unsafe { &*null_ptr };

    use std::ptr;

    use std::ptr;

    match &pass_type[..] {
        "read_db_key" => {
            unsafe { deps.storage.get(null_ptr_slice) };
        }
        "write_db_key" => {
            deps.storage.set(null_ptr_slice, b"write value");
        }
        "write_db_value" => {
            deps.storage.set(b"write key", null_ptr_slice);
        }
        "remove_db_key" => {
            deps.storage.remove(null_ptr_slice);
        }
        "canonicalize_address_input" => {
            deps.api.canonical_address(null_human_addr);
        }
        "canonicalize_address_output" => { /* TODO */ }
        "humanize_address_input" => {
            deps.api.human_address(null_canon_addr);
        }
        "humanize_address_output" => { /* TODO */ }
        _ => {}
    };

    HandleResponse::default()
}

fn test_canonicalize_address_errors<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
) -> HandleResult {
    match deps.api.canonical_address(&HumanAddr(String::from(""))) {
        Err(StdError::GenericErr { msg, backtrace: _ }) => {
            if msg != String::from("canonicalize_address errored: input is empty") {
                return Err(StdError::generic_err(
                    "empty address should have failed with 'canonicalize_address errored: input is empty'",
                ));
            }
            // all is good, continue
        }
        _ => return Err(StdError::generic_err(
            "empty address should have failed with 'canonicalize_address errored: input is empty'",
        )),
    }

    match deps.api.canonical_address(&HumanAddr(String::from("   "))) {
        Err(StdError::GenericErr { msg, backtrace: _ }) => {
            if msg != String::from("canonicalize_address errored: input is empty") {
                return Err(StdError::generic_err(
                    "empty trimmed address should have failed with 'canonicalize_address errored: input is empty'",
                ));
            }
            // all is good, continue
        }
        _ => {
            return Err(StdError::generic_err(
                "empty trimmed address should have failed with 'canonicalize_address errored: input is empty'",
            ))
        }
    }

    match deps
        .api
        .canonical_address(&HumanAddr(String::from("cosmos1h99hrcc54ms9lxxxx")))
    {
        Err(StdError::GenericErr { msg, backtrace: _ }) => {
            if msg != String::from("canonicalize_address errored: invalid checksum") {
                return Err(StdError::generic_err(
                    "bad bech32 should have failed with 'canonicalize_address errored: invalid checksum'",
                ));
            }
            // all is good, continue
        }
        _ => return Err(StdError::generic_err(
            "bad bech32 should have failed with 'canonicalize_address errored: invalid checksum'",
        )),
    }

    match deps.api.canonical_address(&HumanAddr(String::from(
        "cosmos1h99hrcc54ms9luwpex9kw0rwdt7etvfdyxh6gu",
    ))) {
        Err(StdError::GenericErr { msg, backtrace: _ }) => {
            if msg != String::from("canonicalize_address errored: wrong address prefix: \"cosmos\"")
            {
                return Err(StdError::generic_err(
                    "bad prefix should have failed with 'canonicalize_address errored: wrong address prefix: \"cosmos\"'",
                ));
            }
            // all is good, continue
        }
        _ => {
            return Err(StdError::generic_err(
                "bad prefix should have failed with 'canonicalize_address errored: wrong address prefix: \"cosmos\"'",
            ))
        }
    }

    Ok(HandleResponse {
        data: Some(Binary("🤟".as_bytes().to_vec())),
        log: vec![],
        messages: vec![],
    })
}

/////////////////////////////// Query ///////////////////////////////

pub fn query<S: Storage, A: Api, Q: Querier>(
    deps: &Extern<S, A, Q>,
    _msg: QueryMsg,
) -> QueryResult {
    match _msg {
        QueryMsg::ContractError { error_type } => Err(map_string_to_error(error_type)),
        QueryMsg::Panic {} => panic!("panic in query"),
        QueryMsg::ReceiveExternalQuery { num } => {
            Ok(Binary(serde_json_wasm::to_vec(&(num + 1)).unwrap()))
        }
        QueryMsg::SendExternalQueryInfiniteLoop { to, code_hash } => {
            send_external_query_infinite_loop(deps, to, code_hash)
        }
        QueryMsg::WriteToStorage {} => write_to_storage_in_query(deps),
        QueryMsg::RemoveFromStorage {} => remove_from_storage_in_query(deps),
        QueryMsg::SendExternalQueryDepthCounter {
            to,
            depth,
            code_hash,
        } => Ok(to_binary(&send_external_query_depth_counter(
            deps, to, depth, code_hash,
        ))
        .unwrap()),
        QueryMsg::SendExternalQueryRecursionLimit {
            to,
            depth,
            code_hash,
        } => to_binary(&send_external_query_recursion_limit(
            deps, to, depth, code_hash,
        )?),
        QueryMsg::CallToQuery {
            addr,
            code_hash,
            msg,
        } => {
            let answer: u32 = deps
                .querier
                .query(&QueryRequest::Wasm(WasmQuery::Smart {
                    contract_addr: addr,
                    callback_code_hash: code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                }))
                .map_err(|err| {
                    StdError::generic_err(format!("Got an error from query: {:?}", err))
                })?;
            return Ok(to_binary(&answer)?);
        }
        QueryMsg::GetCountFromV1 {} => {
            let count = count_read(&deps.storage).load()?;

            Ok(to_binary(&QueryRes::Get { count })?)
        }
        QueryMsg::Get {} => {
            let count = count_read(&deps.storage).load()?;

            Ok(to_binary(&QueryRes::Get { count })?)
        }
        QueryMsg::GetContractVersion {} => {
            let answer: u8 = 10;
            return Ok(to_binary(&answer)?);
        }
    }
}

fn send_external_query_infinite_loop<S: Storage, A: Api, Q: Querier>(
    deps: &Extern<S, A, Q>,
    contract_addr: HumanAddr,
    code_hash: String,
) -> QueryResult {
    let answer = deps
        .querier
        .query::<Binary>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr: contract_addr.clone(),
            callback_code_hash: code_hash.clone(),
            msg: Binary::from(
                format!(
                    r#"{{"send_external_query_infinite_loop":{{"to":"{}", "code_hash":"{}"}}}}"#,
                    contract_addr.clone().to_string(),
                    &code_hash
                )
                .as_bytes()
                .to_vec(),
            ),
        }));

    match answer {
        Ok(wtf) => Ok(Binary(wtf.into())),
        Err(e) => Err(e),
    }
}

fn write_to_storage_in_query<S: Storage, A: Api, Q: Querier>(
    deps: &Extern<S, A, Q>,
) -> StdResult<Binary> {
    #[allow(clippy::cast_ref_to_mut)]
    let deps = unsafe { &mut *(deps as *const _ as *mut Extern<S, A, Q>) };
    deps.storage.set(b"abcd", b"dcba");

    Ok(Binary(vec![]))
}

fn remove_from_storage_in_query<S: Storage, A: Api, Q: Querier>(
    deps: &Extern<S, A, Q>,
) -> StdResult<Binary> {
    #[allow(clippy::cast_ref_to_mut)]
    let deps = unsafe { &mut *(deps as *const _ as *mut Extern<S, A, Q>) };
    deps.storage.remove(b"abcd");

    Ok(Binary(vec![]))
}
