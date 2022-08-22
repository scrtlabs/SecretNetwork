use cosmwasm_std::{
    entry_point, BankMsg, Binary, CosmosMsg, Deps, DepsMut, DistributionMsg, Env, GovMsg, IbcMsg,
    MessageInfo, QueryRequest, Response, StakingMsg, StdResult, WasmMsg,
};

use crate::msg::{Msg, QueryMsg};

#[entry_point]
pub fn instantiate(deps: DepsMut, env: Env, info: MessageInfo, msg: Msg) -> StdResult<Response> {
    return handle_msg(deps, env, info, msg);
}

#[entry_point]
pub fn execute(deps: DepsMut, env: Env, info: MessageInfo, msg: Msg) -> StdResult<Response> {
    return handle_msg(deps, env, info, msg);
}

fn handle_msg(_deps: DepsMut, _env: Env, _info: MessageInfo, msg: Msg) -> StdResult<Response> {
    match msg {
        Msg::Nop {} => {
            return Ok(Response::new());
        }
        Msg::BankMsgSend { to_address, amount } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Bank(BankMsg::Send { to_address, amount }))
            );
        }
        Msg::StargateMsg { type_url, value } => {
            return Ok(Response::new().add_message(CosmosMsg::Stargate { type_url, value }));
        }
        Msg::StakingMsgDelegate { validator, amount } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Staking(StakingMsg::Delegate {
                    validator,
                    amount,
                })),
            );
        }
        Msg::StakingMsgUndelegate { validator, amount } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Staking(StakingMsg::Undelegate {
                    validator,
                    amount,
                })),
            );
        }
        Msg::StakingMsgRedelegate {
            src_validator,
            dst_validator,
            amount,
        } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Staking(StakingMsg::Redelegate {
                    src_validator,
                    dst_validator,
                    amount,
                })),
            );
        }
        Msg::GovVote { proposal_id, vote } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Gov(GovMsg::Vote { proposal_id, vote }))
            );
        }
        Msg::DistributionMsgSetWithdrawAddress { address } => {
            return Ok(Response::new().add_message(CosmosMsg::Distribution(
                DistributionMsg::SetWithdrawAddress { address },
            )));
        }
        Msg::DistributionMsgWithdrawDelegatorReward { validator } => {
            return Ok(Response::new().add_message(CosmosMsg::Distribution(
                DistributionMsg::WithdrawDelegatorReward { validator },
            )));
        }
        Msg::IbcMsgTransfer {
            channel_id,
            to_address,
            amount,
            timeout,
        } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Ibc(IbcMsg::Transfer {
                    channel_id,
                    to_address,
                    amount,
                    timeout,
                })),
            );
        }
        Msg::IbcMsgSendPacket {
            channel_id,
            data,
            timeout,
        } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Ibc(IbcMsg::SendPacket {
                    channel_id,
                    data,
                    timeout,
                })),
            );
        }
        Msg::IbcMsgCloseChannel { channel_id } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Ibc(IbcMsg::CloseChannel { channel_id }))
            );
        }
        Msg::WasmMsgInstantiate {
            code_id,
            code_hash,
            msg,
            funds,
            label,
        } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Instantiate {
                    code_id,
                    code_hash,
                    msg,
                    funds,
                    label,
                })),
            );
        }
        Msg::WasmMsgExecute {
            contract_addr,
            code_hash,
            msg,
            funds,
        } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                    contract_addr,
                    code_hash,
                    msg,
                    funds,
                })),
            );
        }
    }
}

#[entry_point]
pub fn query(deps: Deps, _env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::StargateQuery { path, data } => {
            let res = deps
                .querier
                .query::<Binary>(&QueryRequest::Stargate { path, data });
            deps.api.debug(&format!("ASSAF {:?}", res));
            return Ok(res?);
        }
    }
}
