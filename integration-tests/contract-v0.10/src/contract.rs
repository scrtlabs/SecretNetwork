use cosmwasm_std::{
    to_binary, Api, BalanceResponse, BankQuery, Binary, Coin, Env, Extern, HandleResponse,
    HandleResult, HumanAddr, InitResponse, InitResult, Querier, QueryRequest, QueryResult, Storage,
    VoteOption,
};

/////////////////////////////// Messages ///////////////////////////////

use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum Msg {
    Nop {},
    BankMsgSend {
        to_address: String,
        amount: Vec<Coin>,
    },
    StargateMsg {
        type_url: String,
        value: Binary,
    },
    StakingMsgDelegate {
        validator: String,
        amount: Coin,
    },
    StakingMsgUndelegate {
        validator: String,
        amount: Coin,
    },
    StakingMsgRedelegate {
        src_validator: String,
        dst_validator: String,
        amount: Coin,
    },
    GovVote {
        proposal_id: u64,
        vote: VoteOption,
    },
    DistributionMsgSetWithdrawAddress {
        address: String,
    },
    DistributionMsgWithdrawDelegatorReward {
        validator: String,
    },

    WasmMsgInstantiate {
        code_id: u64,
        code_hash: String,
        msg: Binary,
        funds: Vec<Coin>,
        label: String,
    },
    WasmMsgExecute {
        contract_addr: String,
        code_hash: String,
        msg: Binary,
        funds: Vec<Coin>,
    },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryMsg {
    BankBalance { address: HumanAddr, denom: String },
}

/////////////////////////////// Init ///////////////////////////////

pub fn init<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: Msg,
) -> InitResult {
    return Ok(InitResponse {
        messages: vec![],
        log: vec![],
    });
}

/////////////////////////////// Handle ///////////////////////////////

pub fn handle<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: Msg,
) -> HandleResult {
    // match msg {}
    return Ok(HandleResponse {
        messages: vec![],
        log: vec![],
        data: None,
    });
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
    }
}
