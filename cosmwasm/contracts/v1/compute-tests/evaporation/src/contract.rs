use cosmwasm_std::{
    entry_point, DepsMut, Env, MessageInfo, Response, StdResult,
};
use crate::msg::{ExecuteMsg, InstantiateMsg,};

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
pub fn execute(deps: DepsMut, _env: Env, _info: MessageInfo, msg: ExecuteMsg) -> StdResult<Response> {
    match msg {
        ExecuteMsg::Evaporate { amount } => {
            deps.api.gas_evaporate(amount)?;
            Ok(Response::default())
        }
        ExecuteMsg::CheckGas { } => {
            let gas_used: u64 = deps.api.check_gas()?;

            Ok(Response::default().add_attribute("gas", gas_used.to_string()))
        }
        ExecuteMsg::UseExact { amount } => {
            let gas_used: u64 = deps.api.check_gas()?;

            let to_evaporate = amount - gas_used as u32;

            deps.api.gas_evaporate(to_evaporate)?;

            Ok(Response::default())
        }
    }

}



