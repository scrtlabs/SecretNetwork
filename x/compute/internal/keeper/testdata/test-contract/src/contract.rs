use snafu::ResultExt;

use cosmwasm::encoding::Binary;
use cosmwasm::errors::{contract_err, unauthorized, Result, SerializeErr};
use cosmwasm::serde::to_vec;
use cosmwasm::traits::{Api, Extern, Storage};
use cosmwasm::types::{log, CosmosMsg, Env, HumanAddr, Response};

use crate::msg::{HandleMsg, InitMsg, OwnerResponse, QueryMsg};
use crate::state::{config, config_read, State};

pub fn init<S: Storage, A: Api>(
    deps: &mut Extern<S, A>,
    env: Env,
    msg: InitMsg,
) -> Result<Response> {
    match msg {
        InitMsg::Nop {} => try_init_nop(deps, env),
        InitMsg::Callback { contract_addr } => try_init_with_callback(deps, env, contract_addr),
    }
}

fn try_init_nop<S: Storage, A: Api>(deps: &mut Extern<S, A>, env: Env) -> Result<Response> {
    Ok(Response {
        messages: vec![],
        log: vec![log("init", "🌈")],
        data: None,
    })
}

fn try_init_with_callback<S: Storage, A: Api>(
    deps: &mut Extern<S, A>,
    env: Env,
    contract_addr: HumanAddr,
) -> Result<Response> {
    Ok(Response {
        messages: vec![CosmosMsg::Contract {
            contract_addr: contract_addr.clone(),
            msg: Binary("{{\"c\":{{\"x\":0,\"y\":13}}}}".as_bytes().to_vec()),
            send: None,
        }],
        log: vec![log("init with a callback", "🦄")],
        data: None,
    })
}

pub fn handle<S: Storage, A: Api>(
    deps: &mut Extern<S, A>,
    env: Env,
    msg: HandleMsg,
) -> Result<Response> {
    match msg {
        HandleMsg::A {
            contract_addr,
            x,
            y,
        } => try_a(deps, env, contract_addr, x, y),
        HandleMsg::B {
            contract_addr,
            x,
            y,
        } => try_b(deps, env, contract_addr, x, y),
        HandleMsg::C { x, y } => try_c(deps, env, x, y),
        HandleMsg::EmptyLogKeyValue {} => try_empty_log_key_value(deps, env),
        HandleMsg::EmptyData {} => try_empty_data(deps, env),
        HandleMsg::NoData {} => try_no_data(deps, env),
    }
}

pub fn try_a<S: Storage, A: Api>(
    deps: &mut Extern<S, A>,
    env: Env,
    contract_addr: HumanAddr,
    x: u8,
    y: u8,
) -> Result<Response> {
    let res = Response {
        messages: vec![CosmosMsg::Contract {
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
            send: None,
        }],
        log: vec![log("banana", "🍌")],
        data: Some(Binary(vec![x, y])),
    };
    Ok(res)
}

pub fn try_b<S: Storage, A: Api>(
    deps: &mut Extern<S, A>,
    env: Env,
    contract_addr: HumanAddr,
    x: u8,
    y: u8,
) -> Result<Response> {
    let res = Response {
        messages: vec![CosmosMsg::Contract {
            contract_addr: contract_addr.clone(),
            msg: Binary(
                format!("{{\"c\":{{\"x\":{} ,\"y\": {} }}}}", x + 1, y + 1)
                    .as_bytes()
                    .to_vec(),
            ),
            send: None,
        }],
        log: vec![log("kiwi", "🥝")],
        data: Some(Binary(vec![x + y])),
    };
    Ok(res)
}

pub fn try_c<S: Storage, A: Api>(
    deps: &mut Extern<S, A>,
    env: Env,
    x: u8,
    y: u8,
) -> Result<Response> {
    let res = Response {
        messages: vec![],
        log: vec![log("watermelon", "🍉")],
        data: Some(Binary(vec![x + y])),
    };
    Ok(res)
}

pub fn try_empty_log_key_value<S: Storage, A: Api>(
    deps: &mut Extern<S, A>,
    env: Env,
) -> Result<Response> {
    Ok(Response {
        messages: vec![],
        log: vec![log("my value is empty", ""), log("", "my key is empty")],
        data: None,
    })
}

pub fn try_empty_data<S: Storage, A: Api>(deps: &mut Extern<S, A>, env: Env) -> Result<Response> {
    Ok(Response {
        messages: vec![],
        log: vec![],
        data: Some(Binary(vec![])),
    })
}

pub fn try_no_data<S: Storage, A: Api>(deps: &mut Extern<S, A>, env: Env) -> Result<Response> {
    Ok(Response {
        messages: vec![],
        log: vec![],
        data: None,
    })
}

pub fn query<S: Storage, A: Api>(deps: &Extern<S, A>, msg: QueryMsg) -> Result<Vec<u8>> {
    match msg {
        QueryMsg::Owner {} => query_owner(deps),
    }
}

fn query_owner<S: Storage, A: Api>(deps: &Extern<S, A>) -> Result<Vec<u8>> {
    let state = config_read(&deps.storage).load()?;

    let resp = OwnerResponse {
        owner: deps.api.human_address(&state.owner)?,
    };
    to_vec(&resp).context(SerializeErr {
        kind: "OwnerResponse",
    })
}
