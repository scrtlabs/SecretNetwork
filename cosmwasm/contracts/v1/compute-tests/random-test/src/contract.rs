use cosmwasm_std::{entry_point, Binary, Deps, DepsMut, Env, MessageInfo, Response, StdResult};

use crate::msg::{ExecuteMsg, InstantiateMsg, QueryMsg};

#[entry_point]
pub fn instantiate(
    _deps: DepsMut,
    env: Env,
    info: MessageInfo,
    msg: InstantiateMsg,
) -> StdResult<Response> {
    match msg {
        InstantiateMsg::GetEnv {} => Ok(Response::new()
            .add_attribute("env", serde_json_wasm::to_string(&env).unwrap())
            .add_attribute("info", serde_json_wasm::to_string(&info).unwrap())),
    }
}

#[entry_point]
pub fn execute(
    _deps: DepsMut,
    env: Env,
    info: MessageInfo,
    msg: ExecuteMsg,
) -> StdResult<Response> {
    match msg {
        ExecuteMsg::GetEnv {} => Ok(Response::new()
            .add_attribute("env", serde_json_wasm::to_string(&env).unwrap())
            .add_attribute("info", serde_json_wasm::to_string(&info).unwrap())),
    }
}

#[entry_point]
pub fn query(_deps: Deps, env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::GetEnv {} => Ok(Binary::from(
            serde_json_wasm::to_string(&env)
                .unwrap()
                .as_bytes()
                .to_vec(),
        )),
    }
}
