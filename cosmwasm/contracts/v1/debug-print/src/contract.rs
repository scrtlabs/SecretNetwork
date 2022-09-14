use cosmwasm_std::{
    Api, Binary, Env, Extern, HandleResponse, InitResponse, Querier, StdError, StdResult, Storage,
};

use crate::msg::{HandleMsg, InitMsg, QueryMsg};

pub fn init<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: InitMsg,
) -> StdResult<InitResponse> {
    Ok(InitResponse::default())
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: HandleMsg,
) -> StdResult<HandleResponse> {
    cosmwasm_std::debug_print("message1");
    cosmwasm_std::debug_print("message2".to_string());
    cosmwasm_std::debug_print!("message{}", 3);

    Ok(HandleResponse::default())
}

pub fn query<S: Storage, A: Api, Q: Querier>(
    _deps: &Extern<S, A, Q>,
    _msg: QueryMsg,
) -> StdResult<Binary> {
    Err(StdError::generic_err(""))
}
