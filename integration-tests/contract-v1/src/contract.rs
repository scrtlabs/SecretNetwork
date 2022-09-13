use crate::ibc::PACKET_LIFETIME;
use cosmwasm_std::{
    entry_point, to_binary, to_vec, AllBalanceResponse, AllDelegationsResponse,
    AllValidatorsResponse, BalanceResponse, BankMsg, BankQuery, Binary, BondedDenomResponse,
    ChannelResponse, ContractInfoResponse, ContractResult, CosmosMsg, DelegationResponse, Deps,
    DepsMut, DistributionMsg, Empty, Env, Event, GovMsg, IbcMsg, IbcQuery, IbcTimeout,
    ListChannelsResponse, MessageInfo, PortIdResponse, QueryRequest, Response, StakingMsg,
    StakingQuery, StdError, StdResult, ValidatorResponse, WasmMsg, WasmQuery,
};

use crate::msg::{Msg, PacketMsg, QueryMsg};
use crate::state::{
    ack_store, ack_store_read, channel_store, channel_store_read, receive_store,
    receive_store_read, timeout_store, timeout_store_read,
};

#[entry_point]
pub fn instantiate(deps: DepsMut, env: Env, info: MessageInfo, msg: Msg) -> StdResult<Response> {
    channel_store(deps.storage).save(&"no channel yet".to_string())?;
    ack_store(deps.storage).save(&"no ack yet".to_string())?;
    receive_store(deps.storage).save(&"no receive yet".to_string())?;
    timeout_store(deps.storage).save(&"no timeout yet".to_string())?;

    return handle_msg(deps, env, info, msg);
}

#[entry_point]
pub fn execute(deps: DepsMut, env: Env, info: MessageInfo, msg: Msg) -> StdResult<Response> {
    return handle_msg(deps, env, info, msg);
}

fn handle_msg(deps: DepsMut, env: Env, _info: MessageInfo, msg: Msg) -> StdResult<Response> {
    match msg {
        Msg::Nop {} => {
            return Ok(Response::new().set_data(vec![137, 137].as_slice()));
        }
        Msg::BankMsgSend { to_address, amount } => {
            return Ok(
                Response::new().add_message(CosmosMsg::Bank(BankMsg::Send { to_address, amount }))
            );
        }
        Msg::BankMsgBurn { amount } => {
            return Ok(Response::new().add_message(CosmosMsg::Bank(BankMsg::Burn { amount })));
        }
        Msg::SendIbcPacket { message } => {
            let channel_id = channel_store_read(deps.storage).load()?;
            let packet = PacketMsg::Message {
                value: channel_id + &message,
            };

            return Ok(Response::new().add_message(IbcMsg::SendPacket {
                channel_id: channel_store_read(deps.storage).load()?,
                data: to_binary(&packet)?,
                timeout: IbcTimeout::with_timestamp(env.block.time.plus_seconds(PACKET_LIFETIME)),
            }));
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

        Msg::StakingMsgWithdraw { validator } => {
            return Ok(Response::new().add_message(CosmosMsg::Distribution(
                DistributionMsg::WithdrawDelegatorReward { validator },
            )));
        }
        Msg::GovMsgVote {
            proposal,
            vote_option,
        } => {
            return Ok(Response::new().add_message(CosmosMsg::Gov(GovMsg::Vote {
                proposal_id: proposal,
                vote: vote_option,
            })));
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
        Msg::GetTxId {} => match env.transaction {
            None => Err(StdError::generic_err("Transaction info wasn't set")),
            Some(t) => {
                return Ok(Response::new().add_event(
                    Event::new("count".to_string())
                        .add_attribute_plaintext("count-val", t.index.to_string()),
                ))
            }
        },
    }
}

#[entry_point]
pub fn query(deps: Deps, env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::Stargate { path, data } => {
            return Ok(to_binary(
                &deps
                    .querier
                    .raw_query(&to_vec(&QueryRequest::<Empty>::Stargate { path, data })?)
                    .unwrap()
                    .unwrap(),
            )?);
        }
        QueryMsg::BankBalance { address, denom } => {
            return Ok(to_binary(&deps.querier.query::<BalanceResponse>(
                &QueryRequest::Bank(BankQuery::Balance { address, denom }),
            )?)?);
        }
        QueryMsg::BankAllBalances { address } => {
            return Ok(to_binary(&deps.querier.query::<AllBalanceResponse>(
                &QueryRequest::Bank(BankQuery::AllBalances { address }),
            )?)?);
        }
        QueryMsg::StakingBondedDenom {} => {
            return Ok(to_binary(&deps.querier.query::<BondedDenomResponse>(
                &QueryRequest::Staking(StakingQuery::BondedDenom {}),
            )?)?);
        }
        QueryMsg::StakingAllDelegations { delegator } => {
            return Ok(to_binary(&deps.querier.query::<AllDelegationsResponse>(
                &QueryRequest::Staking(StakingQuery::AllDelegations { delegator }),
            )?)?);
        }
        QueryMsg::StakingDelegation {
            delegator,
            validator,
        } => {
            return Ok(to_binary(&deps.querier.query::<DelegationResponse>(
                &QueryRequest::Staking(StakingQuery::Delegation {
                    delegator,
                    validator,
                }),
            )?)?);
        }
        QueryMsg::StakingAllValidators {} => {
            return Ok(to_binary(&deps.querier.query::<AllValidatorsResponse>(
                &QueryRequest::Staking(StakingQuery::AllValidators {}),
            )?)?);
        }
        QueryMsg::StakingValidator { address } => {
            return Ok(to_binary(&deps.querier.query::<ValidatorResponse>(
                &QueryRequest::Staking(StakingQuery::Validator { address }),
            )?)?);
        }
        QueryMsg::IbcPortId {} => {
            return Ok(to_binary(&deps.querier.query::<PortIdResponse>(
                &QueryRequest::Ibc(IbcQuery::PortId {}),
            )?)?);
        }
        QueryMsg::IbcListChannels { port_id } => {
            return Ok(to_binary(&deps.querier.query::<ListChannelsResponse>(
                &QueryRequest::Ibc(IbcQuery::ListChannels { port_id }),
            )?)?);
        }
        QueryMsg::IbcChannel {
            channel_id,
            port_id,
        } => {
            return Ok(to_binary(&deps.querier.query::<ChannelResponse>(
                &QueryRequest::Ibc(IbcQuery::Channel {
                    channel_id,
                    port_id,
                }),
            )?)?);
        }
        QueryMsg::WasmSmart {
            contract_addr,
            code_hash,
            msg,
        } => {
            let result = &deps
                .querier
                .raw_query(&to_vec(&QueryRequest::Wasm::<Empty>(WasmQuery::Smart {
                    contract_addr,
                    code_hash,
                    msg,
                }))?)
                .unwrap();

            match result {
                ContractResult::Ok(ok) => Ok(Binary(ok.0.to_vec())),
                ContractResult::Err(err) => Err(StdError::generic_err(err)),
            }
        }
        QueryMsg::WasmContractInfo { contract_addr } => {
            return Ok(to_binary(&deps.querier.query::<ContractInfoResponse>(
                &QueryRequest::Wasm(WasmQuery::ContractInfo { contract_addr }),
            )?)?);
        }
        QueryMsg::GetTxId {} => match env.transaction {
            None => Err(StdError::generic_err("Transaction info wasn't set")),
            Some(t) => return Ok(to_binary(&t.index)?),
        },
        QueryMsg::LastIbcReceive {} => Ok(to_binary(&receive_store_read(deps.storage).load()?)?),
        QueryMsg::LastIbcAck {} => Ok(to_binary(&ack_store_read(deps.storage).load()?)?),
        QueryMsg::LastIbcTimeout {} => Ok(to_binary(&timeout_store_read(deps.storage).load()?)?),
    }
}
