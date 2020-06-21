use cosmwasm_std::{
    generic_err, log, Api, Binary, CosmosMsg, Env, Extern, HandleResponse, HandleResult, HumanAddr,
    InitResponse, InitResult, MigrateResponse, Querier, QueryResult, StdResult, Storage, WasmMsg,
};

use crate::msg::{HandleMsg, InitMsg, MigrateMsg, QueryMsg};

pub fn init<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: InitMsg,
) -> InitResult {
    match msg {
        InitMsg::Nop {} => init_nop(deps, env),
        InitMsg::Callback { contract_addr } => init_with_callback(deps, env, contract_addr),
        InitMsg::Error {} => init_error(deps, env),
    }
}

fn init_nop<S: Storage, A: Api, Q: Querier>(_deps: &mut Extern<S, A, Q>, _env: Env) -> InitResult {
    Ok(InitResponse {
        messages: vec![],
        log: vec![log("init", "🌈")],
    })
}
fn init_error<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
) -> InitResult {
    Err(generic_err("Test error! 🌈"))
}

fn init_with_callback<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    contract_addr: HumanAddr,
) -> InitResult {
    Ok(InitResponse {
        messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            msg: Binary("{{\"c\":{{\"x\":0,\"y\":13}}}}".as_bytes().to_vec()),
            send: vec![],
        })],
        log: vec![log("init with a callback", "🦄")],
    })
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: HandleMsg,
) -> HandleResult {
    Ok(match msg {
        HandleMsg::A {
            contract_addr,
            x,
            y,
        } => a(deps, env, contract_addr, x, y),
        HandleMsg::B {
            contract_addr,
            x,
            y,
        } => b(deps, env, contract_addr, x, y),
        HandleMsg::C { x, y } => c(deps, env, x, y),
        HandleMsg::UnicodeData {} => unicode_data(deps, env),
        HandleMsg::EmptyLogKeyValue {} => empty_log_key_value(deps, env),
        HandleMsg::EmptyData {} => empty_data(deps, env),
        HandleMsg::NoData {} => no_data(deps, env),
    })
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

pub fn query<S: Storage, A: Api, Q: Querier>(
    _deps: &Extern<S, A, Q>,
    _msg: QueryMsg,
) -> QueryResult {
    Ok(Binary(vec![]))
    // match msg {
    //     QueryMsg::Owner {} => query_owner(deps),
    // }
}

// fn query_owner<S: Storage, A: Api,Q:Querier>(deps: &Extern<S, A,Q>) -> QueryResponse {
//     let state = config_read(&deps.storage).load()?;

//     let resp = OwnerResponse {
//         owner: deps.api.human_address(&state.owner)?,
//     };
//     to_vec(&resp).context(SerializeErr {
//         kind: "OwnerResponse",
//     })
// }

pub fn migrate<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: MigrateMsg,
) -> StdResult<MigrateResponse> {
    Ok(MigrateResponse::default())
}
