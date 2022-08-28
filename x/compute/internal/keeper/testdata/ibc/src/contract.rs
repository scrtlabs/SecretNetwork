use crate::msg::{ExecuteMsg, InstantiateMsg, QueryMsg};
use crate::state::{count, count_read};
use cosmwasm_std::{
    entry_point, Binary, Deps, DepsMut, Env, IbcChannelOpenMsg, MessageInfo, Response, StdResult,
};

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
    _deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    _msg: ExecuteMsg,
) -> StdResult<Response> {
    Ok(Response::default())
}

#[entry_point]
pub fn query(deps: Deps, _env: Env, _msg: QueryMsg) -> StdResult<Binary> {
    Ok(count_read(deps.storage).load()?.to_be_bytes().into())
}

#[entry_point]
pub fn ibc_channel_open(deps: DepsMut, _env: Env, _msg: IbcChannelOpenMsg) -> StdResult<()> {
    count(deps.storage).save(&1)?;
    Ok(())
}
