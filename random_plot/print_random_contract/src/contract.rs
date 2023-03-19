use cosmwasm_std::{entry_point, Binary, DepsMut, Env, MessageInfo, Response, StdResult};
use crate::msg::{Msg, PacketMsg, QueryMsg};

#[entry_point]
pub fn instantiate(_deps: DepsMut, _env: Env, _info: MessageInfo, _msg: Msg) -> StdResult<Response> {
    return Ok(Response::new());
}

#[entry_point]
pub fn execute(_deps: DepsMut, _env: Env, info: MessageInfo, msg: Msg) -> StdResult<Response> {
    match msg {
        Msg::GetRandom {} => {
            return Ok(
                Response::new()
                    .add_attribute_plaintext("random_value", format!("{:?}", info.random))
            )
        }
    }
}

#[entry_point]
pub fn query(_deps: Deps, _env: Env, _msg: QueryMsg) -> StdResult<Binary> {
    Ok(Binary::default())
}
