use cosmwasm_std::{
    coins, entry_point, to_binary, BankMsg, Binary, CosmosMsg, Deps, DepsMut, Env, MessageInfo,
    QueryRequest, Reply, ReplyOn, Response, StdError, StdResult, SubMsg, SubMsgResult, WasmMsg,
    WasmQuery,
};

use crate::msg::{ExecuteMsg, InstantiateMsg, QueryMsg, QueryRes};
use crate::state::{count, count_read, expiration, expiration_read};

#[entry_point]
pub fn instantiate(
    deps: DepsMut,
    env: Env,
    _info: MessageInfo,
    msg: InstantiateMsg,
) -> StdResult<Response> {
    count(deps.storage).save(&msg.counter)?;
    let expires = env.block.height + msg.expires;
    expiration(deps.storage).save(&expires)?;

    let mut resp = Response::default();
    resp.data = Some(env.contract.address.as_str().as_bytes().into());
    Ok(resp)
}

#[entry_point]
pub fn execute(
    deps: DepsMut,
    env: Env,
    _info: MessageInfo,
    msg: ExecuteMsg,
) -> StdResult<Response> {
    match msg {
        ExecuteMsg::Increment { addition } => increment(deps, addition),
        ExecuteMsg::TransferMoney { amount } => transfer_money(deps, amount),
        ExecuteMsg::RecursiveReply {} => recursive_reply(env, deps),
        ExecuteMsg::RecursiveReplyFail {} => recursive_reply_fail(env, deps),
        ExecuteMsg::InitNewContract {} => init_new_contract(env, deps),
    }
}

pub fn increment(deps: DepsMut, c: u64) -> StdResult<Response> {
    if c == 0 {
        return Err(StdError::generic_err("got wrong counter"));
    }

    let new_count = count_read(deps.storage).load()? + c;
    count(deps.storage).save(&new_count)?;

    let mut resp = Response::default();
    resp.data = Some((new_count as u32).to_be_bytes().into());

    Ok(resp)
}

pub fn transfer_money(_deps: DepsMut, amount: u64) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1337,
        msg: CosmosMsg::Bank(BankMsg::Send {
            to_address: "secret105w4vl4gm7q00yg5jngewt5kp7aj0xjk7zrnhw".to_string(),
            amount: coins(amount as u128, "uscrt"),
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

pub fn recursive_reply(env: Env, _deps: DepsMut) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1304,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.into_string(),
            code_hash: env.contract.code_hash,
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

pub fn recursive_reply_fail(env: Env, _deps: DepsMut) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1305,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.into_string(),
            code_hash: env.contract.code_hash,
            msg: Binary::from("{\"increment\":{\"addition\":0}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

pub fn init_new_contract(env: Env, _deps: DepsMut) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1404,
        msg: CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_hash: env.contract.code_hash,
            msg: Binary::from("{\"counter\":150, \"expires\":100}".as_bytes().to_vec()),
            funds: vec![],
            label: "new202213".to_string(),
            code_id: 1,
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

#[entry_point]
pub fn query(deps: Deps, env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::Get {} => to_binary(&get(deps, env)?),
    }
}

#[entry_point]
pub fn reply(deps: DepsMut, env: Env, reply: Reply) -> StdResult<Response> {
    match (reply.id, reply.result) {
        (1337, SubMsgResult::Err(_)) => {
            let mut resp = Response::default();
            resp.data = Some(
                (count_read(deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }
        (1337, SubMsgResult::Ok(_)) => Err(StdError::generic_err("got wrong bank answer")),
        (1304, SubMsgResult::Err(e)) => Err(StdError::generic_err(format!(
            "recursive reply failed: {}",
            e
        ))),
        (1304, SubMsgResult::Ok(_)) => {
            let mut resp = Response::default();
            resp.data = Some(
                (count_read(deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }
        (1305, SubMsgResult::Ok(_)) => {
            Err(StdError::generic_err(format!("recursive reply failed")))
        }
        (1305, SubMsgResult::Err(_)) => {
            let mut resp = Response::default();
            let new_count = 10;
            count(deps.storage).save(&new_count)?;

            resp.data = Some(
                (count_read(deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }
        (1404, SubMsgResult::Err(e)) => Err(StdError::generic_err(format!(
            "recursive init failed: {}",
            e
        ))),
        (1404, SubMsgResult::Ok(s)) => match s.data {
            Some(x) => {
                let response = deps.querier.query(&QueryRequest::Wasm(WasmQuery::Smart {
                    callback_code_hash: env.contract.code_hash,
                    contract_addr: String::from_utf8(
                        Binary::from_base64(String::from_utf8(x.to_vec())?.as_str())?.to_vec(),
                    )?,
                    msg: to_binary(&QueryMsg::Get {})?,
                }))?;

                match response {
                    QueryRes::Get { count } => {
                        let mut resp = Response::default();
                        resp.data = Some((count as u32).to_be_bytes().into());
                        return Ok(resp);
                    }
                }
            }
            None => Err(StdError::generic_err(format!(
                "Init didn't response with contract address",
            ))),
        },

        _ => Err(StdError::generic_err("invalid reply id or result")),
    }
}

fn get(deps: Deps, env: Env) -> StdResult<QueryRes> {
    let count = count_read(deps.storage).load()?;
    let expiration = expiration_read(deps.storage).load()?;

    if env.block.height > expiration {
        return Ok(QueryRes::Get { count: 0 });
    }

    Ok(QueryRes::Get { count })
}
