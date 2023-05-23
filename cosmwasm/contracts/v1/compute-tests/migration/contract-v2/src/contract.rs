use crate::msg::{ExecuteMsg, InstantiateMsg, MigrateMsg};
use cosmwasm_std::{entry_point, DepsMut, Env, MessageInfo, Response, StdResult};

#[entry_point]
pub fn instantiate(
    _deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    _msg: InstantiateMsg,
) -> StdResult<Response> {
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
        ExecuteMsg::NewFunction {} => Ok(Response::default()),
    }
}

#[entry_point]
pub fn migrate(_deps: DepsMut, _env: Env, msg: MigrateMsg) -> StdResult<Response> {
    match msg {
        MigrateMsg::Migrate {} => Ok(Response::default()),
    }
}
