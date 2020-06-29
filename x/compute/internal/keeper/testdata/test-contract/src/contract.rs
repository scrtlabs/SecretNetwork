use cosmwasm_storage::PrefixedStorage;

use cosmwasm_std::{
    generic_err, invalid_base64, invalid_utf8, log, not_found, null_pointer, parse_err,
    serialize_err, to_binary, unauthorized, underflow, Api, Binary, CosmosMsg, Env, Extern,
    HandleResponse, HandleResult, HumanAddr, InitResponse, InitResult, MigrateResponse, Querier,
    QueryResult, StdError, StdResult, Storage, WasmMsg,
};

use crate::state::config_read;

/////////////////////////////// Messages ///////////////////////////////

use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum InitMsg {
    Nop {},
    Callback { contract_addr: HumanAddr },
    ContractError { error_type: String },
    State {},
    NoLogs {},
    CallbackToInit { code_id: u64 },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum HandleMsg {
    A {
        contract_addr: HumanAddr,
        x: u8,
        y: u8,
    },
    B {
        contract_addr: HumanAddr,
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
    ContractError {},
    NoLogs {},
    CallbackToInit {
        code_id: u64,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum QueryMsg {
    Owner {},
    ContractError {},
}

// We define a custom struct for each query response
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct OwnerResponse {
    pub owner: HumanAddr,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum MigrateMsg {}

/////////////////////////////// Init ///////////////////////////////

pub fn init<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: InitMsg,
) -> InitResult {
    match msg {
        InitMsg::Nop {} => Ok(InitResponse {
            messages: vec![],
            log: vec![log("init", "🌈")],
        }),
        InitMsg::Callback { contract_addr } => Ok(init_with_callback(deps, env, contract_addr)),
        InitMsg::ContractError { error_type } => Err(init_with_error(error_type)),
        InitMsg::State {} => Ok(init_state(deps, env)),
        InitMsg::NoLogs {} => Ok(InitResponse::default()),
        InitMsg::CallbackToInit { code_id } => Ok(init_callback_to_init(deps, env, code_id)),
    }
}

fn init_with_error(error_type: String) -> StdError {
    let as_str: &str = &error_type[..];
    match as_str {
        "generic_err" => generic_err("la la 🤯"),
        "invalid_base64" => invalid_base64("ra ra 🤯"),
        "invalid_utf8" => invalid_utf8("ka ka 🤯"),
        "not_found" => not_found("za za 🤯"),
        "null_pointer" => null_pointer(),
        "parse_err" => parse_err("na na 🤯", "pa pa 🤯"),
        "serialize_err" => serialize_err("ba ba 🤯", "ga ga 🤯"),
        "unauthorized" => unauthorized(),
        "underflow" => underflow("minuend 🤯", "subtrahend 🤯"),
        _ => generic_err("catch-all 🤯"),
    }
}

fn init_state<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    _env: Env,
) -> InitResponse {
    let _store = PrefixedStorage::new(b"prefix", &mut deps.storage);

    InitResponse::default()
}

fn init_with_callback<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    contract_addr: HumanAddr,
) -> InitResponse {
    InitResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            msg: Binary("{\"c\":{\"x\":0,\"y\":13}}".as_bytes().to_vec()),
            send: vec![],
        })],
        log: vec![log("init with a callback", "🦄")],
    }
}

pub fn init_callback_to_init<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    code_id: u64,
) -> InitResponse {
    InitResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_id: code_id,
            msg: Binary("{\"nop\":{}}".as_bytes().to_vec()),
            send: vec![],
            label: None,
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
        HandleMsg::A {
            contract_addr,
            x,
            y,
        } => Ok(a(deps, env, contract_addr, x, y)),
        HandleMsg::B {
            contract_addr,
            x,
            y,
        } => Ok(b(deps, env, contract_addr, x, y)),
        HandleMsg::C { x, y } => Ok(c(deps, env, x, y)),
        HandleMsg::UnicodeData {} => Ok(unicode_data(deps, env)),
        HandleMsg::EmptyLogKeyValue {} => Ok(empty_log_key_value(deps, env)),
        HandleMsg::EmptyData {} => Ok(empty_data(deps, env)),
        HandleMsg::NoData {} => Ok(no_data(deps, env)),
        HandleMsg::ContractError {} => Err(generic_err("Test error! 🌈")),
        HandleMsg::NoLogs {} => Ok(HandleResponse::default()),
        HandleMsg::CallbackToInit { code_id } => Ok(exec_callback_to_init(deps, env, code_id)),
    }
}

pub fn a<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    contract_addr: HumanAddr,
    x: u8,
    y: u8,
) -> HandleResponse {
    HandleResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            msg: Binary(
                format!(
                    "{{\"b\":{{\"x\":{} ,\"y\": {},\"contract_addr\": \"{}\" }}}}",
                    x,
                    y,
                    contract_addr.as_str()
                )
                .as_bytes()
                .to_vec(),
            ),
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
    x: u8,
    y: u8,
) -> HandleResponse {
    HandleResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            msg: Binary(
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
) -> HandleResponse {
    HandleResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_id: code_id,
            msg: Binary("{\"nop\":{}}".as_bytes().to_vec()),
            send: vec![],
            label: None,
        })],
        log: vec![log("instantiating a new contract", "🪂")],
        data: None,
    }
}

/////////////////////////////// Query ///////////////////////////////

pub fn query<S: Storage, A: Api, Q: Querier>(
    _deps: &Extern<S, A, Q>,
    _msg: QueryMsg,
) -> QueryResult {
    match _msg {
        QueryMsg::Owner {} => query_owner(_deps),
        QueryMsg::ContractError {} => query_contract_error(),
    }
}

fn query_owner<S: Storage, A: Api, Q: Querier>(deps: &Extern<S, A, Q>) -> StdResult<Binary> {
    let state = config_read(&deps.storage).load()?;

    let resp = OwnerResponse {
        owner: deps.api.human_address(&state.owner)?,
    };
    to_binary(&resp)
}

fn query_contract_error() -> QueryResult {
    Err(generic_err("Test error! 🌈"))
}

/////////////////////////////// Migrate ///////////////////////////////

pub fn migrate<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: MigrateMsg,
) -> StdResult<MigrateResponse> {
    Ok(MigrateResponse::default())
}
