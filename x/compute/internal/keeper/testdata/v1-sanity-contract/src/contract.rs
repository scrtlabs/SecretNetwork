use cosmwasm_std::{
    entry_point, to_binary, BankMsg, Binary, CosmosMsg, Deps, DepsMut, Env, MessageInfo, Reply,
    ReplyOn, Response, StdError, StdResult, SubMsg, SubMsgResult,
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

    Ok(Response::default())
}

#[entry_point]
pub fn execute(
    deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    msg: ExecuteMsg,
) -> StdResult<Response> {
    match msg {
        ExecuteMsg::Increment { addition } => increment(deps, addition),
        ExecuteMsg::TransferMoney { amount } => transfer_money(deps, amount),
    }
}

pub fn increment(deps: DepsMut, c: u64) -> StdResult<Response> {
    let new_count = count_read(deps.storage).load()? + c;
    count(deps.storage).save(&new_count)?;

    let mut resp = Response::default();
    resp.data = Some((new_count as u32).to_be_bytes().into());
    Ok(resp)
}

pub fn transfer_money(deps: DepsMut, c: u64) -> StdResult<Response> {
    let new_count = count_read(deps.storage).load()? + c;
    count(deps.storage).save(&new_count)?;

    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1337,
        msg: CosmosMsg::Bank(BankMsg::Send {
            to_address: "asdasadasadadsaadasdasd".to_string(),
            amount: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    resp.data = Some((new_count as u32).to_be_bytes().into());
    Ok(resp)
}

#[entry_point]
pub fn query(deps: Deps, env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::Get {} => to_binary(&get(deps, env)?),
    }
}

#[entry_point]
pub fn reply(deps: DepsMut, _env: Env, reply: Reply) -> StdResult<Response> {
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
