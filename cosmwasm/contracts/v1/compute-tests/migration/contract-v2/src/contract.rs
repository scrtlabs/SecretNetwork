use crate::msg::{ExecuteMsg, InstantiateMsg, MigrateMsg, QueryMsg};
use cosmwasm_std::{entry_point, DepsMut, Env, MessageInfo, Response, StdError, StdResult, Deps, Binary, to_binary};

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

        ExecuteMsg::Increment {} |
        ExecuteMsg::NewFunctionWithStorage {} => {
            let mut x = read_storage(&deps.as_ref())?;

            // let mut resp = Response::new();
            //
            // resp.x += 1;
            x += 1;
            write_to_storage(deps, x)?;
            Ok(Response::default())
        }
    }
}

#[entry_point]
pub fn migrate(_deps: DepsMut, _env: Env, msg: MigrateMsg) -> StdResult<Response> {
    match msg {
        MigrateMsg::Migrate {} => Ok(Response::default()),
        MigrateMsg::StdError {} => Err(StdError::generic_err("this is an std error")),
    }
}

pub fn read_storage(deps: &Deps) -> StdResult<u64> {
    let x = deps.storage.get(b"test.key").unwrap_or(vec![]);

    let mut y = [0u8; 8];
    y.copy_from_slice(&x);
    Ok(u64::from_be_bytes(y))
}

#[allow(dead_code)]
pub fn write_to_storage(deps: DepsMut, value: u64) -> StdResult<()> {
    deps.storage.set(b"test.key", &value.to_be_bytes());

    Ok(())
}

pub fn query(deps: Deps, _env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::GetCounter {} => to_binary(&read_storage(&deps)?),
    }
}
