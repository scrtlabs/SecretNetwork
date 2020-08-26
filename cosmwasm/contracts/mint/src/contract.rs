use cosmwasm_std::{
    log, to_binary, Api, Binary, BondedRatioResponse, Coin, CosmosMsg, Env, Extern, HandleResponse,
    HumanAddr, InflationResponse, InitResponse, InitResult, MintQuery, Querier, RewardsResponse,
    StdResult, Storage, VoteOption,
};

use crate::msg::{HandleMsg, InitMsg};

pub fn init<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: InitMsg,
) -> StdResult<InitResponse> {
    Ok(InitResponse::default())
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: HandleMsg,
) -> StdResult<HandleResponse> {
    match msg {
        HandleMsg::InflationRate {} => try_inflation_rate(deps, env),
        HandleMsg::BondedRatio {} => try_bonded_ratio(deps, env),
    }
}

pub fn try_inflation_rate<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
) -> StdResult<HandleResponse> {
    let query = MintQuery::Inflation {};

    let mut query_rewards: InflationResponse =
        deps.querier
            .query(&query.into())
            .unwrap_or_else(|_| InflationResponse {
                inflation_rate: "".to_string(),
            });

    let active_proposal = query_rewards.inflation_rate.as_bytes().to_vec();

    Ok(HandleResponse {
        messages: vec![],
        log: vec![],
        data: Some(Binary::from(active_proposal)),
    })
}

pub fn try_bonded_ratio<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
) -> StdResult<HandleResponse> {
    let query = MintQuery::BondedRatio {};

    let mut query_rewards: BondedRatioResponse =
        deps.querier
            .query(&query.into())
            .unwrap_or_else(|_| BondedRatioResponse {
                bonded_ratio: "".to_string(),
            });

    let active_proposal = query_rewards.bonded_ratio.as_bytes().to_vec();

    Ok(HandleResponse {
        messages: vec![],
        log: vec![],
        data: Some(Binary::from(active_proposal)),
    })
}
