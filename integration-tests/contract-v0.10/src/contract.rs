use cosmwasm_std::{to_binary, Api, BalanceResponse, BankMsg, BankQuery, Binary, Coin, CosmosMsg, Empty, Env, Extern, GovMsg, HandleResponse, HandleResult, HumanAddr, InitResponse, InitResult, LogAttribute, Querier, QueryRequest, QueryResult, StakingMsg, Storage, VoteOption, WasmMsg, StdError};

/////////////////////////////// Messages ///////////////////////////////

use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum Msg {
    Nop {},
    Echo {
        log: Option<Vec<LogAttribute>>,
        data: Option<Binary>,
    },
    BankMsgSend {
        to_address: HumanAddr,
        amount: Vec<Coin>,
    },
    StakingMsgDelegate {
        validator: HumanAddr,
        amount: Coin,
    },
    StakingMsgUndelegate {
        validator: HumanAddr,
        amount: Coin,
    },
    StakingMsgRedelegate {
        src_validator: HumanAddr,
        dst_validator: HumanAddr,
        amount: Coin,
    },
    StakingMsgWithdraw {
        validator: HumanAddr,
        recipient: Option<HumanAddr>,
    },
    GovMsgVote {
        proposal: u64,
        vote_option: VoteOption,
    },
    WasmMsgInstantiate {
        code_id: u64,
        callback_code_hash: String,
        msg: Binary,
        send: Vec<Coin>,
        label: String,
    },
    WasmMsgExecute {
        contract_addr: HumanAddr,
        callback_code_hash: String,
        msg: Binary,
        send: Vec<Coin>,
    },
    CustomMsg {},
    Forward {
        recipient_address: HumanAddr,
        recipient_hash: String,
        msg: Binary,
    },
    FailTx {}
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryMsg {
    BankBalance { address: HumanAddr, denom: String },
    Forward {},
}

/////////////////////////////// Init ///////////////////////////////

pub fn init<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: Msg,
) -> InitResult {
    deps.storage.set("forwarded".as_bytes(), "no-fail".as_bytes());
    return Ok(InitResponse {
        messages: vec![],
        log: vec![],
    });
}

/////////////////////////////// Handle ///////////////////////////////

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: Msg,
) -> HandleResult {
    match msg {
        Msg::Nop {} => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: None,
        }),
        Msg::Echo { log, data } => Ok(HandleResponse {
            messages: vec![],
            log: log.unwrap_or(vec![]),
            data,
        }),
        Msg::BankMsgSend { to_address, amount } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Bank(BankMsg::Send {
                from_address: env.contract.address,
                to_address,
                amount,
            })],
            log: vec![],
            data: None,
        }),
        Msg::StakingMsgDelegate { validator, amount } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Staking(StakingMsg::Delegate {
                validator,
                amount,
            })],
            log: vec![],
            data: None,
        }),
        Msg::StakingMsgUndelegate { validator, amount } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Staking(StakingMsg::Undelegate {
                validator,
                amount,
            })],
            log: vec![],
            data: None,
        }),
        Msg::StakingMsgRedelegate {
            src_validator,
            dst_validator,
            amount,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Staking(StakingMsg::Redelegate {
                src_validator,
                dst_validator,
                amount,
            })],
            log: vec![],
            data: None,
        }),
        Msg::StakingMsgWithdraw {
            validator,
            recipient,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Staking(StakingMsg::Withdraw {
                validator,
                recipient,
            })],
            log: vec![],
            data: None,
        }),
        Msg::GovMsgVote {
            proposal,
            vote_option,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Gov(GovMsg::Vote {
                proposal,
                vote_option,
            })],
            log: vec![],
            data: None,
        }),
        Msg::WasmMsgInstantiate {
            code_id,
            callback_code_hash,
            msg,
            send,
            label,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                callback_code_hash,
                msg,
                send,
                label,
            })],
            log: vec![],
            data: None,
        }),
        Msg::WasmMsgExecute {
            contract_addr,
            callback_code_hash,
            msg,
            send,
        } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr,
                callback_code_hash,
                msg,
                send,
            })],
            log: vec![],
            data: None,
        }),
        Msg::CustomMsg {} => Ok(HandleResponse {
            messages: vec![CosmosMsg::Custom(Empty {})],
            log: vec![],
            data: None,
        }),
        Msg::Forward { recipient_address, recipient_hash, msg } => {
            deps.storage.set("forwarded".as_bytes(), "forwarded".as_bytes());
            Ok(HandleResponse {
                messages: vec![CosmosMsg::Wasm(
                    WasmMsg::Execute {
                        contract_addr: recipient_address,
                        callback_code_hash: recipient_hash,
                        msg,
                        send: vec![]
                    },
                )],
                log: vec![],
                data: None,
            })
        }
        Msg::FailTx {} => Err (StdError::generic_err("this should always fail")),
    }
}

/////////////////////////////// Query ///////////////////////////////

pub fn query<S: Storage, A: Api, Q: Querier>(deps: &Extern<S, A, Q>, msg: QueryMsg) -> QueryResult {
    match msg {
        QueryMsg::BankBalance { address, denom } => {
            let res =
                deps.querier
                    .query::<BalanceResponse>(&QueryRequest::Bank(BankQuery::Balance {
                        address,
                        denom,
                    }))?;
            return Ok(to_binary(&res)?);
        }
        QueryMsg::Forward { } => Ok(to_binary(&deps.storage.get("forwarded".as_bytes()))?),
    }

}
