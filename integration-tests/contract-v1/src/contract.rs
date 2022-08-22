use cosmwasm_std::{
    entry_point, BankMsg, Binary, CosmosMsg, Deps, DepsMut, Env, MessageInfo, Response, StdResult,
};

use crate::msg::{ExecuteMsg, InstantiateMsg, QueryMsg};

#[entry_point]
pub fn instantiate(
    _deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    _msg: InstantiateMsg,
) -> StdResult<Response> {
    Ok(Response::new())
}

#[entry_point]
pub fn execute(
    _deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    msg: ExecuteMsg,
) -> StdResult<Response> {
    match msg {
        ExecuteMsg::BankMsg { to_address, amount } => {
            Ok(Response::new().add_message(CosmosMsg::Bank(BankMsg::Send { to_address, amount })))
        }
        ExecuteMsg::StargateMsg { type_url, value } => {
            Ok(Response::new().add_message(CosmosMsg::Stargate { type_url, value }))
        }
    }
}

#[entry_point]
pub fn query(_deps: Deps, _env: Env, _msg: QueryMsg) -> StdResult<Binary> {
    Ok(Binary(vec![]))
}
