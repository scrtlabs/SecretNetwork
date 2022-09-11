use cosmwasm_std::{
    log, to_binary, Api, Binary, Coin, CosmosMsg, DistQuery, Env, Extern, GovQuery, HandleResponse,
    HumanAddr, InitResponse, InitResult, Querier, RewardsResponse, StdResult, Storage, VoteOption,
};

use crate::msg::{HandleMsg, InitMsg};

pub fn init<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: InitMsg,
) -> InitResult {
    Ok(InitResponse::default())
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: HandleMsg,
) -> StdResult<HandleResponse> {
    match msg {
        HandleMsg::Rewards { address } => try_query_rewards(deps, env, address),
    }
}

pub fn try_query_rewards<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    address: HumanAddr,
) -> StdResult<HandleResponse> {
    let query = DistQuery::Rewards {
        delegator: address.clone(),
    };

    let mut query_rewards: RewardsResponse =
        deps.querier
            .query(&query.into())
            .unwrap_or_else(|_| RewardsResponse {
                rewards: vec![],
                total: vec![],
            });

    let active_proposal = query_rewards
        .total
        .pop()
        .unwrap_or_else(|| Coin {
            denom: "stake".to_string(),
            amount: Default::default(),
        })
        .amount
        .0 as u64;

    Ok(HandleResponse {
        messages: vec![],
        log: vec![],
        data: Some(Binary::from(active_proposal.to_be_bytes().to_vec())),
    })
}
