use crate::msg::{ExecuteMsg, InstantiateMsg};
use cosmwasm_std::{entry_point, DepsMut, Env, MessageInfo, Response, StdResult};

#[entry_point]
pub fn instantiate(
    deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    _msg: InstantiateMsg,
) -> StdResult<Response> {
    write_to_storage(deps)?;
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
        ExecuteMsg::Test {} => Ok(Response::default()),
    }
}

#[allow(dead_code)]
pub fn write_to_storage(deps: DepsMut) -> StdResult<()> {
    deps.storage.set(b"test.key", &1u64.to_be_bytes());

    Ok(())
}
