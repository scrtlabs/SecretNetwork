use crate::msg::{ExecuteMsg, InstantiateMsg, QueryMsg};
use crate::state::{count, count_read};
use cosmwasm_std::{
    entry_point, to_binary, Binary, CosmosMsg, Deps, DepsMut, Env, Event, Ibc3ChannelOpenResponse,
    IbcBasicResponse, IbcChannelConnectMsg, IbcChannelOpenMsg, IbcChannelOpenResponse, MessageInfo,
    Reply, ReplyOn, Response, StdError, StdResult, SubMsg, SubMsgResult, WasmMsg,
};

pub const IBC_APP_VERSION: &str = "ibc-v1";

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
        ExecuteMsg::Increment { addition } => increment(deps, addition),
    }
}

#[entry_point]
pub fn query(deps: Deps, _env: Env, _msg: QueryMsg) -> StdResult<Binary> {
    let answer = count_read(deps.storage).load()?;
    Ok(to_binary(&answer)?)
}

#[entry_point]
pub fn reply(deps: DepsMut, _env: Env, reply: Reply) -> StdResult<Response> {
    match (reply.id, reply.result) {
        (1, SubMsgResult::Err(_)) => Err(StdError::generic_err("Failed to inc")),
        (1, SubMsgResult::Ok(_)) => {
            increment(deps, 6)?;
            Ok(Response::default())
        }
        _ => Err(StdError::generic_err("invalid reply id or result")),
    }
}

#[entry_point]
pub fn ibc_channel_open(
    deps: DepsMut,
    _env: Env,
    msg: IbcChannelOpenMsg,
) -> StdResult<IbcChannelOpenResponse> {
    match msg {
        IbcChannelOpenMsg::OpenInit { channel: _ } => count(deps.storage).save(&1)?,
        IbcChannelOpenMsg::OpenTry {
            channel: _,
            counterparty_version: _,
        } => count(deps.storage).save(&2)?,
        _ => {}
    }

    Ok(Some(Ibc3ChannelOpenResponse {
        version: IBC_APP_VERSION.to_string(),
    }))
}

pub fn increment(deps: DepsMut, c: u64) -> StdResult<Response> {
    let new_count = count_read(deps.storage).load()? + c;
    count(deps.storage).save(&new_count)?;

    let mut resp = Response::default();
    resp.data = Some((new_count as u32).to_be_bytes().into());

    Ok(resp)
}

#[entry_point]
pub fn ibc_channel_connect(
    deps: DepsMut,
    env: Env,
    msg: IbcChannelConnectMsg,
) -> StdResult<IbcBasicResponse> {
    match msg {
        IbcChannelConnectMsg::OpenAck {
            channel: _,
            counterparty_version: _,
        } => {
            count(deps.storage).save(&3)?;
            Ok(IbcBasicResponse::default())
        }
        IbcChannelConnectMsg::OpenConfirm { channel } => {
            let num: u64 = channel.connection_id.parse::<u64>().map_err(|err| {
                StdError::generic_err(format!("Got an error from parsing: {:?}", err))
            })?;

            count(deps.storage).save(&(num + 4))?;

            match num {
                0 => Ok(IbcBasicResponse::default()),
                1 => Ok(IbcBasicResponse::new().add_submessage(SubMsg {
                    id: 1,
                    msg: CosmosMsg::Wasm(WasmMsg::Execute {
                        code_hash: env.contract.code_hash,
                        contract_addr: env.contract.address.into_string(),
                        msg: Binary::from("{\"increment\":{\"addition\":5}}".as_bytes().to_vec()),
                        funds: vec![],
                    })
                    .into(),
                    reply_on: ReplyOn::Never,
                    gas_limit: None,
                })),
                2 => Ok(IbcBasicResponse::new().add_submessage(SubMsg {
                    id: 1,
                    msg: CosmosMsg::Wasm(WasmMsg::Execute {
                        code_hash: env.contract.code_hash,
                        contract_addr: env.contract.address.into_string(),
                        msg: Binary::from("{\"increment\":{\"addition\":5}}".as_bytes().to_vec()),
                        funds: vec![],
                    })
                    .into(),
                    reply_on: ReplyOn::Always,
                    gas_limit: None,
                })),
                3 => Ok(IbcBasicResponse::new().add_attribute("attr1", "😗")),
                4 => Ok(IbcBasicResponse::new()
                    .add_event(Event::new("cyber1".to_string()).add_attribute("attr1", "🤯"))),
                5 => Err(StdError::generic_err("Intentional")),
                _ => Err(StdError::generic_err("Unsupported channel connect type")),
            }
        }
        _ => Err(StdError::generic_err("Unsupported channel connect")),
    }
}
