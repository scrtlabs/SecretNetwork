use crate::msg::{ExecuteMsg, InstantiateMsg, QueryMsg};
use crate::state::{count, count_read};
use cosmwasm_std::{
    entry_point, to_binary, Binary, CosmosMsg, Deps, DepsMut, Env, Event, Ibc3ChannelOpenResponse,
    IbcBasicResponse, IbcChannelCloseMsg, IbcChannelConnectMsg, IbcChannelOpenMsg,
    IbcChannelOpenResponse, IbcPacketAckMsg, IbcPacketReceiveMsg, IbcPacketTimeoutMsg,
    IbcReceiveResponse, MessageInfo, Reply, ReplyOn, Response, StdError, StdResult, SubMsg,
    SubMsgResult, WasmMsg,
};
use serde::{Deserialize, Serialize};
use serde_json_wasm as serde_json;

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
pub fn reply(deps: DepsMut, env: Env, reply: Reply) -> StdResult<Response> {
    match (reply.id, reply.result) {
        (1, SubMsgResult::Err(_)) => Err(StdError::generic_err("Failed to inc")),
        (1, SubMsgResult::Ok(_)) => {
            increment(deps, 6)?;
            Ok(Response::new().set_data(to_binary(&"out".to_string())?))
        }
        (2, SubMsgResult::Err(_)) => Err(StdError::generic_err("Failed to inc")),
        (2, SubMsgResult::Ok(_)) => {
            increment(deps, 6)?;
            Ok(Response::new().add_submessage(SubMsg {
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
            }))
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

pub fn get_resp_based_on_num(env: Env, num: u64) -> StdResult<IbcBasicResponse> {
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

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct ContractInfo {
    pub address: String,
    pub hash: String,
}

pub fn is_reply_on(num: u64) -> bool {
    match num {
        2 | 6 => true,
        _ => false,
    }
}

pub fn get_recv_resp_based_on_num(
    env: Env,
    num: u64,
    data: Binary,
) -> StdResult<IbcReceiveResponse> {
    match num {
        0 => Ok(IbcReceiveResponse::default()),
        1 => Ok(IbcReceiveResponse::new().add_submessage(SubMsg {
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
        2 => Ok(IbcReceiveResponse::new().add_submessage(SubMsg {
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
        3 => Ok(IbcReceiveResponse::new().add_attribute("attr1", "😗")),
        4 => Ok(IbcReceiveResponse::new()
            .add_event(Event::new("cyber1".to_string()).add_attribute("attr1", "🤯"))),
        5 => Err(StdError::generic_err("Intentional")),
        6 => Ok(IbcReceiveResponse::new().add_submessage(SubMsg {
            id: 2,
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
        7 => {
            let contract_info: ContractInfo = serde_json::from_slice(data.as_slice()).unwrap();
            Ok(IbcReceiveResponse::new().add_submessage(SubMsg {
                id: 1,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: contract_info.hash,
                    contract_addr: contract_info.address,
                    msg: Binary::from(
                        "{\"increment_from_v1\":{\"addition\":5}}"
                            .as_bytes()
                            .to_vec(),
                    ),
                    funds: vec![],
                })
                .into(),
                reply_on: ReplyOn::Always,
                gas_limit: None,
            }))
        }
        _ => Err(StdError::generic_err("Unsupported channel connect type")),
    }
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

            get_resp_based_on_num(env, num)
        }
        _ => Err(StdError::generic_err("Unsupported channel connect")),
    }
}

#[entry_point]
pub fn ibc_channel_close(
    deps: DepsMut,
    env: Env,
    msg: IbcChannelCloseMsg,
) -> StdResult<IbcBasicResponse> {
    match msg {
        IbcChannelCloseMsg::CloseInit { channel: _ } => {
            count(deps.storage).save(&5)?;
            Ok(IbcBasicResponse::default())
        }
        IbcChannelCloseMsg::CloseConfirm { channel } => {
            let num: u64 = channel.connection_id.parse::<u64>().map_err(|err| {
                StdError::generic_err(format!("Got an error from parsing: {:?}", err))
            })?;

            count(deps.storage).save(&(num + 6))?;
            get_resp_based_on_num(env, num)
        }
        _ => Err(StdError::generic_err("Unsupported channel close")),
    }
}

#[entry_point]
pub fn ibc_packet_receive(
    deps: DepsMut,
    env: Env,
    msg: IbcPacketReceiveMsg,
) -> StdResult<IbcReceiveResponse> {
    count(deps.storage).save(&(msg.packet.sequence + 7))?;
    let mut resp = get_recv_resp_based_on_num(env, msg.packet.sequence, msg.packet.data);
    match &mut resp {
        Ok(r) => {
            if !is_reply_on(msg.packet.sequence) {
                r.acknowledgement = to_binary(&"out".to_string())?;
            }
        }
        Err(_) => {}
    }

    resp
}

#[entry_point]
pub fn ibc_packet_ack(
    deps: DepsMut,
    env: Env,
    msg: IbcPacketAckMsg,
) -> StdResult<IbcBasicResponse> {
    let mut ack = [0u8; 8];
    ack.copy_from_slice(&msg.acknowledgement.data.as_slice()[0..8]);

    if u64::from_le_bytes(ack) != msg.original_packet.sequence {
        return Err(StdError::generic_err("Wrong ack"));
    }

    count(deps.storage).save(&(msg.original_packet.sequence + 8))?;
    get_resp_based_on_num(env, msg.original_packet.sequence)
}

#[entry_point]
pub fn ibc_packet_timeout(
    deps: DepsMut,
    env: Env,
    msg: IbcPacketTimeoutMsg,
) -> StdResult<IbcBasicResponse> {
    count(deps.storage).save(&(msg.packet.sequence + 9))?;
    get_resp_based_on_num(env, msg.packet.sequence)
}
